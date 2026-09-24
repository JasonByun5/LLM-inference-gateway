package balancer

import (
	"fmt"
	"llm-inference-gateway/internal/lru"
	"llm-inference-gateway/internal/radix"
	"net/http"
	"sync"
	"time"
)

type Policy int

const (
	RoundRobin Policy = iota
	LeastInflight
	CacheAware
)

type CacheConfig struct {
	OverlapWeight float64
	LoadPenalty   float64
	Capacity      int           // predicted blocks kept per worker
	TTL           time.Duration // how long a predicted entry stays valid
}

type Pool struct {
	mu       sync.Mutex
	next     int
	policy   Policy
	cache    CacheConfig
	backends []Backend
	tree     *radix.Tree
	lru      *lru.Lru
}

func New(urls []string, policy Policy, cache CacheConfig) *Pool {
	p := &Pool{
		policy: policy,
		cache:  cache,
		tree:   radix.New(),
		lru:    lru.New(),
	}
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

func (p *Pool) Healthy() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for i := range p.backends {
		if p.backends[i].healthy {
			n++
		}
	}
	return n
}

// Pick chooses a backend. blocks is this prompt's hashes in order, from
// splitBlocks. Round-robin and least-inflight ignore it. Cache-aware uses
// it as the lookup key for the predicted cache.
func (p *Pool) Pick(blocks []uint64) (*Backend, error) {
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
	case CacheAware:
		b = p.pickCacheAware(blocks)
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

// pickCacheAware chooses the healthy backend with the highest score:
// cache.OverlapWeight * matchedBlocks - cache.LoadPenalty * inflight.
// matchedBlocks is the leading run of blocks the predicted cache still
// has for that backend. Record the chosen backend's blocks on the way out.
func (p *Pool) pickCacheAware(blocks []uint64) *Backend {
	return nil
}

func (p *Pool) Release(backend *Backend) {
	p.mu.Lock()
	defer p.mu.Unlock()

	backend.inflight--
}
