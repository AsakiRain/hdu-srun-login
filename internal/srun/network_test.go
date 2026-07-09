package srun

import (
	"context"
	"testing"
	"time"
)

func TestNetworkWatcherPollingDetectsSnapshotChanges(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	snapshots := make(chan string, 4)
	snapshots <- "iface=tun0 down"
	snapshots <- "iface=tun0 down"
	snapshots <- "iface=tun0 up"

	w := &NetworkWatcher{
		Debounce:     time.Millisecond,
		PollInterval: time.Millisecond,
		Snapshot: func() (string, error) {
			select {
			case snapshot := <-snapshots:
				return snapshot, nil
			default:
				return "iface=tun0 up", nil
			}
		},
		logger: noopLogger{},
	}

	events := make(chan string, 1)
	go w.watchByPolling(ctx, events)

	select {
	case reason := <-events:
		if reason == "" {
			t.Fatal("polling watcher emitted empty reason")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("polling watcher did not detect snapshot change")
	}
}

func TestClassifyNetworkFingerprintChange(t *testing.T) {
	base := "1|eth0|up|1500|aa:bb|10.0.0.1/24"

	tests := []struct {
		name string
		old  string
		new  string
		want string
	}{
		{
			name: "interface added",
			old:  base,
			new:  base + "\n2|tun0|up|1500||198.18.0.1/15",
			want: "network interface added",
		},
		{
			name: "interface removed",
			old:  base + "\n2|tun0|up|1500||198.18.0.1/15",
			new:  base,
			want: "network interface removed",
		},
		{
			name: "ip changed",
			old:  base,
			new:  "1|eth0|up|1500|aa:bb|10.0.0.2/24",
			want: "IP address changed",
		},
		{
			name: "state changed",
			old:  base,
			new:  "1|eth0|down|1500|aa:bb|10.0.0.1/24",
			want: "network interface state changed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyNetworkFingerprintChange(tt.old, tt.new); got != tt.want {
				t.Fatalf("classifyNetworkFingerprintChange = %q, want %q", got, tt.want)
			}
		})
	}
}
