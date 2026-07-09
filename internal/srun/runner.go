package srun

import (
	"context"
	"sync"
)

type Runner struct {
	auth           Auth
	logger         Logger
	mu             sync.Mutex
	bindInterfaces []string
}

func NewRunner(auth Auth, logger Logger, bindInterfaces []string) *Runner {
	if logger == nil {
		logger = noopLogger{}
	}
	return &Runner{
		auth:           auth,
		logger:         logger,
		bindInterfaces: bindInterfaces,
	}
}

func (r *Runner) Refresh(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.logger.Logf("INFO", "Refreshing login session...")
	client, err := NewClient(r.auth.Username, r.auth.Password, r.logger, r.bindInterfaces)
	if err != nil {
		return err
	}
	if _, err := client.Logout(ctx); err != nil {
		return err
	}

	_, err = client.Login(ctx)
	return err
}

func (r *Runner) Check(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.logger.Logf("INFO", "Checking online status...")
	client, err := NewClient("", "", r.logger, r.bindInterfaces)
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

	r.logger.Logf("WARNING", "Not online (%s), attempting login...", stringValue(status, "error"))
	client.Username = r.auth.Username
	client.Password = r.auth.Password
	_, err = client.Login(ctx)
	return err
}
