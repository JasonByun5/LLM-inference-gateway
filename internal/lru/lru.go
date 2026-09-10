package lru

type lru struct {
	m    map[string]int
	list DoublyLinkedList
}

func New(cap int) *lru {
	return &lru{}
}

func Get(key string) (value, ok) {

}
