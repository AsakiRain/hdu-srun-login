package srun

import (
	"context"
	"errors"
	"time"
)

type Scheduler struct {
	Check   func(context.Context) error
	Refresh func(context.Context) error
	Network <-chan struct{}
	Logger  Logger

	CheckInterval       time.Duration
	RefreshInterval     time.Duration
	UnreachableInterval time.Duration
}

func (s *Scheduler) Run(ctx context.Context) {
	logger := s.Logger
	if logger == nil {
		logger = noopLogger{}
	}

	checkInterval := s.CheckInterval
	if checkInterval <= 0 {
		checkInterval = 2 * time.Minute
	}
	unreachableInterval := s.UnreachableInterval
	if unreachableInterval <= 0 {
		unreachableInterval = DefaultUnreachableCheckInterval
	}

	activeCheckInterval := checkInterval
	checkTimer := time.NewTimer(0)
	defer checkTimer.Stop()

	var refreshTicker *time.Ticker
	var refreshChan <-chan time.Time
	if s.RefreshInterval > 0 && s.Refresh != nil {
		refreshTicker = time.NewTicker(s.RefreshInterval)
		refreshChan = refreshTicker.C
		defer refreshTicker.Stop()
	}

	runCheck := func(reason string) {
		if reason != "" {
			logger.Logf("INFO", "Running status check (%s)", reason)
		}
		if s.Check == nil {
			return
		}
		if err := s.Check(ctx); err != nil {
			if errors.Is(err, ErrNoLoginHost) {
				if activeCheckInterval != unreachableInterval {
					logger.Logf("INFO", "Login endpoint unreachable; slowing connectivity checks to %v until the network changes", unreachableInterval)
				}
				activeCheckInterval = unreachableInterval
				return
			}
			logger.Logf("ERROR", "check failed: %v", err)
			return
		}
		if activeCheckInterval != checkInterval {
			logger.Logf("INFO", "Login endpoint reachable again; restoring status check interval to %v", checkInterval)
		}
		activeCheckInterval = checkInterval
	}

	resetCheckTimer := func(d time.Duration) {
		if !checkTimer.Stop() {
			select {
			case <-checkTimer.C:
			default:
			}
		}
		checkTimer.Reset(d)
	}

	networkEvents := s.Network
	for {
		select {
		case <-ctx.Done():
			logger.Logf("INFO", "Process stopped")
			return
		case <-checkTimer.C:
			runCheck("scheduled")
			resetCheckTimer(activeCheckInterval)
		case _, ok := <-networkEvents:
			if !ok {
				networkEvents = nil
				continue
			}
			runCheck("network changed")
			resetCheckTimer(activeCheckInterval)
		case <-refreshChan:
			if err := s.Refresh(ctx); err != nil {
				logger.Logf("ERROR", "refresh failed: %v", err)
			}
		}
	}
}
