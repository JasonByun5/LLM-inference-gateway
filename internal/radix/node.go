package radix

type node struct {
	workers  map[string]int // worker URL -> how many live sequences cover this prefix
	children map[uint64]*node
}

func newNode() *node {
	return &node{
		children: make(map[uint64]*node),
		workers:  make(map[string]int),
	}
}
