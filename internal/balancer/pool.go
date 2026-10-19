package balancer

import (
	"fmt"
	"llm-inference-gateway/internal/radix"
	"math"
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

type cachedPrefix struct {
	blocks []uint64
	at     time.Time
}

type Pool struct {
	mu       sync.Mutex
	next     int
	policy   Policy
	cache    CacheConfig
	backends []Backend
	tree     *radix.Tree
	seen     map[string][]cachedPrefix // worker URL -> sequences, oldest first
}

func New(urls []string, policy Policy, cache CacheConfig) *Pool {
	p := &Pool{
		policy: policy,
		cache:  cache,
		tree:   radix.New(),
		seen:   make(map[string][]cachedPrefix),
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

func (p *Pool) pickCacheAware(blocks []uint64) *Backend {
	now := time.Now()
	p.expire(now)
	matched := p.tree.PrefixLengths(blocks)

	var best *Backend
	bestScore := math.Inf(-1)

	for i := range p.backends {
		b := &p.backends[i]

		if !b.healthy {
			continue
		}

		overlap := matched[b.URL]
		score := p.cache.OverlapWeight*float64(overlap) - p.cache.LoadPenalty*float64(b.inflight)

		if best == nil || score > bestScore {
			best = b
			bestScore = score
		}
	}

	if best == nil {
		return nil
	}

	p.remember(blocks, best, now)
	return best
}

func (p *Pool) remember(blocks []uint64, backend *Backend, now time.Time) {
	if len(blocks) == 0 || backend == nil {
		return
	}

	copied := make([]uint64, len(blocks))
	copy(copied, blocks)
	p.tree.Insert(copied, backend.URL)
	p.seen[backend.URL] = append(p.seen[backend.URL], cachedPrefix{
		blocks: copied,
		at:     now,
	})
	p.evict(backend.URL)
}

func (p *Pool) expire(now time.Time) {
	if p.cache.TTL <= 0 {
		return
	}
	for worker, seqs := range p.seen {
		kept := seqs[:0]
		for _, s := range seqs {
			if now.Sub(s.at) > p.cache.TTL {
				p.tree.Remove(s.blocks, worker)
				continue
			}
			kept = append(kept, s)
		}
		if len(kept) == 0 {
			delete(p.seen, worker)
			continue
		}
		p.seen[worker] = kept
	}
}

func (p *Pool) evict(worker string) {
	if p.cache.Capacity <= 0 {
		return
	}
	for p.blockCount(worker) > p.cache.Capacity {
		seqs := p.seen[worker]
		if len(seqs) == 0 {
			return
		}
		p.tree.Remove(seqs[0].blocks, worker)
		seqs = seqs[1:]
		if len(seqs) == 0 {
			delete(p.seen, worker)
			return
		}
		p.seen[worker] = seqs
	}
}

func (p *Pool) blockCount(worker string) int {
	n := 0
	for _, s := range p.seen[worker] {
		n += len(s.blocks)
	}
	return n
}

func (p *Pool) Release(backend *Backend) {
	p.mu.Lock()
	defer p.mu.Unlock()

	backend.inflight--
}
