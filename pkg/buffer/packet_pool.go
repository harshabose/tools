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
		pool: sync.Pool{
			New: func() any {
				return astiav.AllocPacket()
			},
		},
	}
}

func (pool *packetPool) Get() *astiav.Packet {
	packet, ok := pool.pool.Get().(*astiav.Packet)

	if packet == nil || !ok {
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
		packet, ok := pool.pool.Get().(*astiav.Packet)
		if packet == nil {
			break
		}
		if !ok {
			continue
		}
		// fmt.Printf("🗑️ Releasing packet: ptr=%p\n", packet)
		packet.Free()
	}
}
