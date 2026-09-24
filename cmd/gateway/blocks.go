package main

import (
	"hash/fnv"
)

// splits the prompt into blocks and hashes them
func splitBlocks(prompt string, blockSize int) []uint64 {
	if blockSize < 1 || prompt == "" {
		return nil
	}
	b := []byte(prompt)
	out := make([]uint64, 0, (len(b)+blockSize-1)/blockSize)
	h := fnv.New64a()
	for start := 0; start < len(b); start += blockSize {
		end := start + blockSize
		if end > len(b) {
			end = len(b)
		}
		h.Reset()
		_, _ = h.Write(b[start:end])
		out = append(out, h.Sum64())
	}
	return out
}
