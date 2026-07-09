//go:build linux

package srun

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestLinuxInterfaceEventFilterIgnoresRoutes(t *testing.T) {
	if !isLinuxInterfaceEvent(unix.RTM_NEWLINK) {
		t.Fatal("RTM_NEWLINK should be treated as an interface event")
	}
	if !isLinuxInterfaceEvent(unix.RTM_NEWADDR) {
		t.Fatal("RTM_NEWADDR should be treated as an address event")
	}
	if isLinuxInterfaceEvent(unix.RTM_NEWROUTE) {
		t.Fatal("RTM_NEWROUTE should be ignored")
	}
	if isLinuxInterfaceEvent(unix.RTM_DELROUTE) {
		t.Fatal("RTM_DELROUTE should be ignored")
	}
}

func TestLinuxNetlinkMessageName(t *testing.T) {
	tests := map[uint16]string{
		unix.RTM_NEWLINK:  "RTM_NEWLINK",
		unix.RTM_NEWADDR:  "RTM_NEWADDR",
		unix.RTM_NEWROUTE: "RTM_NEWROUTE",
		65535:             "type=65535",
	}

	for messageType, want := range tests {
		if got := linuxNetlinkMessageName(messageType); got != want {
			t.Fatalf("linuxNetlinkMessageName(%d) = %q, want %q", messageType, got, want)
		}
	}
}

func TestLinuxNetlinkChangeReason(t *testing.T) {
	tests := map[uint16]string{
		unix.RTM_NEWLINK: "network interface added or changed",
		unix.RTM_DELLINK: "network interface removed",
		unix.RTM_NEWADDR: "IP address changed",
		unix.RTM_DELADDR: "IP address changed",
	}

	for messageType, want := range tests {
		if got := linuxNetlinkChangeReason(messageType); got != want {
			t.Fatalf("linuxNetlinkChangeReason(%d) = %q, want %q", messageType, got, want)
		}
	}
}
