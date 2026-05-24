package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"

	"hdu-srun-login/internal/srun"

	"github.com/AsakiRain/logger-go"
)

// srunLogger 是适配 srun.Logger 接口的包装器
type srunLogger struct {
	*logger.Logger
}

func (l *srunLogger) Logf(level string, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	switch level {
	case "DEBUG":
		l.Logger.Debug(msg)
	case "INFO":
		l.Logger.Info(msg)
	case "WARNING":
		l.Logger.Warn(msg)
	case "ERROR":
		l.Logger.Error(msg)
	case "SUCCESS":
		l.Logger.Info(msg)
	default:
		l.Logger.Info(msg)
	}
}

// Config represents the configuration file structure
type Config struct {
	Accounts        []srun.Auth `yaml:"accounts" json:"accounts"`
	CheckInterval   string      `yaml:"check_interval" json:"check_interval"`
	RefreshInterval string      `yaml:"refresh_interval" json:"refresh_interval"`
	Once            bool        `yaml:"once" json:"once"`
	BindInterfaces  []string    `yaml:"bind_interfaces" json:"bind_interfaces"`
	Log             LogConfig   `yaml:"log" json:"log"`
}

// LogConfig represents the logger configuration
type LogConfig struct {
	ConsoleLevel string `yaml:"console_level" json:"console_level"`
	FileLevel    string `yaml:"file_level" json:"file_level"`
	LogFile      string `yaml:"log_file" json:"log_file"`
	MaxSizeMB    int    `yaml:"max_size_mb" json:"max_size_mb"`
	MaxBackups   int    `yaml:"max_backups" json:"max_backups"`
	ShowCaller   bool   `yaml:"show_caller" json:"show_caller"`
}

// DefaultConfig returns a Config with default values
func DefaultConfig() Config {
	return Config{
		CheckInterval:   "2m",
		RefreshInterval: "6h",
		Log: LogConfig{
			ConsoleLevel: "info",
			FileLevel:    "debug",
			LogFile:      "srun_login.log",
			MaxSizeMB:    10,
			MaxBackups:   5,
			ShowCaller:   false,
		},
	}
}

func loadConfig(path string) (Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}

	ext := filepath.Ext(path)

	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("yaml parse error: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("json parse error: %w", err)
		}
	default:
		// 默认尝试 YAML 格式
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("parse error (tried yaml): %w", err)
		}
	}

	// 向后兼容：如果 accounts 为空，尝试直接解析为数组
	if len(cfg.Accounts) == 0 {
		var auths []srun.Auth
		switch ext {
		case ".yaml", ".yml":
			_ = yaml.Unmarshal(data, &auths)
		case ".json":
			_ = json.Unmarshal(data, &auths)
		default:
			_ = yaml.Unmarshal(data, &auths)
		}
		cfg.Accounts = auths
	}

	if len(cfg.Accounts) == 0 {
		return cfg, errors.New("auth list is empty")
	}
	for i, auth := range cfg.Accounts {
		if auth.Username == "" || auth.Password == "" {
			return cfg, fmt.Errorf("auth[%d] must include username and password", i)
		}
	}
	return cfg, nil
}

func main() {
	configPath := flag.String("config", "config.yaml", "config file (supports YAML and JSON)")
	flagOnce := flag.Bool("once", false, "override config: check status once and login when needed")
	flagCheckInterval := flag.Duration("check-interval", 0, "override config: status check interval (e.g. 2m, 5m)")
	flagRefreshInterval := flag.Duration("refresh-interval", 0, "override config: logout/login refresh interval (e.g. 6h, 12h)")
	flagLogFile := flag.String("log-file", "", "override config: log file path")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	// 命令行参数覆盖配置文件
	once := cfg.Once
	if *flagOnce {
		once = true
	}

	checkInterval, _ := time.ParseDuration(cfg.CheckInterval)
	if *flagCheckInterval > 0 {
		checkInterval = *flagCheckInterval
	}
	if checkInterval == 0 {
		checkInterval = 2 * time.Minute
	}

	// refresh_interval 允许设置为 0 来关闭
	refreshInterval, _ := time.ParseDuration(cfg.RefreshInterval)
	if *flagRefreshInterval > 0 {
		refreshInterval = *flagRefreshInterval
	}

	logFile := cfg.Log.LogFile
	if *flagLogFile != "" {
		logFile = *flagLogFile
	}

	// 初始化日志库
	logCfg := logger.Config{
		ConsoleLevel: cfg.Log.ConsoleLevel,
		FileLevel:    cfg.Log.FileLevel,
		LogFile:      logFile,
		MaxSize:      cfg.Log.MaxSizeMB,
		MaxBackups:   cfg.Log.MaxBackups,
		DefaultTag:   "srun_login",
		ShowCaller:   cfg.Log.ShowCaller,
	}
	log := logger.New(logCfg)
	defer log.Close()

	sLog := &srunLogger{Logger: log}
	sLog.Logf("INFO", "Process started (config: %s, check: %v, refresh: %v, once: %v, bind_interfaces: %v)", *configPath, checkInterval, refreshInterval, once, cfg.BindInterfaces)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runner := srun.NewRunner(cfg.Accounts, sLog, cfg.BindInterfaces)

	if once {
		if err := runner.Check(ctx); err != nil {
			sLog.Logf("ERROR", "check failed: %v", err)
			os.Exit(1)
		}
		return
	}

	checkTicker := time.NewTicker(checkInterval)
	defer checkTicker.Stop()

	// 使用 nil channel 来禁用刷新逻辑（当 refreshInterval 为 0 时）
	var refreshTicker *time.Ticker
	var refreshChan <-chan time.Time
	if refreshInterval > 0 {
		refreshTicker = time.NewTicker(refreshInterval)
		refreshChan = refreshTicker.C
		defer refreshTicker.Stop()
	}

	if err := runner.Check(ctx); err != nil {
		sLog.Logf("ERROR", "check failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			sLog.Logf("INFO", "Process stopped")
			return
		case <-checkTicker.C:
			if err := runner.Check(ctx); err != nil {
				sLog.Logf("ERROR", "check failed: %v", err)
			}
		case <-refreshChan:
			if err := runner.Refresh(ctx); err != nil {
				sLog.Logf("ERROR", "refresh failed: %v", err)
			}
		}
	}
}
