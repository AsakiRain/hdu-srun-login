package srun

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"
)

const (
	DefaultNetworkPollInterval      = 2 * time.Second
	DefaultNetworkEventDebounce     = time.Second
	DefaultUnreachableCheckInterval = 10 * time.Minute
)

type NetworkWatcher struct {
	Debounce     time.Duration
	PollInterval time.Duration
	Snapshot     func() (string, error)
	logger       Logger
}

func NewNetworkWatcher(logger Logger) *NetworkWatcher {
	if logger == nil {
		logger = noopLogger{}
	}
	return &NetworkWatcher{
		Debounce:     DefaultNetworkEventDebounce,
		PollInterval: DefaultNetworkPollInterval,
		Snapshot:     NetworkFingerprint,
		logger:       logger,
	}
}

func (w *NetworkWatcher) Watch(ctx context.Context) <-chan struct{} {
	events := make(chan struct{}, 1)
	debounce := w.Debounce
	if debounce <= 0 {
		debounce = DefaultNetworkEventDebounce
	}

	rawEvents := make(chan string, 1)
	go func() {
		if err := watchNetworkChanges(ctx, rawEvents, w); err != nil && ctx.Err() == nil {
			w.logger.Logf("WARNING", "Native network watcher failed: %v; polling will continue", err)
		}
	}()
	go w.watchByPolling(ctx, rawEvents)

	go func() {
		defer close(events)

		var timer *time.Timer
		var timerChan <-chan time.Time
		lastReason := "network state changed"
		for {
			select {
			case <-ctx.Done():
				if timer != nil {
					timer.Stop()
				}
				return
			case reason, ok := <-rawEvents:
				if !ok {
					if timer != nil {
						timer.Stop()
					}
					return
				}
				if reason != "" {
					lastReason = reason
				}
				if timer == nil {
					timer = time.NewTimer(debounce)
					timerChan = timer.C
					continue
				}
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(debounce)
			case <-timerChan:
				w.logger.Logf("INFO", "Network environment changed (%s), scheduling an immediate check", lastReason)
				select {
				case events <- struct{}{}:
				default:
				}
				lastReason = "network state changed"
				timerChan = nil
				timer = nil
			}
		}
	}()

	return events
}

func (w *NetworkWatcher) watchByPolling(ctx context.Context, events chan<- string) {
	interval := w.PollInterval
	if interval <= 0 {
		interval = DefaultNetworkPollInterval
	}
	snapshot := w.Snapshot
	if snapshot == nil {
		snapshot = NetworkFingerprint
	}

	last, err := snapshot()
	if err != nil {
		w.logger.Logf("DEBUG", "Initial network snapshot failed: %v", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			current, err := snapshot()
			if err != nil {
				w.logger.Logf("DEBUG", "Network snapshot failed: %v", err)
				continue
			}
			if current == last {
				continue
			}
			reason := classifyNetworkFingerprintChange(last, current)
			last = current
			w.logger.Logf("DEBUG", "Network polling watcher detected interface snapshot change: %s", reason)
			emitNetworkEvent(ctx, events, reason)
		}
	}
}

func emitNetworkEvent(ctx context.Context, events chan<- string, reason string) {
	select {
	case events <- reason:
	case <-ctx.Done():
	default:
	}
}

func NetworkFingerprint() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	parts := make([]string, 0, len(ifaces))
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			return "", fmt.Errorf("interface %s addresses: %w", iface.Name, err)
		}
		addrStrings := make([]string, 0, len(addrs))
		for _, addr := range addrs {
			addrStrings = append(addrStrings, addr.String())
		}
		sort.Strings(addrStrings)

		parts = append(parts, fmt.Sprintf("%d|%s|%s|%d|%s|%s",
			iface.Index,
			iface.Name,
			iface.Flags.String(),
			iface.MTU,
			iface.HardwareAddr.String(),
			strings.Join(addrStrings, ","),
		))
	}

	sort.Strings(parts)
	return strings.Join(parts, "\n"), nil
}

func classifyNetworkFingerprintChange(oldFingerprint, newFingerprint string) string {
	oldInterfaces := parseNetworkFingerprint(oldFingerprint)
	newInterfaces := parseNetworkFingerprint(newFingerprint)

	for key := range oldInterfaces {
		if _, ok := newInterfaces[key]; !ok {
			return "network interface removed"
		}
	}
	for key := range newInterfaces {
		if _, ok := oldInterfaces[key]; !ok {
			return "network interface added"
		}
	}
	for key, oldIface := range oldInterfaces {
		newIface := newInterfaces[key]
		if oldIface.addrs != newIface.addrs {
			return "IP address changed"
		}
		if oldIface.flags != newIface.flags || oldIface.mtu != newIface.mtu || oldIface.mac != newIface.mac {
			return "network interface state changed"
		}
	}
	return "network state changed"
}

type networkFingerprintEntry struct {
	flags string
	mtu   string
	mac   string
	addrs string
}

func parseNetworkFingerprint(fingerprint string) map[string]networkFingerprintEntry {
	result := make(map[string]networkFingerprintEntry)
	for _, line := range strings.Split(fingerprint, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 6)
		if len(parts) != 6 {
			continue
		}
		key := parts[0] + "|" + parts[1]
		result[key] = networkFingerprintEntry{
			flags: parts[2],
			mtu:   parts[3],
			mac:   parts[4],
			addrs: parts[5],
		}
	}
	return result
}
