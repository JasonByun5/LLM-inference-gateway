package main

import "testing"

func TestSplitBlocksEmpty(t *testing.T) {
	if got := splitBlocks("", 4); got != nil {
		t.Fatalf("empty prompt: got %v", got)
	}
	if got := splitBlocks("abcd", 0); got != nil {
		t.Fatalf("block size 0: got %v", got)
	}
}

func TestSplitBlocksCount(t *testing.T) {
	got := splitBlocks("abcdefghij", 4) // 4 + 4 + 2
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
}

func TestSplitBlocksSharedPrefix(t *testing.T) {
	a := splitBlocks("abcdefghij", 4)
	b := splitBlocks("abcdZZZZij", 4)
	if a[0] != b[0] {
		t.Fatal("shared first block should hash the same")
	}
	if a[1] == b[1] {
		t.Fatal("different second block should hash differently")
	}
}
