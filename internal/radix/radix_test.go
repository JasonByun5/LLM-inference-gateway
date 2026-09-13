package radix

import "testing"

func TestEmpty(t *testing.T) {
	r := new()
	if _, ok := r.longestPrefixMatch([]byte("hello")); ok {
		t.Fatal("empty tree: expected miss")
	}
}

func TestExactMatch(t *testing.T) {
	r := new()
	r.insert("hello", "hello")

	got, ok := r.longestPrefixMatch([]byte("hello"))
	if !ok {
		t.Fatal("expected exact hit")
	}
	if got != "hello" {
		t.Fatalf("got %q want %q", got, "hello")
	}
}

func TestStoredPrefix(t *testing.T) {
	r := new()
	r.insert("hello", "hello")

	got, ok := r.longestPrefixMatch([]byte("hello world"))
	if !ok || got != "hello" {
		t.Fatalf("got %q ok=%v want hello true", got, ok)
	}
}

func TestSharedBranchNoStoredPrefix(t *testing.T) {
	r := new()
	r.insert("hello", "hello")

	if _, ok := r.longestPrefixMatch([]byte("help")); ok {
		t.Fatal("shared branch hel is not a stored prefix")
	}
}

func TestLongestOfTwoPrefixes(t *testing.T) {
	r := new()
	r.insert("hel", "hel")
	r.insert("hello", "hello")

	got, ok := r.longestPrefixMatch([]byte("help"))
	if !ok || got != "hel" {
		t.Fatalf("help: got %q ok=%v want hel true", got, ok)
	}

	got, ok = r.longestPrefixMatch([]byte("hello!"))
	if !ok || got != "hello" {
		t.Fatalf("hello!: got %q ok=%v want hello true", got, ok)
	}
}

func TestNoMatch(t *testing.T) {
	r := new()
	r.insert("hello", "hello")

	if _, ok := r.longestPrefixMatch([]byte("xyz")); ok {
		t.Fatal("xyz should miss")
	}
}
