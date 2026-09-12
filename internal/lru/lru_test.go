package lru

import "testing"

func TestGetMiss(t *testing.T) {
	c := New(2)
	if _, ok := c.Get("a"); ok {
		t.Fatal("empty cache: expected miss")
	}
}

func TestPutGet(t *testing.T) {
	c := New(2)
	c.Put("a", 1)

	got, ok := c.Get("a")
	if !ok {
		t.Fatal("expected hit")
	}
	if got != 1 {
		t.Fatalf("got %d want 1", got)
	}
}

func TestPutUpdatesValue(t *testing.T) {
	c := New(2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("a", 9)

	got, ok := c.Get("a")
	if !ok || got != 9 {
		t.Fatalf("got %d ok=%v want 9 true", got, ok)
	}
	if _, ok := c.Get("b"); !ok {
		t.Fatal("update should not evict b")
	}
}

func TestEvictsLeastRecentPut(t *testing.T) {
	c := New(2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)

	if _, ok := c.Get("a"); ok {
		t.Fatal("a should be evicted")
	}
	if got, ok := c.Get("b"); !ok || got != 2 {
		t.Fatalf("b: got %d ok=%v", got, ok)
	}
	if got, ok := c.Get("c"); !ok || got != 3 {
		t.Fatalf("c: got %d ok=%v", got, ok)
	}
}

func TestGetRefreshesRecency(t *testing.T) {
	c := New(2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Get("a") // a is now most recent; b is least
	c.Put("c", 3)

	if _, ok := c.Get("b"); ok {
		t.Fatal("b should be evicted")
	}
	if _, ok := c.Get("a"); !ok {
		t.Fatal("a should remain after Get")
	}
	if _, ok := c.Get("c"); !ok {
		t.Fatal("c should be present")
	}
}

func TestCapacityOne(t *testing.T) {
	c := New(1)
	c.Put("a", 1)
	c.Put("b", 2)

	if _, ok := c.Get("a"); ok {
		t.Fatal("a should be evicted")
	}
	if got, ok := c.Get("b"); !ok || got != 2 {
		t.Fatalf("b: got %d ok=%v", got, ok)
	}
}
