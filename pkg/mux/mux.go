package mux

import "context"

type noCopy struct{}

func (n *noCopy) Lock()   {}
func (n *noCopy) Unlock() {}

type Mux struct {
	_         noCopy
	semaphore chan struct{}
}

func NewMux() *Mux {
	m := &Mux{
		semaphore: make(chan struct{}, 1),
	}
	m.semaphore <- struct{}{}

	return m
}

func (m *Mux) Lock(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-m.semaphore:
		return nil
	}
}

func (m *Mux) LockCh() <-chan struct{} {
	return m.semaphore
}

func (m *Mux) Unlock() {
	m.semaphore <- struct{}{}
}
