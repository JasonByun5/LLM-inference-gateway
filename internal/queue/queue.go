package queue

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrShed       = errors.New("shed")
	ErrNoCapacity = errors.New("no capacity")
)

type Queue struct {
	mu         sync.Mutex
	serving    int
	waiters    []chan struct{}
	avgService time.Duration
	maxWaiters int
	deadline   time.Duration
	capacity   func() int
}

func New(maxWaiters int, deadline time.Duration, capacity func() int) *Queue {
	return &Queue{
		maxWaiters: maxWaiters,
		deadline:   deadline,
		capacity:   capacity,
	}
}

func (q *Queue) Acquire(ctx context.Context) error {
	q.mu.Lock()
	cap := q.capacity()
	if cap < 1 {
		q.mu.Unlock()
		return ErrNoCapacity
	}

	//no waiters
	if q.serving < cap && len(q.waiters) == 0 {
		q.serving++
		q.mu.Unlock()
		return nil
	}

	// too many waiters
	if q.maxWaiters >= 0 && len(q.waiters) >= q.maxWaiters {
		q.mu.Unlock()
		return ErrShed
	}

	// now checks if you should keep or not
	ahead := len(q.waiters)
	if q.serving >= cap {
		ahead++
	}

	// estimates waiting time and checks if it is ok
	if q.deadline > 0 && q.avgService > 0 {
		est := time.Duration(ahead) * q.avgService / time.Duration(cap)
		if est > q.deadline {
			q.mu.Unlock()
			return ErrShed
		}
	}

	//else gives worker a place in line
	ch := make(chan struct{})
	q.waiters = append(q.waiters, ch)
	q.mu.Unlock()

	select {
	case <-ctx.Done():
		q.drop(ch)
		return ctx.Err()
	case <-ch:
		return nil
	}
}

func (q *Queue) Release(d time.Duration) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if d > 0 {
		if q.avgService == 0 {
			q.avgService = d
		} else {
			q.avgService = q.avgService*4/5 + d/5
		}
	}
	q.handoffLocked()
}

func (q *Queue) drop(ch chan struct{}) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, w := range q.waiters {
		if w == ch {
			q.waiters = append(q.waiters[:i], q.waiters[i+1:]...)
			return
		}
	}
	q.handoffLocked()
}

func (q *Queue) handoffLocked() {
	if len(q.waiters) > 0 {
		ch := q.waiters[0]
		q.waiters = q.waiters[1:]
		close(ch)
		return
	}
	q.serving--
}
