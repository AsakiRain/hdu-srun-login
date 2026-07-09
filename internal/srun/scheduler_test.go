package srun

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func waitForChecks(t *testing.T, checks <-chan int, want int, timeout time.Duration) {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case got := <-checks:
			if got >= want {
				return
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for check #%d", want)
		}
	}
}

func TestSchedulerRunsInitialCheck(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	checks := make(chan int, 4)
	var count int32
	s := &Scheduler{
		Check: func(context.Context) error {
			checks <- int(atomic.AddInt32(&count, 1))
			return nil
		},
		Logger:              noopLogger{},
		CheckInterval:       time.Hour,
		UnreachableInterval: time.Hour,
	}

	go s.Run(ctx)
	waitForChecks(t, checks, 1, 100*time.Millisecond)
}

func TestSchedulerNetworkEventBreaksUnreachableSlowCheck(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	network := make(chan struct{}, 1)
	checks := make(chan int, 4)
	var count int32
	s := &Scheduler{
		Check: func(context.Context) error {
			n := atomic.AddInt32(&count, 1)
			checks <- int(n)
			if n == 1 {
				return ErrNoLoginHost
			}
			return nil
		},
		Network:             network,
		Logger:              noopLogger{},
		CheckInterval:       10 * time.Millisecond,
		UnreachableInterval: time.Hour,
	}

	go s.Run(ctx)
	waitForChecks(t, checks, 1, 100*time.Millisecond)
	network <- struct{}{}
	waitForChecks(t, checks, 2, 100*time.Millisecond)
}

func TestSchedulerSlowsAfterNoLoginHost(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	checks := make(chan int, 4)
	var count int32
	s := &Scheduler{
		Check: func(context.Context) error {
			checks <- int(atomic.AddInt32(&count, 1))
			return ErrNoLoginHost
		},
		Logger:              noopLogger{},
		CheckInterval:       20 * time.Millisecond,
		UnreachableInterval: 200 * time.Millisecond,
	}

	go s.Run(ctx)
	waitForChecks(t, checks, 1, 100*time.Millisecond)
	select {
	case got := <-checks:
		t.Fatalf("unexpected check #%d before unreachable interval", got)
	case <-time.After(80 * time.Millisecond):
	}
}

func TestSchedulerRestoresNormalIntervalAfterReachableCheck(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	network := make(chan struct{}, 1)
	checks := make(chan int, 8)
	var count int32
	s := &Scheduler{
		Check: func(context.Context) error {
			n := atomic.AddInt32(&count, 1)
			checks <- int(n)
			if n == 1 {
				return ErrNoLoginHost
			}
			return nil
		},
		Network:             network,
		Logger:              noopLogger{},
		CheckInterval:       20 * time.Millisecond,
		UnreachableInterval: time.Hour,
	}

	go s.Run(ctx)
	waitForChecks(t, checks, 1, 100*time.Millisecond)
	network <- struct{}{}
	waitForChecks(t, checks, 2, 100*time.Millisecond)
	waitForChecks(t, checks, 3, 100*time.Millisecond)
}

func TestSchedulerDoesNotSlowForOtherErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	checks := make(chan int, 4)
	var count int32
	s := &Scheduler{
		Check: func(context.Context) error {
			checks <- int(atomic.AddInt32(&count, 1))
			return errors.New("temporary failure")
		},
		Logger:              noopLogger{},
		CheckInterval:       20 * time.Millisecond,
		UnreachableInterval: time.Hour,
	}

	go s.Run(ctx)
	waitForChecks(t, checks, 1, 100*time.Millisecond)
	waitForChecks(t, checks, 2, 100*time.Millisecond)
}
