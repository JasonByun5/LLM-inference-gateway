package metrics

import (
	"testing"
	"time"
)

func TestPercentiles(t *testing.T) {
	r := New()
	for i := 1; i <= 100; i++ {
		r.Record(time.Duration(i) * time.Millisecond)
	}

	cases := []struct {
		p    float64
		want time.Duration
	}{
		{0.50, 50 * time.Millisecond},
		{0.95, 95 * time.Millisecond},
		{0.99, 99 * time.Millisecond},
	}
	for _, c := range cases {
		got := r.Percentile(c.p)
		if got != c.want {
			t.Fatalf("p%02.0f: got %v want %v", c.p*100, got, c.want)
		}
	}
}

func TestPercentileEmpty(t *testing.T) {
	r := New()
	if got := r.Percentile(0.99); got != 0 {
		t.Fatalf("empty: got %v want 0", got)
	}
}

func TestPercentileSortsCopy(t *testing.T) {
	r := New()
	r.Record(100 * time.Millisecond)
	r.Record(1 * time.Millisecond)
	r.Record(50 * time.Millisecond)

	if got := r.Percentile(0.50); got != 50*time.Millisecond {
		t.Fatalf("p50: got %v want 50ms", got)
	}

	want := []time.Duration{100 * time.Millisecond, 1 * time.Millisecond, 50 * time.Millisecond}
	if len(r.samples) != len(want) {
		t.Fatalf("len: got %d want %d", len(r.samples), len(want))
	}
	for i := range want {
		if r.samples[i] != want[i] {
			t.Fatalf("samples mutated: got %v want %v", r.samples, want)
		}
	}
}
