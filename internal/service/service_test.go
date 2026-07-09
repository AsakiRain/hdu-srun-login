package service

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestGetInstallConfigPathUsesExecutableDirectory(t *testing.T) {
	targetExec := filepath.Join("tmp", "bin", "hdu-srun-login")
	if runtime.GOOS == "windows" {
		targetExec += ".exe"
	}

	got := GetInstallConfigPath(targetExec)
	want := filepath.Join("tmp", "bin", "config.yaml")
	if got != want {
		t.Fatalf("install config path = %q, want %q", got, want)
	}
}

func TestGetInstallPathsUsesCurrentExecutableByDefault(t *testing.T) {
	currentExec := filepath.Join("opt", "bin", "hdu-srun-login", "hdu-srun-login.exe")

	targetExec, configPath, shouldCopy, err := getInstallPaths(InstallOptions{}, currentExec)
	if err != nil {
		t.Fatalf("getInstallPaths error = %v", err)
	}
	if targetExec != currentExec {
		t.Fatalf("target executable = %q, want current executable %q", targetExec, currentExec)
	}
	if shouldCopy {
		t.Fatal("shouldCopy = true, want false")
	}

	wantConfig := filepath.Join("opt", "bin", "hdu-srun-login", "config.yaml")
	if configPath != wantConfig {
		t.Fatalf("config path = %q, want %q", configPath, wantConfig)
	}
}

func TestGetInstallPathsHonorsExplicitConfig(t *testing.T) {
	currentExec := filepath.Join("opt", "bin", "hdu-srun-login.exe")
	explicitConfig := filepath.Join("configs", "hdu.yaml")

	_, configPath, _, err := getInstallPaths(InstallOptions{ConfigPath: explicitConfig}, currentExec)
	if err != nil {
		t.Fatalf("getInstallPaths error = %v", err)
	}
	if configPath != explicitConfig {
		t.Fatalf("config path = %q, want explicit config %q", configPath, explicitConfig)
	}
}

func TestProgramStartReturnsFastFailure(t *testing.T) {
	oldRunFunc := RunFunc
	oldExitProcess := exitProcess
	defer func() {
		RunFunc = oldRunFunc
		exitProcess = oldExitProcess
	}()
	exitProcess = func(int) {}

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	wantErr := errors.New("startup failed")
	RunFunc = func(context.Context, string) error {
		return wantErr
	}

	p := &program{configPath: configPath}
	err := p.Start(nil)
	if err == nil || !strings.Contains(err.Error(), wantErr.Error()) {
		t.Fatalf("Start error = %v, want %v", err, wantErr)
	}
}

func TestProgramStopCancelsRunContext(t *testing.T) {
	oldRunFunc := RunFunc
	oldExitProcess := exitProcess
	defer func() {
		RunFunc = oldRunFunc
		exitProcess = oldExitProcess
	}()
	exitProcess = func(int) {}

	started := make(chan struct{})
	RunFunc = func(ctx context.Context, _ string) error {
		close(started)
		<-ctx.Done()
		return nil
	}

	p := &program{}
	startErr := make(chan error, 1)
	go func() {
		startErr <- p.Start(nil)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("RunFunc did not start")
	}
	if err := <-startErr; err != nil {
		t.Fatalf("Start error = %v", err)
	}

	if err := p.Stop(nil); err != nil {
		t.Fatalf("Stop error = %v", err)
	}
}

func TestProgramUnexpectedExitAfterStartupExitsProcess(t *testing.T) {
	oldRunFunc := RunFunc
	oldExitProcess := exitProcess
	defer func() {
		RunFunc = oldRunFunc
		exitProcess = oldExitProcess
	}()

	started := make(chan struct{})
	release := make(chan struct{})
	exited := make(chan int, 1)
	RunFunc = func(context.Context, string) error {
		close(started)
		<-release
		return errors.New("unexpected worker exit")
	}
	exitProcess = func(code int) {
		exited <- code
	}

	p := &program{configPath: filepath.Join(t.TempDir(), "config.yaml")}
	startErr := make(chan error, 1)
	go func() {
		startErr <- p.Start(nil)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("RunFunc did not start")
	}
	if err := <-startErr; err != nil {
		t.Fatalf("Start error = %v", err)
	}

	close(release)
	select {
	case code := <-exited:
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
	case <-time.After(time.Second):
		t.Fatal("process exit was not requested")
	}
}
