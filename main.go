package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/JBNRZ/srun-login-go/internal/srun"
)

const logMaxSize = 10 * 1024 * 1024

type rotatingWriter struct {
	path string
	max  int64
	mu   sync.Mutex
	file *os.File
	size int64
}

func newRotatingWriter(path string, max int64) (*rotatingWriter, error) {
	w := &rotatingWriter{path: path, max: max}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *rotatingWriter) open() error {
	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return err
	}
	w.file = file
	w.size = info.Size()
	return nil
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	if w.size+int64(len(p)) > w.max {
		if err := w.file.Close(); err != nil {
			return 0, err
		}
		_ = os.Remove(w.path + ".1")
		_ = os.Rename(w.path, w.path+".1")
		w.file = nil
		if err := w.open(); err != nil {
			return 0, err
		}
	}

	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

type logger struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *logger) Logf(level string, format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	line := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintf(l.w, "%s [%s] srun_login | %s\n", time.Now().Format("01-02 15:04:05"), level, line)
}

func loadAuths(path string) ([]srun.Auth, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var auths []srun.Auth
	if err := json.NewDecoder(file).Decode(&auths); err != nil {
		return nil, err
	}
	if len(auths) == 0 {
		return nil, errors.New("auth list is empty")
	}
	for i, auth := range auths {
		if auth.Username == "" || auth.Password == "" {
			return nil, fmt.Errorf("auth[%d] must include username and password", i)
		}
	}
	return auths, nil
}

func main() {
	configPath := flag.String("config", "auth.json", "auth JSON file")
	once := flag.Bool("once", false, "check status once and login when needed")
	checkInterval := flag.Duration("check-interval", 2*time.Minute, "status check interval")
	refreshInterval := flag.Duration("refresh-interval", 6*time.Hour, "logout/login refresh interval")
	flag.Parse()

	logFile, err := newRotatingWriter("srun_login.log", logMaxSize)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open log file: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	log := &logger{w: io.MultiWriter(os.Stdout, logFile)}
	auths, err := loadAuths(*configPath)
	if err != nil {
		log.Logf("ERROR", "%v, please check %s", err, *configPath)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runner := srun.NewRunner(auths, log)
	log.Logf("INFO", "Process started")

	if *once {
		if err := runner.Check(ctx); err != nil {
			log.Logf("ERROR", "check failed: %v", err)
			os.Exit(1)
		}
		return
	}

	checkTicker := time.NewTicker(*checkInterval)
	refreshTicker := time.NewTicker(*refreshInterval)
	defer checkTicker.Stop()
	defer refreshTicker.Stop()

	if err := runner.Check(ctx); err != nil {
		log.Logf("ERROR", "check failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Logf("INFO", "Process stopped")
			return
		case <-checkTicker.C:
			if err := runner.Check(ctx); err != nil {
				log.Logf("ERROR", "check failed: %v", err)
			}
		case <-refreshTicker.C:
			if err := runner.Refresh(ctx); err != nil {
				log.Logf("ERROR", "refresh failed: %v", err)
			}
		}
	}
}
