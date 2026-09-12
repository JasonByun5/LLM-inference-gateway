package lru

type DoublyLinkedList struct {
	head   *Node
	tail   *Node
	length int
}

func newList() *DoublyLinkedList {
	return &DoublyLinkedList{}
}

func (d *DoublyLinkedList) pushFront(n *Node) {

	if d.head == nil {
		d.head = n
		d.tail = n
		d.length++
		return
	}

	n.next = d.head
	d.head.prev = n
	d.head = n
	d.length++
}

func (d *DoublyLinkedList) remove(n *Node) {
	if n.prev != nil {
		n.prev.next = n.next
	} else {
		d.head = n.next
	}

	if n.next != nil {
		n.next.prev = n.prev
	} else {
		d.tail = n.prev
	}

	n.next = nil
	n.prev = nil
	d.length--
}

func (d *DoublyLinkedList) moveToFront(n *Node) {
	// want to connect the ones it is in between, then make the head pointer into this one and have this one

	if n == d.head {
		return
	}
	d.remove(n)
	d.pushFront(n)

}

func (d *DoublyLinkedList) PopBack() *Node {
	if d.tail == nil {
		return nil
	}
	n := d.tail
	d.remove(n)
	return n
}
