// TODO: CLEAN THIS; THIS IS STUPID

package buffer

import "context"

type Pool[T any] interface {
	Get() T
	Put(T)
	Release()
}

type Buffer[T any] interface {
	Push(context.Context, T) error
	Pop(context.Context) (T, error)
	Size() int
	Close()
}

type PeekBuffer[T any] interface {
	Buffer[T]
	TryPush(T) bool
	TryPop() (T, bool)
}

type BufferWithGenerator[T any] interface {
	Buffer[T]
	Get() T
	Put(T)
}
