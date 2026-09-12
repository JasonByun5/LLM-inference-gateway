package lru

type Node struct {
	key  string
	data int
	next *Node
	prev *Node
}

func newNode(key string, data int) *Node {
	return &Node{key: key, data: data}
}
