//go:build windows

package srun

import (
	"context"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

func watchNetworkChanges(ctx context.Context, events chan<- string, _ *NetworkWatcher) error {
	callback := windows.NewCallback(func(_ unsafe.Pointer, _ unsafe.Pointer, _ uint32) uintptr {
		emitNetworkEvent(ctx, events, "Windows IP interface changed")
		return 0
	})

	var handle windows.Handle
	if err := windows.NotifyIpInterfaceChange(windows.AF_UNSPEC, callback, nil, false, &handle); err != nil {
		return err
	}
	defer windows.CancelMibChangeNotify2(handle)

	<-ctx.Done()
	runtime.KeepAlive(callback)
	return nil
}
