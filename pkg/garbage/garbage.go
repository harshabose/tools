package garbage

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"
)

type Garbage struct {
	len      uint
	interval time.Duration

	last time.Time
}

func NewGarbage(len uint, interval time.Duration) *Garbage {
	return &Garbage{
		len:      len,
		interval: interval,

		last: time.Now(),
	}
}

func (g *Garbage) Generate(ctx context.Context) ([]byte, error) {
	if g.len <= 0 {
		return nil, fmt.Errorf("garbage length must be greater than 0")
	}

	if elapsed := time.Since(g.last); elapsed < g.interval {
		interval := g.interval - elapsed

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}

	g.last = time.Now()

	buf := make([]byte, g.len)

	_, err := rand.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to generate garbage data: %w", err)
	}

	return buf, nil
}

func (g *Garbage) Consume(_ context.Context, _ []byte) error {
	return nil
}
