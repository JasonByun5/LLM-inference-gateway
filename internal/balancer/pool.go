package balancer

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Pool struct {
	mu       sync.Mutex
	next     int
	backends []Backend
}

func New(urls []string) *Pool {
	p := &Pool{}
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

	for i := 0; i < n; i++ {
		b := &p.backends[p.next]
		p.next = (p.next + 1) % n
		if b.healthy {
			b.inflight++
			return b, nil
		}
	}

	return nil, fmt.Errorf("no healthy backends")

}

func (p *Pool) Release(backend *Backend) {
	p.mu.Lock()
	defer p.mu.Unlock()

	backend.inflight--
}
