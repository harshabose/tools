package buffer

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type ChannelBuffer[T any] struct {
	pool          Pool[T]
	bufferChannel chan T
	inputBuffer   chan T
	closed        bool
	mux           sync.RWMutex
	ctx           context.Context
}

func CreateChannelBuffer[T any](ctx context.Context, size int, pool Pool[T]) *ChannelBuffer[T] {
	buffer := &ChannelBuffer[T]{
		pool:          pool,
		bufferChannel: make(chan T, size),
		inputBuffer:   make(chan T, size),
		closed:        false,
		ctx:           ctx,
	}
	go buffer.loop()
	return buffer
}

func (buffer *ChannelBuffer[T]) Push(ctx context.Context, element T) error {
	buffer.mux.RLock()
	defer buffer.mux.RUnlock()

	if buffer.closed {
		return errors.New("buffer closed")
	}
	select {
	case buffer.inputBuffer <- element:
		// WARN: LACKS CHECKS FOR CLOSED CHANNEL
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (buffer *ChannelBuffer[T]) Pop(ctx context.Context) (T, error) {
	buffer.mux.RLock()
	defer buffer.mux.RUnlock()

	var zero T

	if buffer.closed {
		return zero, errors.New("buffer closed")
	}
	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	case data, ok := <-buffer.bufferChannel:
		if !ok {
			return zero, ErrorChannelBufferClose
		}
		// TODO: NIL CHECK IS REQUIRED BUT GENERIC CANNOT DO THIS. SOLVE THIS ASAP
		// if data == nil {
		// 	return zero, ErrorElementUnallocated
		// }
		return data, nil
	}
}

func (buffer *ChannelBuffer[T]) Generate() T {
	return buffer.pool.Get()
}

func (buffer *ChannelBuffer[T]) PutBack(element T) {
	if buffer.pool != nil {
		buffer.pool.Put(element)
	}
}

func (buffer *ChannelBuffer[T]) GetChannel() chan T {
	return buffer.bufferChannel
}

func (buffer *ChannelBuffer[T]) Size() int {
	return len(buffer.bufferChannel)
}

func (buffer *ChannelBuffer[T]) loop() {
	defer buffer.close()
loop:
	for {
		select {
		case <-buffer.ctx.Done():
			return
		case element, ok := <-buffer.inputBuffer:
			// if !ok || element == nil {
			// 	continue loop
			// }
			if !ok {
				continue loop
			}
			select {
			case buffer.bufferChannel <- element: // SUCCESSFULLY BUFFERED
				continue loop
			default:
				select {
				case oldElement := <-buffer.bufferChannel:
					buffer.PutBack(oldElement)
					select {
					case buffer.bufferChannel <- element:
						continue loop
					default:
						fmt.Println("unexpected buffer state. skipping the element..")
						buffer.PutBack(element)
					}
				}
			}
		}
	}
}

func (buffer *ChannelBuffer[T]) close() {
	buffer.mux.Lock()
	buffer.closed = true
	buffer.mux.Unlock()

loop:
	for {
		select {
		case element := <-buffer.bufferChannel:
			if buffer.pool != nil {
				buffer.pool.Put(element)
			}
		default:
			close(buffer.bufferChannel)
			close(buffer.inputBuffer)
			break loop
		}
	}
	if buffer.pool != nil {
		buffer.pool.Release()
	}
}
