package radix

type Tree struct {
	root *node
}

func New() *Tree {
	return &Tree{
		root: newNode(),
	}
}

func (r *Tree) Insert(blocks []uint64, worker string) {
	cur := r.root

	for _, block := range blocks {

		if cur.children[block] == nil {
			cur.children[block] = newNode()
		}

		cur = cur.children[block]
		cur.workers[worker]++

	}
}

func (r *Tree) Remove(blocks []uint64, worker string) {
	cur := r.root

	for _, block := range blocks {
		next := cur.children[block]
		if next == nil {
			return
		}
		cur = next
		cur.workers[worker]--
		if cur.workers[worker] <= 0 {
			delete(cur.workers, worker)
		}
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
