package cond

import (
	"context"
	"sync"

	"github.com/harshabose/tools/pkg/set"
)

type waiter struct {
	ch   chan struct{}
	once sync.Once
}

func (w *waiter) close() {
	w.once.Do(func() {
		close(w.ch)
	})
}

type waiters struct {
	waiters *set.SafeSet[*waiter]
}

func (ws *waiters) new() *waiter {
	w := &waiter{ch: make(chan struct{})}
	ws.waiters.Add(w)

	return w
}

func (ws *waiters) remove(w *waiter) {
	ws.waiters.Remove(w)
}

func (ws *waiters) items() []*waiter {
	return ws.waiters.Items()
}

func (ws *waiters) len() int {
	return ws.waiters.Size()
}

type ContextCond struct {
	*waiters
	L sync.Locker
}

func NewContextCond(l sync.Locker) *ContextCond {
	return &ContextCond{
		L:       l,
		waiters: &waiters{waiters: set.NewSafeSet[*waiter]()},
	}
}

func (c *ContextCond) Wait(ctx context.Context) error {
	c.L.Unlock()
	defer c.L.Lock()

	w := c.new()
	defer w.close()
	defer c.remove(w) // remove from the set (no channel closing)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-w.ch:
		return nil
	}
}

func (c *ContextCond) Signal() {
	ws := c.items()

	for _, w := range ws {
		w.close()
		c.remove(w)

		return
	}
}

func (c *ContextCond) Broadcast() {
	ws := c.items()

	for _, w := range ws {
		w.close() // only closes once
		c.remove(w)
	}
}

func (c *ContextCond) Len() int {
	return c.waiters.len()
}
