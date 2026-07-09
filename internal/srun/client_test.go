package srun

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestEnsureHostReturnsErrNoLoginHost(t *testing.T) {
	oldHosts := defaultHosts
	defaultHosts = []string{"http://login.invalid"}
	defer func() { defaultHosts = oldHosts }()

	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("dial failed")
		})},
		logger: noopLogger{},
	}

	err := client.ensureHost(context.Background())
	if !errors.Is(err, ErrNoLoginHost) {
		t.Fatalf("ensureHost error = %v, want ErrNoLoginHost", err)
	}
}

func TestEnsureHostAcceptsStatusBefore500(t *testing.T) {
	oldHosts := defaultHosts
	defaultHosts = []string{"http://first.invalid", "http://second.invalid"}
	defer func() { defaultHosts = oldHosts }()

	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			status := http.StatusForbidden
			if req.URL.Host == "first.invalid" {
				status = http.StatusInternalServerError
			}
			return &http.Response{
				StatusCode: status,
				Status:     http.StatusText(status),
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		})},
		logger: noopLogger{},
	}

	if err := client.ensureHost(context.Background()); err != nil {
		t.Fatalf("ensureHost error = %v, want nil", err)
	}
	if client.Host != "http://second.invalid" {
		t.Fatalf("client.Host = %q, want second host", client.Host)
	}
}

func TestEnsureHostSkips5xx(t *testing.T) {
	oldHosts := defaultHosts
	defaultHosts = []string{"http://first.invalid", "http://second.invalid"}
	defer func() { defaultHosts = oldHosts }()

	seen := make([]string, 0, 2)
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			seen = append(seen, req.URL.Host)
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Status:     "502 Bad Gateway",
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		})},
		logger: noopLogger{},
	}

	err := client.ensureHost(context.Background())
	if !errors.Is(err, ErrNoLoginHost) {
		t.Fatalf("ensureHost error = %v, want ErrNoLoginHost", err)
	}
	if len(seen) != 2 {
		t.Fatalf("tried %d hosts, want 2", len(seen))
	}
}
