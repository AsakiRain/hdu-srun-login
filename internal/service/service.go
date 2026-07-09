package service

import (
	"context"
	"embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/kardianos/service"
)

//go:embed config.example.yaml
var configExample embed.FS

const serviceName = "hdu-srun-login"

var exitProcess = os.Exit

// InstallOptions 安装选项
type InstallOptions struct {
	BinDir     string // 程序安装目录
	ConfigPath string // 配置文件路径（可选）
}

// GetDefaultBinDir 获取默认 bin 目录
func GetDefaultBinDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(homeDir, "AppData", "Local")
		}
		return filepath.Join(localAppData, "bin"), nil
	}
	return filepath.Join(homeDir, ".local", "bin"), nil
}

// GetDefaultConfigPath 获取默认配置文件路径（用户目录下的 hdu-srun-login.yaml）
func GetDefaultConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, "hdu-srun-login.yaml"), nil
}

// GetInstallConfigPath 获取服务安装时的默认配置文件路径（程序同目录 config.yaml）
func GetInstallConfigPath(targetExec string) string {
	return filepath.Join(filepath.Dir(targetExec), "config.yaml")
}

// GetExecutablePath 获取安装后的可执行文件路径
func GetExecutablePath(binDir string) (string, error) {
	if binDir == "" {
		var err error
		binDir, err = GetDefaultBinDir()
		if err != nil {
			return "", err
		}
	}

	if runtime.GOOS == "windows" {
		return filepath.Join(binDir, serviceName+".exe"), nil
	}
	return filepath.Join(binDir, serviceName), nil
}

func getInstallPaths(opts InstallOptions, currentExec string) (targetExec, configPath string, shouldCopy bool, err error) {
	if opts.BinDir == "" {
		targetExec = currentExec
	} else {
		targetExec, err = GetExecutablePath(opts.BinDir)
		if err != nil {
			return "", "", false, fmt.Errorf("get executable path: %w", err)
		}
		shouldCopy = true
	}

	configPath = opts.ConfigPath
	if configPath == "" {
		configPath = GetInstallConfigPath(targetExec)
	}
	return targetExec, configPath, shouldCopy, nil
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy content: %w", err)
	}

	if runtime.GOOS != "windows" {
		if err := out.Chmod(0755); err != nil {
			return fmt.Errorf("set executable permission: %w", err)
		}
	}

	return out.Close()
}

// ensureConfig 确保配置文件存在，如果不存在则从 embedded template 创建
func ensureConfig(configPath string) error {
	if _, err := os.Stat(configPath); err == nil {
		return nil
	}

	// 创建父目录
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	// 从 embedded template 创建
	srcFile, err := configExample.Open("config.example.yaml")
	if err != nil {
		return fmt.Errorf("open embedded config.example.yaml: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("create config file: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	fmt.Printf("Created default config: %s\n", configPath)
	return nil
}

// Install 安装服务
func Install(opts InstallOptions) error {
	// 获取当前可执行文件路径
	currentExec, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get current executable: %w", err)
	}

	targetExec, configPath, shouldCopy, err := getInstallPaths(opts, currentExec)
	if err != nil {
		return err
	}

	if shouldCopy {
		if err := os.MkdirAll(filepath.Dir(targetExec), 0755); err != nil {
			return fmt.Errorf("create bin dir: %w", err)
		}
		if err := copyFile(currentExec, targetExec); err != nil {
			return fmt.Errorf("copy executable: %w", err)
		}
	}

	// 确保配置文件存在
	if err := ensureConfig(configPath); err != nil {
		return err
	}

	// 创建服务配置
	svcConfig := &service.Config{
		Name:        serviceName,
		DisplayName: "HDU SRun Login",
		Description: "HDU Campus Network Auto Login Service",
		Executable:  targetExec,
		Arguments:   []string{"--service", "--config", configPath},
	}

	// Linux 平台添加 systemd 依赖和配置
	if runtime.GOOS == "linux" {
		svcConfig.Dependencies = []string{
			"After=network-online.target",
			"Wants=network-online.target",
		}
		svcConfig.Option = service.KeyValue{
			"StartType":  "automatic",
			"Restart":    "on-failure",
			"RestartSec": "10",
			"WantedBy":   "network-online.target",
		}
	} else if runtime.GOOS == "windows" {
		svcConfig.Option = service.KeyValue{
			"StartType": "automatic",
		}
	}

	// 创建服务程序实例
	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}

	// 安装服务
	if err := s.Install(); err != nil {
		return getSystemdError("install service", fmt.Errorf("install service: %w", err))
	}

	fmt.Printf("Service %s installed successfully.\n", serviceName)
	fmt.Printf("Executable: %s\n", targetExec)
	fmt.Printf("Config: %s\n", configPath)

	return nil
}

