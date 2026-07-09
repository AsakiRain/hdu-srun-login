//go:build darwin

package srun

import (
	"context"
	"errors"

	"golang.org/x/sys/unix"
)

func watchNetworkChanges(ctx context.Context, events chan<- string, _ *NetworkWatcher) error {
	fd, err := unix.Socket(unix.AF_ROUTE, unix.SOCK_RAW, unix.AF_UNSPEC)
	if err != nil {
		return err
	}
	defer unix.Close(fd)

	go func() {
		<-ctx.Done()
		_ = unix.Close(fd)
	}()

	buf := make([]byte, 8192)
	for {
		n, err := unix.Read(fd, buf)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, unix.EBADF) || errors.Is(err, unix.EINTR) {
				return nil
			}
			return err
		}
		if n < 4 {
			continue
		}
		messageType := buf[3]
		if isDarwinInterfaceEvent(messageType) {
			emitNetworkEvent(ctx, events, darwinRouteChangeReason(messageType))
		}
	}
}

func isDarwinInterfaceEvent(messageType byte) bool {
	switch int(messageType) {
	case unix.RTM_IFINFO, unix.RTM_IFINFO2, unix.RTM_NEWADDR, unix.RTM_DELADDR:
		return true
	default:
		return false
	}
}

func darwinRouteChangeReason(messageType byte) string {
	switch int(messageType) {
	case unix.RTM_NEWADDR, unix.RTM_DELADDR:
		return "IP address changed"
	case unix.RTM_IFINFO, unix.RTM_IFINFO2:
		return "network interface state changed"
	default:
		return "network state changed"
	}
}
