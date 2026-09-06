package buffer

import (
	"context"
	"fmt"
	"sync"
)

type (
	BucketIndexer[T any] func(T) int
	BucketFactory[T any] func() Buffer[T]
)

var _ Buffer[[]int] = &BucketBuffer[[]int, int]{}

type BucketBuffer[T ~[]E, E any] struct {
	buffs   []Buffer[E]
	indexer BucketIndexer[E]
	factory BucketFactory[E]
	mux     sync.RWMutex
}

func NewBucketBuffer[T any](indexer BucketIndexer[T], factory BucketFactory[T], buffs ...Buffer[T]) *BucketBuffer[[]T, T] {
	return &BucketBuffer[[]T, T]{
		buffs:   buffs,
		indexer: indexer,
		factory: factory,
	}
}

func (b *BucketBuffer[T, E]) Push(ctx context.Context, ts T) error {
	b.mux.Lock()
	defer b.mux.Unlock()

	for _, t := range ts {
		idx := b.indexer(t)
		if idx < 0 {
			return fmt.Errorf("negative index %d for element %v", idx, t)
		}

		// grow the buffer until idx (risky; change this) TODO;
		if idx >= len(b.buffs) {
			fmt.Printf("index %d out of range for %d buckets; creating...\n", idx, len(b.buffs))
			for len(b.buffs) <= idx {
				b.buffs = append(b.buffs, b.factory())
			}
		}

		if err := b.buffs[idx].Push(ctx, t); err != nil {
			return err
		}
	}

	return nil
}

func (b *BucketBuffer[T, E]) Pop(ctx context.Context) (T, error) {
	// returned in order as per the indexer

	b.mux.Lock()
	defer b.mux.Unlock()

	out := make(T, 0, len(b.buffs))
	for _, buf := range b.buffs {
		if buf.Size() == 0 {
			continue
		}
		e, err := buf.Pop(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil

}

func (b *BucketBuffer[T, E]) Size() int {
	b.mux.RLock()
	defer b.mux.RUnlock()

	if b.buffs == nil {
		return 0
	}

	var size = 0
	for _, buff := range b.buffs {
		size += buff.Size()
	}

	return size
}

func (b *BucketBuffer[T, E]) Close() {
	b.mux.Lock()
	defer b.mux.Unlock()

	for _, buff := range b.buffs {
		buff.Close()
	}

	b.buffs = nil
}
