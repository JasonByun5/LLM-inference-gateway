package radix

import "testing"

func TestEmpty(t *testing.T) {
	r := New()
	got := r.PrefixLengths([]uint64{1, 2, 3})
	if len(got) != 0 {
		t.Fatalf("empty tree: got %v", got)
	}
}

func TestExactMatch(t *testing.T) {
	r := New()
	r.insert([]uint64{1, 2, 3}, "worker-a")

	got := r.PrefixLengths([]uint64{1, 2, 3})
	if got["worker-a"] != 3 {
		t.Fatalf("got %v want worker-a=3", got)
	}
}

func TestLongerKey(t *testing.T) {
	r := New()
	r.insert([]uint64{1, 2, 3}, "worker-a")

	got := r.PrefixLengths([]uint64{1, 2, 3, 9})
	if got["worker-a"] != 3 {
		t.Fatalf("got %v want worker-a=3", got)
	}
}

func TestDivergingSuffix(t *testing.T) {
	r := New()
	r.insert([]uint64{1, 2, 3, 4}, "worker-a")

	got := r.PrefixLengths([]uint64{1, 2, 9})
	if got["worker-a"] != 2 {
		t.Fatalf("got %v want worker-a=2", got)
	}
}

func TestTwoWorkers(t *testing.T) {
	r := New()
	r.insert([]uint64{1, 2}, "worker-a")
	r.insert([]uint64{1, 2, 3}, "worker-b")

	got := r.PrefixLengths([]uint64{1, 2, 3, 4})
	if got["worker-a"] != 2 || got["worker-b"] != 3 {
		t.Fatalf("got %v want worker-a=2 worker-b=3", got)
	}
}

func TestNoMatch(t *testing.T) {
	r := New()
	r.insert([]uint64{1, 2, 3}, "worker-a")

	got := r.PrefixLengths([]uint64{9, 8, 7})
	if len(got) != 0 {
		t.Fatalf("got %v want empty", got)
	}
}
