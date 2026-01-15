package buffer

import (
	"context"
	"time"
)

type timed[T any] struct {
	element T
	created time.Time
}

type TimedBuffer[T any] struct {
	b   *ChannelBuffer[*timed[T]]
	ttl time.Duration
}

func NewTimedBuffer[T any](ctx context.Context, size uint, nworkers int, ttl time.Duration) *TimedBuffer[T] {
	return &TimedBuffer[T]{
		b:   NewChannelBuffer[*timed[T]](ctx, size, nworkers),
		ttl: ttl,
	}
}

func (b *TimedBuffer[T]) Push(ctx context.Context, t T) error {
	return b.b.Push(ctx, &timed[T]{element: t, created: time.Now()})
}

func (b *TimedBuffer[T]) Pop(ctx context.Context) (T, error) {
	var zero T
	for {
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
			t, err := b.b.Pop(ctx)
			if err != nil {
				return zero, err
			}

			if time.Since(t.created) <= b.ttl {
				return t.element, nil
			}
		}
	}
}

func (b *TimedBuffer[T]) Size() int {
	return b.b.Size()
}

func (b *TimedBuffer[T]) Close() {
	b.b.Close()
}
