package set

var exists = struct{}{}

type Set[T comparable] struct {
	items map[T]struct{}
}

func NewSet[T comparable](items ...T) *Set[T] {
	s := &Set[T]{
		items: make(map[T]struct{}),
	}

	if len(items) > 0 {
		s.Add(items...)
	}
	return s
}

func (s *Set[T]) Add(items ...T) {
	if len(items) == 0 {
		return
	}

	for _, item := range items {
		s.items[item] = exists
	}
}

func (s *Set[T]) Remove(items ...T) {
	for _, item := range items {
		delete(s.items, item)
	}
}

func (s *Set[T]) RemoveIf(f func(T) bool) {
	for item := range s.items {
		if f(item) {
			delete(s.items, item)
		}
	}
}

func (s *Set[T]) Empty() bool {
	return s.Size() == 0
}

func (s *Set[T]) Size() int {
	return len(s.items)
}

func (s *Set[T]) Clear() {
	s.items = make(map[T]struct{})
}

func (s *Set[T]) Items() []T {
	items := make([]T, s.Size())
	i := 0
	for item := range s.items {
		items[i] = item
		i++
	}

	return items
}
