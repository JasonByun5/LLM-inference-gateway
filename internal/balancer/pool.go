package balancer

import (
	"fmt"
	"sync"
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
