package buffer

import "context"

type ChannelBufferWithGenerator[T any] struct {
	pool Pool[T]

	*ChannelBuffer[T]
}

func NewChannelBufferWithGenerator[T any](ctx context.Context, pool Pool[T], size uint, nworkers int) *ChannelBufferWithGenerator[T] {
	b := &ChannelBufferWithGenerator[T]{
		pool:          pool,
		ChannelBuffer: newChannelBuffer[T](ctx, size, 0),
	}

	if nworkers <= 0 {
		nworkers = 1
	}

	for i := 0; i < nworkers; i++ {
		go b.loop()
	}

	return b
}

func (b *ChannelBufferWithGenerator[T]) Get() T {
	return b.pool.Get()
}

func (b *ChannelBufferWithGenerator[T]) Put(element T) {
	b.pool.Put(element)
}

func (b *ChannelBufferWithGenerator[T]) loop() {
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
					case old := <-b.output:
						b.pool.Put(old)
						// NOTE: REMOVED OLDEST
					}
				}
			}
		}
	}
}