// Uninstall 卸载服务
func Uninstall() error {
	// 创建服务配置
	svcConfig := &service.Config{
		Name:        serviceName,
		DisplayName: "HDU SRun Login",
		Description: "HDU Campus Network Auto Login Service",
	}

	// 创建服务程序实例
	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}

	// 停止服务（如果正在运行）
	_ = s.Stop()

	// 卸载服务
	if err := s.Uninstall(); err != nil {
		return getSystemdError("uninstall service", fmt.Errorf("uninstall service: %w", err))
	}

	// 删除 bin 目录中的程序
	binDir, err := GetDefaultBinDir()
	if err == nil {
		targetExec, _ := GetExecutablePath(binDir)
		_ = os.Remove(targetExec)
	}

	fmt.Printf("Service %s uninstalled successfully.\n", serviceName)
	return nil
}

// RunFunc 是服务模式运行时的回调函数
var RunFunc func(context.Context, string) error

// program 服务程序实现
type program struct {
	configPath      string
	ctx             context.Context
	cancel          context.CancelFunc
	done            chan error
	startupComplete chan struct{}
}

func (p *program) Start(s service.Service) error {
	ctx, cancel := context.WithCancel(context.Background())
	p.ctx = ctx
	p.cancel = cancel
	p.done = make(chan error, 1)
	p.startupComplete = make(chan struct{})
	defer close(p.startupComplete)

	go p.run()
	select {
	case err := <-p.done:
		if err != nil {
			return err
		}
		return fmt.Errorf("service stopped during startup")
	case <-time.After(1500 * time.Millisecond):
	}
	return nil
}

func (p *program) Stop(s service.Service) error {
	if p.cancel != nil {
		p.cancel()
	}
	if p.done != nil {
		select {
		case <-p.done:
		case <-time.After(5 * time.Second):
			return fmt.Errorf("service did not stop within 5s")
		}
	}
	return nil
}

func (p *program) run() {
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("service panic: %v", r)
			p.done <- err
			p.exitAfterStartupIfUnexpected(err)
		}
	}()

	if RunFunc != nil {
		ctx := p.ctx
		if ctx == nil {
			ctx = context.Background()
		}
		err := RunFunc(ctx, p.configPath)
		p.done <- err
		p.exitAfterStartupIfUnexpected(err)
		return
	}
	err := fmt.Errorf("service run function is not configured")
	p.done <- err
	p.exitAfterStartupIfUnexpected(err)
}

func (p *program) exitAfterStartupIfUnexpected(err error) {
	if err == nil || p.ctx == nil || p.ctx.Err() != nil {
		return
	}
	if p.startupComplete != nil {
		select {
		case <-p.startupComplete:
		default:
			return
		}
	}
	exitProcess(1)
}

// Run 以服务模式运行
func Run(configPath string) error {
	svcConfig := &service.Config{
		Name:        serviceName,
		DisplayName: "HDU SRun Login",
		Description: "HDU Campus Network Auto Login Service",
	}

	prg := &program{configPath: configPath}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		return err
	}

	return s.Run()
}

