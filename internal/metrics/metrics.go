package metrics

import (
	"slices"
	"sync"
	"time"
)

// Recorder stores raw latency samples and computes percentiles from them.
type Recorder struct {
	mu      sync.Mutex
	samples []time.Duration
}

func New() *Recorder {
	return &Recorder{}
}

func (r *Recorder) Record(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.samples = append(r.samples, d)
}

// Percentile returns the sample at index int(p*(n-1)) of a sorted copy.
// p is clamped to [0, 1]. An empty recorder returns 0.
func (r *Recorder) Percentile(p float64) time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()

	n := len(r.samples)
	if n == 0 {
		return 0
	}
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}

	sorted := slices.Clone(r.samples)
	slices.Sort(sorted)
	return sorted[int(p*float64(n-1))]
}
