package balancer

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Policy int

const (
	RoundRobin Policy = iota
	LeastInflight
)

type Pool struct {
	mu       sync.Mutex
	next     int
	policy   Policy
	backends []Backend
}

func New(urls []string, policy Policy) *Pool {
	p := &Pool{policy: policy}
	for _, url := range urls {
		p.backends = append(p.backends, Backend{
			URL:      url,
			healthy:  true,
			inflight: 0,
		})
	}

	return p
}

func (p *Pool) CheckHealth() {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		p.mu.Lock()
		urls := make([]string, len(p.backends))
		for i := range p.backends {
			urls[i] = p.backends[i].URL
		}
		p.mu.Unlock()

		for i, url := range urls {
			resp, err := client.Get(url + "/health")
			ok := err == nil && resp.StatusCode == 200
			if resp != nil {
				resp.Body.Close()
			}

			p.mu.Lock()
			b := &p.backends[i]
			if ok {
				b.successes++
				b.fails = 0
				if b.successes >= 2 {
					b.healthy = true
				}
			} else {
				b.fails++
				b.successes = 0
				if b.fails >= 2 {
					b.healthy = false
				}
			}
			p.mu.Unlock()

		}
	}
}

func (p *Pool) Pick() (*Backend, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	n := len(p.backends)
	if n == 0 {
		return nil, fmt.Errorf("no backends")
	}

	var b *Backend

	switch p.policy {
	case LeastInflight:
		b = p.pickLeastInflight()
	default:
		b = p.pickRoundRobin()
	}

	if b == nil {
		return nil, fmt.Errorf("no healthy backends")
	}

	b.inflight++
	return b, nil
}

func (p *Pool) pickRoundRobin() *Backend {
	n := len(p.backends)

	for i := 0; i < n; i++ {
		b := &p.backends[p.next]
		p.next = (p.next + 1) % n
		if b.healthy {
			return b
		}
	}

	return nil
}

func (p *Pool) pickLeastInflight() *Backend {
	var best *Backend
	for i := range p.backends {
		b := &p.backends[i]
		if !b.healthy {
			continue
		}
		if best == nil || b.inflight < best.inflight {
			best = b
		}
	}
	return best
}

func (p *Pool) Release(backend *Backend) {
	p.mu.Lock()
	defer p.mu.Unlock()

	backend.inflight--
}