// FindConfig 查找配置文件（仅查找当前目录的 config.yaml）
func FindConfig(configPath string) (string, error) {
	// 1. 如果指定了路径，检查是否存在
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}
		return "", fmt.Errorf("config file not found: %s", configPath)
	}

	// 2. 当前目录下的 config.yaml
	if _, err := os.Stat("config.yaml"); err == nil {
		abs, _ := filepath.Abs("config.yaml")
		return abs, nil
	}

	return "", fmt.Errorf("config file not found")
}

// IsService 检查是否在服务模式运行
func IsService() bool {
	for _, arg := range os.Args {
		if arg == "--service" {
			return true
		}
	}
	return false
}

// getSystemdError 为 Linux systemd 服务操作失败添加权限提示
func getSystemdError(action string, err error) error {
	if err == nil {
		return nil
	}
	if runtime.GOOS != "linux" {
		return err
	}

	// 检查是否是 exit status 1 错误（systemctl 权限不足时常见）
	errMsg := err.Error()
	if strings.Contains(errMsg, "exit status 1") {
		return fmt.Errorf("%s: %s\nNote: this may require root privileges, try: sudo %s", action, errMsg, strings.Join(os.Args, " "))
	}
	return err
}

// StartService 启动服务
func StartService() error {
	svcConfig := &service.Config{
		Name:        serviceName,
		DisplayName: "HDU SRun Login",
		Description: "HDU Campus Network Auto Login Service",
	}

	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}

	if err := s.Start(); err != nil {
		return getSystemdError("start", fmt.Errorf("start service: %w", err))
	}
	if err := waitForServiceRunning(s, 5*time.Second); err != nil {
		return getSystemdError("start", err)
	}

	fmt.Printf("Service %s started successfully.\n", serviceName)
	return nil
}

func waitForServiceRunning(s service.Service, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastStatus service.Status
	for {
		status, err := s.Status()
		if err == nil {
			lastStatus = status
			if status == service.StatusRunning {
				return nil
			}
			if status == service.StatusStopped {
				return fmt.Errorf("start service: service stopped shortly after start")
			}
		}
		if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("start service: could not confirm service status: %w", err)
			}
			return fmt.Errorf("start service: service did not report running within %v (last status: %v)", timeout, lastStatus)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// StopService 停止服务
func StopService() error {
	svcConfig := &service.Config{
		Name:        serviceName,
		DisplayName: "HDU SRun Login",
		Description: "HDU Campus Network Auto Login Service",
	}

	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}

	if err := s.Stop(); err != nil {
		return getSystemdError("stop", fmt.Errorf("stop service: %w", err))
	}

	fmt.Printf("Service %s stopped successfully.\n", serviceName)
	return nil
}

// StatusService 查看服务状态
func StatusService() error {
	svcConfig := &service.Config{
		Name:        serviceName,
		DisplayName: "HDU SRun Login",
		Description: "HDU Campus Network Auto Login Service",
	}

	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}

	status, err := s.Status()
	if err != nil {
		return fmt.Errorf("get service status: %w", err)
	}

	switch status {
	case service.StatusRunning:
		fmt.Printf("Service %s is running.\n", serviceName)
	case service.StatusStopped:
		fmt.Printf("Service %s is stopped.\n", serviceName)
	case service.StatusUnknown:
		fmt.Printf("Service %s status is unknown.\n", serviceName)
	}

	return nil
}

// RestartService 重启服务
func RestartService() error {
	svcConfig := &service.Config{
		Name:        serviceName,
		DisplayName: "HDU SRun Login",
		Description: "HDU Campus Network Auto Login Service",
	}

	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}

	if err := s.Restart(); err != nil {
		return getSystemdError("restart", fmt.Errorf("restart service: %w", err))
	}

	fmt.Printf("Service %s restarted successfully.\n", serviceName)
	return nil
}

// EnsureAdminPrivileges 确保以管理员权限运行（Windows 需要）
func EnsureAdminPrivileges() error {
	if runtime.GOOS == "windows" {
		// 检查是否有管理员权限
		cmd := exec.Command("net", "session")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("install/uninstall service requires administrator privileges on Windows")
		}
	}
	return nil
}
