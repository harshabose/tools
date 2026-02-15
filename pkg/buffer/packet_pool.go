//go:build cgo_enabled

package buffer

import (
	"sync"

	"github.com/asticode/go-astiav"
)

type packetPool struct {
	pool sync.Pool
}

func CreatePacketPool() Pool[*astiav.Packet] {
	return &packetPool{
		pool: sync.Pool{},
	}
}

func (pool *packetPool) Get() *astiav.Packet {
	v := pool.pool.Get()
	if v == nil {
		return astiav.AllocPacket()
	}

	packet, ok := v.(*astiav.Packet)
	if !ok {
		return astiav.AllocPacket()
	}
	return packet
}

func (pool *packetPool) Put(packet *astiav.Packet) {
	if packet == nil {
		return
	}

	packet.Unref()
	pool.pool.Put(packet)
}

func (pool *packetPool) Release() {
	for {
		v := pool.pool.Get()
		if v == nil {
			break
		}

		packet, ok := v.(*astiav.Packet)
		if !ok {
			continue
		}

		packet.Free()
	}
}
