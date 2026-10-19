package radix

type Tree struct {
	root *node
}

func New() *Tree {
	return &Tree{
		root: newNode(),
	}
}

func (r *Tree) insert(blocks []uint64, worker string) {
	cur := r.root

	for _, block := range blocks {

		if cur.children[block] == nil {
			cur.children[block] = newNode()
		}

		cur = cur.children[block]
		cur.workers[worker] = struct{}{}

	}
}

func (r *Tree) PrefixLengths(blocks []uint64) map[string]int {
	cur := r.root
	depth := make(map[string]int)

	for i, block := range blocks {
		cur = cur.children[block]
		if cur == nil {
			break
		}
		for worker := range cur.workers {
			depth[worker] = i + 1
		}
	}
	return depth
}
