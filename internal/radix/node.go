package radix

type node struct {
	workers  map[string]struct{}
	children map[uint64]*node
}

func newNode() *node {
	return &node{
		children: make(map[uint64]*node),
		workers:  make(map[string]struct{}),
	}
}
