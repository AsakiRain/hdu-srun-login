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
	Username         string    `yaml:"username" json:"username"`
	Password         string    `yaml:"password" json:"password"`
	CheckInterval    string    `yaml:"check_interval" json:"check_interval"`
	RefreshInterval  string    `yaml:"refresh_interval" json:"refresh_interval"`
	Once             bool      `yaml:"once" json:"once"`
	NetworkDetection bool      `yaml:"network_detection" json:"network_detection"`
	BindInterfaces   []string  `yaml:"bind_interfaces" json:"bind_interfaces"`
	Log              LogConfig `yaml:"log" json:"log"`
}

// LogConfig represents the logger configuration
type LogConfig struct {
	ConsoleLevel string `yaml:"console_level" json:"console_level"`
	FileLevel    string `yaml:"file_level" json:"file_level"`
	LogFile      string `yaml:"log_file" json:"log_file"`
	MaxSizeMB    int    `yaml:"max_size_mb" json:"max_size_mb"`
	MaxBackups   int    `yaml:"max_backups" json:"max_backups"`
	Flush        bool   `yaml:"flush" json:"flush"`
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

func parseConfig(path string) (Config, error) {
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

	return cfg, nil
}

func validateConfig(cfg Config) error {
	if cfg.Username == "" || cfg.Password == "" {
		return errors.New("config must include username and password")
	}
	return nil
}

func loadConfig(path string) (Config, error) {
	cfg, err := parseConfig(path)
	if err != nil {
		return cfg, err
	}
	return cfg, validateConfig(cfg)
}

func resolveLogFile(logFile string) string {
	if logFile == "" || filepath.IsAbs(logFile) {
		return logFile
	}
	execPath, err := os.Executable()
	if err != nil {
		return logFile
	}
	return filepath.Join(filepath.Dir(execPath), logFile)
}

func newLogger(cfg Config, flagLogFile string) *logger.Logger {
	logFile := cfg.Log.LogFile
	if flagLogFile != "" {
		logFile = flagLogFile
	}
	logFile = resolveLogFile(logFile)

	return logger.New(logger.Config{
		ConsoleLevel: cfg.Log.ConsoleLevel,
		FileLevel:    cfg.Log.FileLevel,
		LogFile:      logFile,
		MaxSize:      cfg.Log.MaxSizeMB,
		MaxBackups:   cfg.Log.MaxBackups,
		Flush:        cfg.Log.Flush,
		DefaultTag:   "srun_login",
		ShowCaller:   cfg.Log.ShowCaller,
	})
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

func runWithContext(ctx context.Context, configPath string, once bool, flagCheckInterval, flagRefreshInterval time.Duration, flagLogFile string) error {
	cfg, err := parseConfig(configPath)
	log := newLogger(cfg, flagLogFile)
	defer log.Close()
	sLog := &srunLogger{Logger: log}

	if err != nil {
		sLog.Logf("ERROR", "load config failed: %v", err)
		return fmt.Errorf("load config: %w", err)
	}
	if err := validateConfig(cfg); err != nil {
		sLog.Logf("ERROR", "invalid config %s: %v", configPath, err)
		return err
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

	sLog.Logf("INFO", "Process started (config: %s, check: %v, refresh: %v, once: %v, network_detection: %v, bind_interfaces: %v)", configPath, checkInterval, refreshInterval, once, cfg.NetworkDetection, cfg.BindInterfaces)

	runner := srun.NewRunner(srun.Auth{Username: cfg.Username, Password: cfg.Password}, sLog, cfg.BindInterfaces)

	if once {
		if err := runner.Check(ctx); err != nil {
			sLog.Logf("ERROR", "check failed: %v", err)
			return err
		}
		return nil
	}

	var networkEvents <-chan struct{}
	if cfg.NetworkDetection {
		sLog.Logf("INFO", "Network environment detection enabled")
		networkEvents = srun.NewNetworkWatcher(sLog).Watch(ctx)
	} else {
		sLog.Logf("INFO", "Network environment detection disabled; using scheduled checks only")
	}
	scheduler := &srun.Scheduler{
		Check:               runner.Check,
		Refresh:             runner.Refresh,
		Network:             networkEvents,
		Logger:              sLog,
		CheckInterval:       checkInterval,
		RefreshInterval:     refreshInterval,
		UnreachableInterval: srun.DefaultUnreachableCheckInterval,
	}
	scheduler.Run(ctx)
	return nil
}

func run(configPath string, once bool, flagCheckInterval, flagRefreshInterval time.Duration, flagLogFile string) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := runWithContext(ctx, configPath, once, flagCheckInterval, flagRefreshInterval, flagLogFile); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
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
		service.RunFunc = func(ctx context.Context, configPath string) error {
			return runWithContext(ctx, configPath, false, 0, 0, "")
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
