package radix

type Tree struct {
	root *node
}

func New() *Tree {
	return &Tree{
		root: newNode(),
	}
}

func (r *Tree) insert(s string, value string) {
	cur := r.root

	for i := 0; i < len(s); i++ {
		letter := s[i]

		if cur.children[letter] == nil {
			cur.children[letter] = newNode()
		}
		cur = cur.children[letter]

	}

	cur.end = true
	cur.value = value
}

func (r *Tree) longestPrefixMatch(key []byte) (string, bool) {
	cur := r.root
	var best string
	found := false

	for i := 0; i < len(key); i++ {
		next := cur.children[key[i]]
		if next == nil {
			break
		}
		cur = next
		if cur.end {
			best = cur.value
			found = true
		}
	}
	return best, found
}
