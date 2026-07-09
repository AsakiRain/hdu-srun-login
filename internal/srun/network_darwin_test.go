//go:build darwin

package srun

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestDarwinInterfaceEventFilterIgnoresRoutes(t *testing.T) {
	if !isDarwinInterfaceEvent(byte(unix.RTM_IFINFO)) {
		t.Fatal("RTM_IFINFO should be treated as an interface event")
	}
	if !isDarwinInterfaceEvent(byte(unix.RTM_NEWADDR)) {
		t.Fatal("RTM_NEWADDR should be treated as an address event")
	}
	if isDarwinInterfaceEvent(byte(unix.RTM_ADD)) {
		t.Fatal("RTM_ADD should be ignored")
	}
	if isDarwinInterfaceEvent(byte(unix.RTM_DELETE)) {
		t.Fatal("RTM_DELETE should be ignored")
	}
}

func TestDarwinRouteChangeReason(t *testing.T) {
	tests := map[byte]string{
		byte(unix.RTM_IFINFO):  "network interface state changed",
		byte(unix.RTM_NEWADDR): "IP address changed",
		byte(unix.RTM_DELADDR): "IP address changed",
	}

	for messageType, want := range tests {
		if got := darwinRouteChangeReason(messageType); got != want {
			t.Fatalf("darwinRouteChangeReason(%d) = %q, want %q", messageType, got, want)
		}
	}
}
