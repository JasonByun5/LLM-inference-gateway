package queue

import (
	"context"
	"testing"
	"time"
)

func TestAcquireImmediate(t *testing.T) {
	q := New(0, 0, func() int { return 1 })
	if err := q.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestShedWhenBusyAndNoWaiters(t *testing.T) {
	q := New(0, 0, func() int { return 1 })
	if err := q.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := q.Acquire(context.Background()); err != ErrShed {
		t.Fatalf("got %v want ErrShed", err)
	}
}

func TestWaitsUntilRelease(t *testing.T) {
	q := New(4, 0, func() int { return 1 })
	if err := q.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- q.Acquire(context.Background()) }()

	select {
	case err := <-done:
		t.Fatalf("Acquire returned before Release: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	q.Release(0)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("waiter not unblocked")
	}
}

func TestShedWhenQueueFull(t *testing.T) {
	q := New(1, 0, func() int { return 1 })
	if err := q.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}

	waiting := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(waiting)
		done <- q.Acquire(context.Background())
	}()
	<-waiting
	time.Sleep(20 * time.Millisecond)

	if err := q.Acquire(context.Background()); err != ErrShed {
		t.Fatalf("got %v want ErrShed", err)
	}

	q.Release(0)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestShedWhenWaitExceedsDeadline(t *testing.T) {
	q := New(8, 500*time.Millisecond, func() int { return 1 })
	if err := q.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	q.Release(time.Second)

	if err := q.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := q.Acquire(context.Background()); err != ErrShed {
		t.Fatalf("got %v want ErrShed", err)
	}
}

func TestNoCapacity(t *testing.T) {
	q := New(4, 0, func() int { return 0 })
	if err := q.Acquire(context.Background()); err != ErrNoCapacity {
		t.Fatalf("got %v want ErrNoCapacity", err)
	}
}

func TestCancelWhileWaiting(t *testing.T) {
	q := New(4, 0, func() int { return 1 })
	if err := q.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- q.Acquire(ctx) }()
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("got %v want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled waiter still blocked")
	}

	q.Release(0)
	if err := q.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestFIFO(t *testing.T) {
	q := New(8, 0, func() int { return 1 })
	if err := q.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}

	order := make(chan int, 2)
	go func() {
		if err := q.Acquire(context.Background()); err != nil {
			t.Errorf("waiter 1: %v", err)
			return
		}
		order <- 1
		q.Release(0)
	}()
	time.Sleep(20 * time.Millisecond)
	go func() {
		if err := q.Acquire(context.Background()); err != nil {
			t.Errorf("waiter 2: %v", err)
			return
		}
		order <- 2
		q.Release(0)
	}()
	time.Sleep(20 * time.Millisecond)

	q.Release(0)
	got := []int{<-order, <-order}
	if got[0] != 1 || got[1] != 2 {
		t.Fatalf("order %v want 1, 2", got)
	}
}
