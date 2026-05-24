package srun

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

type Runner struct {
	auths          []Auth
	logger         Logger
	rand           *rand.Rand
	mu             sync.Mutex
	bindInterfaces []string
}

func NewRunner(auths []Auth, logger Logger, bindInterfaces []string) *Runner {
	if logger == nil {
		logger = noopLogger{}
	}
	return &Runner{
		auths:          append([]Auth(nil), auths...),
		logger:         logger,
		rand:           rand.New(rand.NewSource(time.Now().UnixNano())),
		bindInterfaces: bindInterfaces,
	}
}

func (r *Runner) randomAuth() Auth {
	return r.auths[r.rand.Intn(len(r.auths))]
}

func (r *Runner) Refresh(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.logger.Logf("INFO", "Refreshing login session...")
	auth := r.randomAuth()
	client, err := NewClient(auth.Username, auth.Password, r.logger, r.bindInterfaces)
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
		r.logger.Logf("INFO", "Already online (user: %s, ip: %s)", stringValue(status, "user_name"), stringValue(status, "online_ip"))
		return nil
	}

	r.logger.Logf("WARNING", "Not online (%s), attempting login...", stringValue(status, "error"))
	auth := r.randomAuth()
	client.Username = auth.Username
	client.Password = auth.Password
	_, err = client.Login(ctx)
	return err
}
