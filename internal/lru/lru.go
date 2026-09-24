package lru

type Lru struct {
	cap  int
	m    map[string]*Node
	list *DoublyLinkedList
}

func New(cap int) *Lru {
	return &Lru{
		cap:  cap,
		m:    make(map[string]*Node),
		list: newList(),
	}
}

func (l *Lru) Get(key string) (value int, ok bool) {
	n, ok := l.m[key]
	if !ok {
		return 0, false
	}
	l.list.moveToFront(n)
	return n.data, true

}

func (l *Lru) Put(key string, value int) {
	n, ok := l.m[key]
	if ok {
		n.data = value
		l.list.moveToFront(n)
		return
	}

	node := newNode(key, value)
	l.m[key] = node
	l.list.pushFront(node)

	if l.list.length > l.cap {
		victim := l.list.popBack()
		delete(l.m, victim.key)
	}

}
