package radix

type node struct {
	value    string
	children map[byte]*node
	end      bool
}

func newNode() *node {
	return &node{
		children: make(map[byte]*node),
	}
}
