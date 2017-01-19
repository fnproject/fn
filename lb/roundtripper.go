package lb

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/golang/groupcache/singleflight"
)

// ErrNoFallbackNodeFound happens when the fallback routine does not manage to
// find a TCP reachable node in alternative to the chosen one.
var ErrNoFallbackNodeFound = errors.New("no fallback node found - whole cluster seems offline")

// FallbackRoundTripper implements http.RoundTripper in a way that when an
// outgoing request does not manage to succeed with its original target host,
// it fallsback to a list of alternative hosts. Internally it keeps a list of
// dead hosts, and pings them until they are back online, diverting traffic
// back to them. This is meant to be used by ConsistentHashReverseProxy().
type FallbackRoundTripper struct {
	nodes []string
	sf    singleflight.Group

	mu sync.Mutex
	// a list of offline servers that must be rechecked to see when they
	// get back online. If a server is in this list, it must have a fallback
	// available to which requests are sent.
	fallback map[string]string
}

// NewRoundTripper creates a new FallbackRoundTripper and triggers the internal
// host TCP health checks.
func NewRoundTripper(ctx context.Context, nodes []string) *FallbackRoundTripper {
	frt := &FallbackRoundTripper{
		nodes:    nodes,
		fallback: make(map[string]string),
	}
	go frt.checkHealth(ctx)
	return frt
}

func (f *FallbackRoundTripper) checkHealth(ctx context.Context) {
	tick := time.NewTicker(1 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			f.mu.Lock()
			if len(f.fallback) == 0 {
				f.mu.Unlock()
				continue
			}
			fallback := make(map[string]string)
			for k, v := range f.fallback {
				fallback[k] = v
			}
			f.mu.Unlock()

			updatedlist := make(map[string]string)
			for host, target := range fallback {
				if !f.ping(host) {
					updatedlist[host] = target
				}
			}

			f.mu.Lock()
			f.fallback = make(map[string]string)
			for k, v := range updatedlist {
				f.fallback[k] = v
			}
			f.mu.Unlock()
		}
	}
}

func (f *FallbackRoundTripper) ping(host string) bool {
	conn, err := net.Dial("tcp", host)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func (f *FallbackRoundTripper) fallbackHost(targetHost, failedFallback string) string {
	detected, err := f.sf.Do(targetHost, func() (interface{}, error) {
		for _, node := range f.nodes {
			if node != targetHost && node != failedFallback && f.ping(node) {
				f.mu.Lock()
				f.fallback[targetHost] = node
				f.mu.Unlock()
				return node, nil
			}
		}
		return "", ErrNoFallbackNodeFound
	})

	if err != nil {
		return ""
	}
	return detected.(string)

}

// RoundTrip implements http.RoundTrip. It tried to fullfil an *http.Request to
// its original target host, falling back to a list of nodes in case of failure.
// After the first failure, it consistently delivers traffic to the fallback
// host, until original host is back online. If no fallback node is available,
// it fails with ErrNoFallbackNodeFound. In case of cascaded failure, that is,
// the fallback node is also offline, it will look for another online host.
func (f *FallbackRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	targetHost := req.URL.Host

	f.mu.Lock()
	fallback, ok := f.fallback[targetHost]
	f.mu.Unlock()
	if ok {
		req.URL.Host = fallback
		resp, err := f.callNode(req)
		if err == nil {
			return resp, err
		}
		fallback := f.fallbackHost(targetHost, fallback)
		if fallback == "" {
			return nil, ErrNoFallbackNodeFound
		}
		req.URL.Host = fallback
		return f.callNode(req)
	}

	resp, err := f.callNode(req)
	if err == nil {
		return resp, err
	}

	fallback = f.fallbackHost(targetHost, "")
	if fallback == "" {
		return nil, ErrNoFallbackNodeFound
	}
	req.URL.Host = fallback
	return f.callNode(req)
}

func (f *FallbackRoundTripper) callNode(req *http.Request) (*http.Response, error) {
	requestURI := req.RequestURI
	req.RequestURI = ""
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		resp.Request.RequestURI = requestURI
	}
	return resp, err
}
