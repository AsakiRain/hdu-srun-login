package srun

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

type Runner struct {
	auths  []Auth
	logger Logger
	rand   *rand.Rand
	mu     sync.Mutex
}

func NewRunner(auths []Auth, logger Logger) *Runner {
	if logger == nil {
		logger = noopLogger{}
	}
	return &Runner{
		auths:  append([]Auth(nil), auths...),
		logger: logger,
		rand:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (r *Runner) randomAuth() Auth {
	return r.auths[r.rand.Intn(len(r.auths))]
}

func (r *Runner) Refresh(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.logger.Logf("DEBUG", "Try to refresh...")
	auth := r.randomAuth()
	client, err := NewClient(auth.Username, auth.Password, r.logger)
	if err != nil {
		return err
	}
	if _, err := client.Logout(ctx); err != nil {
		return err
	}

	for {
		result, err := client.Login(ctx)
		if err != nil {
			return err
		}
		if stringValue(result, "error_msg") != "4xx" {
			return nil
		}

		auth = r.randomAuth()
		client.Username = auth.Username
		client.Password = auth.Password
		r.logger.Logf("DEBUG", "username or password is incorrect, retry in 2 seconds...")
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (r *Runner) Check(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.logger.Logf("DEBUG", "Check status...")
	client, err := NewClient("", "", r.logger)
	if err != nil {
		return err
	}
	status, err := client.Check(ctx)
	if err != nil {
		return err
	}
	if stringValue(status, "error") == "ok" {
		return nil
	}

	r.logger.Logf("WARNING", "%s, try to login...", stringValue(status, "error"))
	for {
		auth := r.randomAuth()
		client.Username = auth.Username
		client.Password = auth.Password
		result, err := client.Login(ctx)
		if err != nil {
			return err
		}
		if stringValue(result, "error_msg") != "4xx" {
			return nil
		}

		r.logger.Logf("DEBUG", "username or password is incorrect, retry in 2 seconds...")
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}
