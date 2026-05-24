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

	"hdu-srun-login/internal/service"
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
			LogFile:      "hdu-srun-login.log",
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

func printUsage() {
	fmt.Println("Usage: hdu-srun-login [command] [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  install     Install as a system service")
	fmt.Println("  uninstall   Uninstall the system service")
	fmt.Println("  start       Start the service")
	fmt.Println("  stop        Stop the service")
	fmt.Println("  restart     Restart the service")
	fmt.Println("  status      Show service status")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --config string        Config file path (default: config.yaml)")
	fmt.Println("  --once                 Check status once and exit")
	fmt.Println("  --check-interval dur   Status check interval (e.g. 2m, 5m)")
	fmt.Println("  --refresh-interval dur Refresh interval (e.g. 6h, 12h)")
	fmt.Println("  --log-file string      Log file path")
	fmt.Println()
	fmt.Println("Install Options:")
	fmt.Println("  --bin-dir string       Binary installation directory")
	fmt.Println("  --config string        Config file path to install")
}

func handleInstall(args []string) {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	binDir := fs.String("bin-dir", "", "Binary installation directory")
	configPath := fs.String("config", "", "Config file path to install")
	fs.Parse(args)

	opts := service.InstallOptions{
		BinDir:     *binDir,
		ConfigPath: *configPath,
	}

	if err := service.Install(opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func handleUninstall(args []string) {
	if err := service.Uninstall(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func handleStart(args []string) {
	if err := service.StartService(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func handleStop(args []string) {
	if err := service.StopService(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func handleRestart(args []string) {
	if err := service.RestartService(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func handleStatus(args []string) {
	if err := service.StatusService(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(configPath string, once bool, flagCheckInterval, flagRefreshInterval time.Duration, flagLogFile string) {
	cfg, err := loadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	// 命令行参数覆盖配置文件
	if once {
		once = true
	}

	checkInterval, _ := time.ParseDuration(cfg.CheckInterval)
	if flagCheckInterval > 0 {
		checkInterval = flagCheckInterval
	}
	if checkInterval == 0 {
		checkInterval = 2 * time.Minute
	}

	// refresh_interval 允许设置为 0 来关闭
	refreshInterval, _ := time.ParseDuration(cfg.RefreshInterval)
	if flagRefreshInterval > 0 {
		refreshInterval = flagRefreshInterval
	}

	logFile := cfg.Log.LogFile
	// 如果日志文件路径是相对路径，相对于可执行文件所在目录
	if logFile != "" && !filepath.IsAbs(logFile) {
		execPath, err := os.Executable()
		if err == nil {
			execDir := filepath.Dir(execPath)
			logFile = filepath.Join(execDir, logFile)
		}
	}
	if flagLogFile != "" {
		logFile = flagLogFile
		// 命令行指定的路径如果是相对路径，也相对于可执行文件所在目录
		if !filepath.IsAbs(logFile) {
			execPath, err := os.Executable()
			if err == nil {
				execDir := filepath.Dir(execPath)
				logFile = filepath.Join(execDir, logFile)
			}
		}
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
	sLog.Logf("INFO", "Process started (config: %s, check: %v, refresh: %v, once: %v, bind_interfaces: %v)", configPath, checkInterval, refreshInterval, once, cfg.BindInterfaces)

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

func main() {
	// 检查子命令
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			handleInstall(os.Args[2:])
			return
		case "uninstall":
			handleUninstall(os.Args[2:])
			return
		case "start":
			handleStart(os.Args[2:])
			return
		case "stop":
			handleStop(os.Args[2:])
			return
		case "restart":
			handleRestart(os.Args[2:])
			return
		case "status":
			handleStatus(os.Args[2:])
			return
		case "help", "--help", "-h":
			printUsage()
			return
		}
	}

	// 正常解析参数
	configPath := flag.String("config", "", "config file (supports YAML and JSON)")
	flagOnce := flag.Bool("once", false, "override config: check status once and login when needed")
	flagCheckInterval := flag.Duration("check-interval", 0, "override config: status check interval (e.g. 2m, 5m)")
	flagRefreshInterval := flag.Duration("refresh-interval", 0, "override config: logout/login refresh interval (e.g. 6h, 12h)")
	flagLogFile := flag.String("log-file", "", "override config: log file path")
	flagService := flag.Bool("service", false, "run as a service")
	flag.Parse()

	// 服务模式
	if *flagService {
		// 设置服务运行时的回调函数
		service.RunFunc = func(configPath string) error {
			run(configPath, false, 0, 0, "")
			return nil
		}
		if err := service.Run(*configPath); err != nil {
			fmt.Fprintf(os.Stderr, "service run error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// 查找配置文件
	if *configPath == "" {
		found, err := service.FindConfig("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			fmt.Println()
			printUsage()
			os.Exit(1)
		}
		*configPath = found
	}

	run(*configPath, *flagOnce, *flagCheckInterval, *flagRefreshInterval, *flagLogFile)
}
