package balancer

import (
	"testing"
	"time"
)

func TestPickRoundRobin(t *testing.T) {
	p := New([]string{
		"http://localhost:9001",
		"http://localhost:9002",
		"http://localhost:9003",
	}, RoundRobin, CacheConfig{})
	want := []string{
		"http://localhost:9001",
		"http://localhost:9002",
		"http://localhost:9003",
		"http://localhost:9001",
	}
	for i, u := range want {
		b, err := p.Pick(nil)
		if err != nil {
			t.Fatal(err)
		}
		if b.URL != u {
			t.Fatalf("pick %d: got %s want %s", i, b.URL, u)
		}
	}
}

func TestPickSkipsUnhealthy(t *testing.T) {
	p := New([]string{
		"http://localhost:9001",
		"http://localhost:9002",
		"http://localhost:9003",
	}, RoundRobin, CacheConfig{})
	p.backends[1].healthy = false // 9002 down

	want := []string{
		"http://localhost:9001",
		"http://localhost:9003",
		"http://localhost:9001",
	}
	for i, u := range want {
		b, err := p.Pick(nil)
		if err != nil {
			t.Fatal(err)
		}
		if b.URL != u {
			t.Fatalf("pick %d: got %s want %s", i, b.URL, u)
		}
	}
}

func TestCacheAwarePrefersOverlap(t *testing.T) {
	p := New([]string{
		"http://localhost:9001",
		"http://localhost:9002",
	}, CacheAware, CacheConfig{
		OverlapWeight: 10,
		LoadPenalty:   1,
		Capacity:      8,
	})

	first, err := p.Pick([]uint64{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	if first.URL != "http://localhost:9001" {
		t.Fatalf("first pick: got %s", first.URL)
	}

	second, err := p.Pick([]uint64{1, 2, 3, 4})
	if err != nil {
		t.Fatal(err)
	}
	if second.URL != "http://localhost:9001" {
		t.Fatalf("overlap pick: got %s want 9001", second.URL)
	}
}

func TestCacheAwareEvictsOldest(t *testing.T) {
	p := New([]string{"http://localhost:9001"}, CacheAware, CacheConfig{
		Capacity: 3,
	})
	worker := &p.backends[0]
	now := time.Now()
	p.remember([]uint64{1, 2, 3}, worker, now)
	p.remember([]uint64{4, 5, 6}, worker, now)

	if got := p.tree.PrefixLengths([]uint64{1, 2, 3}); len(got) != 0 {
		t.Fatalf("evicted prefix still present: %v", got)
	}
	if got := p.tree.PrefixLengths([]uint64{4, 5, 6}); got[worker.URL] != 3 {
		t.Fatalf("kept prefix: got %v", got)
	}
}

func TestCacheAwareKeepsSharedPrefix(t *testing.T) {
	p := New([]string{"http://localhost:9001"}, CacheAware, CacheConfig{
		Capacity: 4,
	})
	worker := &p.backends[0]
	now := time.Now()
	p.remember([]uint64{1, 2, 3}, worker, now)
	p.remember([]uint64{1, 2, 9}, worker, now)

	got := p.tree.PrefixLengths([]uint64{1, 2, 3})
	if got[worker.URL] != 2 {
		t.Fatalf("shared prefix: got %v want 2", got)
	}
}

func TestCacheAwareExpires(t *testing.T) {
	p := New([]string{"http://localhost:9001"}, CacheAware, CacheConfig{
		TTL:      time.Second,
		Capacity: 8,
	})
	worker := &p.backends[0]
	p.remember([]uint64{1, 2, 3}, worker, time.Now().Add(-2*time.Second))
	p.expire(time.Now())

	if got := p.tree.PrefixLengths([]uint64{1, 2, 3}); len(got) != 0 {
		t.Fatalf("expired prefix still present: %v", got)
	}
}
