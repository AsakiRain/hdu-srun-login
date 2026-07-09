//go:build !windows && !linux && !darwin

package srun

import "context"

func watchNetworkChanges(ctx context.Context, events chan<- string, w *NetworkWatcher) error {
	<-ctx.Done()
	return nil
}
