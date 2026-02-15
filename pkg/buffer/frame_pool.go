//go:build cgo_enabled

package buffer

import (
	"sync"

	"github.com/asticode/go-astiav"
)

type framePool struct {
	pool sync.Pool
}

func CreateFramePool() Pool[*astiav.Frame] {
	return &framePool{
		pool: sync.Pool{},
	}
}

func (pool *framePool) Get() *astiav.Frame {
	v := pool.pool.Get()
	if v == nil {
		return astiav.AllocFrame()
	}

	frame, ok := v.(*astiav.Frame)
	if !ok {
		return astiav.AllocFrame()
	}
	return frame
}

func (pool *framePool) Put(frame *astiav.Frame) {
	if frame == nil {
		return
	}

	frame.Unref()
	pool.pool.Put(frame)
}

func (pool *framePool) Release() {
	for {
		v := pool.pool.Get()
		if v == nil {
			break
		}

		frame, ok := v.(*astiav.Frame)
		if !ok {
			continue
		}

		frame.Free()
	}
}
