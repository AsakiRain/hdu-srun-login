//go:build linux

package srun

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
)

func watchNetworkChanges(ctx context.Context, events chan<- string, w *NetworkWatcher) error {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.NETLINK_ROUTE)
	if err != nil {
		return err
	}
	defer unix.Close(fd)

	groups := unix.RTMGRP_LINK | unix.RTMGRP_IPV4_IFADDR | unix.RTMGRP_IPV6_IFADDR
	if err := unix.Bind(fd, &unix.SockaddrNetlink{Family: unix.AF_NETLINK, Groups: uint32(groups)}); err != nil {
		return err
	}
	w.logger.Logf("DEBUG", "Linux network watcher started (groups: link, ipv4_ifaddr, ipv6_ifaddr)")

	go func() {
		<-ctx.Done()
		_ = unix.Close(fd)
	}()

	buf := make([]byte, 8192)
	for {
		n, _, err := unix.Recvfrom(fd, buf, 0)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, unix.EBADF) || errors.Is(err, unix.EINTR) {
				return nil
			}
			return err
		}
		for _, messageType := range parseLinuxNetlinkMessageTypes(buf[:n]) {
			if isLinuxInterfaceEvent(messageType) {
				reason := linuxNetlinkChangeReason(messageType)
				w.logger.Logf("DEBUG", "Linux network watcher accepted netlink event: %s (%s)", linuxNetlinkMessageName(messageType), reason)
				emitNetworkEvent(ctx, events, reason)
			} else {
				w.logger.Logf("DEBUG", "Linux network watcher ignored netlink event: %s", linuxNetlinkMessageName(messageType))
			}
		}
	}
}

func linuxNetlinkChangeReason(messageType uint16) string {
	switch messageType {
	case unix.RTM_NEWADDR, unix.RTM_DELADDR:
		return "IP address changed"
	case unix.RTM_NEWLINK:
		return "network interface added or changed"
	case unix.RTM_DELLINK:
		return "network interface removed"
	default:
		return "network state changed"
	}
}

func parseLinuxNetlinkMessageTypes(buf []byte) []uint16 {
	const netlinkHeaderLen = 16

	order := nativeEndian()
	types := make([]uint16, 0, 4)
	for len(buf) >= netlinkHeaderLen {
		messageLen := int(order.Uint32(buf[0:4]))
		if messageLen < netlinkHeaderLen || messageLen > len(buf) {
			break
		}
		types = append(types, order.Uint16(buf[4:6]))

		alignedLen := (messageLen + 3) &^ 3
		if alignedLen > len(buf) {
			break
		}
		buf = buf[alignedLen:]
	}
	return types
}

func nativeEndian() binary.ByteOrder {
	var x uint16 = 0x0102
	if *(*byte)(unsafe.Pointer(&x)) == 0x02 {
		return binary.LittleEndian
	}
	return binary.BigEndian
}

func isLinuxInterfaceEvent(messageType uint16) bool {
	switch messageType {
	case unix.RTM_NEWLINK, unix.RTM_DELLINK, unix.RTM_NEWADDR, unix.RTM_DELADDR:
		return true
	default:
		return false
	}
}

func linuxNetlinkMessageName(messageType uint16) string {
	switch messageType {
	case unix.RTM_NEWLINK:
		return "RTM_NEWLINK"
	case unix.RTM_DELLINK:
		return "RTM_DELLINK"
	case unix.RTM_NEWADDR:
		return "RTM_NEWADDR"
	case unix.RTM_DELADDR:
		return "RTM_DELADDR"
	case unix.RTM_NEWROUTE:
		return "RTM_NEWROUTE"
	case unix.RTM_DELROUTE:
		return "RTM_DELROUTE"
	case unix.RTM_GETROUTE:
		return "RTM_GETROUTE"
	default:
		return fmt.Sprintf("type=%d", messageType)
	}
}
