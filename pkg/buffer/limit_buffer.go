package buffer

import (
	"context"
	"sync"
)

type ChannelBuffer[T any] struct {
	input  chan T
	output chan T

	once   sync.Once
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

func NewChannelBuffer[T any](ctx context.Context, size uint, nworkers int) *ChannelBuffer[T] {
	if nworkers <= 0 {
		nworkers = 1
	}

	return newChannelBuffer[T](ctx, size, nworkers)
}

func newChannelBuffer[T any](ctx context.Context, size uint, nworkers int) *ChannelBuffer[T] {
	ctx2, cancel2 := context.WithCancel(ctx)

	b := &ChannelBuffer[T]{
		input:  make(chan T, size),
		output: make(chan T, size),
		ctx:    ctx2,
		cancel: cancel2,
	}

	for i := 0; i < nworkers; i++ {
		go b.loop()
	}

	return b
}

func (b *ChannelBuffer[T]) Push(ctx context.Context, element T) error {
	select {
	case <-b.ctx.Done():
		return ErrorChannelBufferClose
	case b.input <- element:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *ChannelBuffer[T]) Pop(ctx context.Context) (T, error) {
	var zero T

	select {
	case <-b.ctx.Done():
		return zero, ErrorChannelBufferClose
	case <-ctx.Done():
		return zero, ctx.Err()
	case data, ok := <-b.output:
		if !ok {
			return zero, ErrorChannelBufferClose
		}
		return data, nil
	}
}

func (b *ChannelBuffer[T]) Size() int {
	return len(b.output)
}

func (b *ChannelBuffer[T]) loop() {
	defer b.close()

	b.wg.Add(1)
	defer b.wg.Done()

	for {
		select {
		case <-b.ctx.Done():
			return
		case element, ok := <-b.input:
			if !ok {
				continue
			}

		loop2:
			for {
				select {
				case b.output <- element:
					break loop2 // NOTE: SUCCESSFULLY BUFFERED
				default:
					select {
					case _ = <-b.output:
						// NOTE: REMOVED OLDEST
					}
				}
			}
		}
	}
}

func (b *ChannelBuffer[T]) close() {
	b.once.Do(func() {
		if b.cancel != nil {
			b.cancel()
		}

		b.wg.Wait()

		// close(b.input)
		close(b.output)
	})
}

// Close stops the buffering and put the buffer in an unusable state.
// Cancelling the original ctx will also have the same effect.
// This method should be treated as a manual overide.
func (b *ChannelBuffer[T]) Close() {
	b.close()
}

func (b *ChannelBuffer[T]) TryPush(element T) bool {
	select {
	case <-b.ctx.Done():
		return false
	case b.input <- element:
		return true
	default:
		return false
	}
}

func (b *ChannelBuffer[T]) TryPop() (T, bool) {
	var zero T

	select {
	case <-b.ctx.Done():
		return zero, false
	case data, ok := <-b.output:
		if !ok {
			return zero, false
		}
		return data, true
	default:
		return zero, false
	}
}
