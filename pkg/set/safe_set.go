package set

import (
	"sync"
)

type SafeSet[T comparable] struct {
	*Set[T]
	mux sync.RWMutex
}

func NewSafeSet[T comparable](items ...T) *SafeSet[T] {
	return &SafeSet[T]{
		Set: NewSet(items...),
	}
}

func (s *SafeSet[T]) Add(items ...T) {
	s.mux.Lock()
	defer s.mux.Unlock()

	s.Set.Add(items...)
}

func (s *SafeSet[T]) Remove(items ...T) {
	s.mux.Lock()
	defer s.mux.Unlock()

	s.Set.Remove(items...)
}

func (s *SafeSet[T]) RemoveIf(f func(T) bool) {
	s.mux.Lock()
	defer s.mux.Unlock()

	s.Set.RemoveIf(f)
}

func (s *SafeSet[T]) Empty() bool {
	return s.Size() == 0
}

func (s *SafeSet[T]) Size() int {
	s.mux.RLock()
	defer s.mux.RUnlock()

	return s.Set.Size()
}

func (s *SafeSet[T]) Exists(item T) bool {
	s.mux.RLock()
	defer s.mux.RUnlock()

	return s.Set.Exists(item)
}

func (s *SafeSet[T]) ExistsIf(f func(T) bool) bool {
	s.mux.RLock()
	defer s.mux.RUnlock()

	return s.Set.ExistsIf(f)
}

func (s *SafeSet[T]) Clear() {
	s.mux.Lock()
	defer s.mux.Unlock()

	s.Set.Clear()
}

func (s *SafeSet[T]) Items() []T {
	s.mux.RLock()
	defer s.mux.RUnlock()

	return s.Set.Items()
}
