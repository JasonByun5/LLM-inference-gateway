package lru

type Node struct {
	data int
	next *Node
	prev *Node
}

func newNode(data int) *Node {
	return &Node{data: data}
}
