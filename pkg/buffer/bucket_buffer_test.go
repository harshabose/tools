package buffer

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// mockBuffer: a simple, synchronous, deterministic Buffer[T] used to test
// BucketBuffer's own routing/growth/locking logic in isolation from
// TimedBuffer/ChannelBuffer's async worker behavior.
// ---------------------------------------------------------------------------

type mockBuffer[T any] struct {
	mu      sync.Mutex
	items   []T
	closed  bool
	pushErr error // if set, Push always returns this error
}

func newMockBuffer[T any]() *mockBuffer[T] {
	return &mockBuffer[T]{}
}

func (m *mockBuffer[T]) Push(_ context.Context, t T) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pushErr != nil {
		return m.pushErr
	}
	m.items = append(m.items, t)
	return nil
}

func (m *mockBuffer[T]) Pop(_ context.Context) (T, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var zero T
	if len(m.items) == 0 {
		return zero, errors.New("empty")
	}
	t := m.items[0]
	m.items = m.items[1:]
	return t, nil
}

func (m *mockBuffer[T]) Size() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.items)
}

func (m *mockBuffer[T]) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	m.items = nil
}

// ---------------------------------------------------------------------------
// BucketBuffer tests (mockBuffer-backed — deterministic, no timing involved)
// ---------------------------------------------------------------------------

func TestBucketBuffer_PushRoutesToCorrectBucket(t *testing.T) {
	ctx := context.Background()

	indexer := func(x int) int { return x % 3 }
	factory := func() Buffer[int] { return newMockBuffer[int]() }

	b := NewBucketBuffer[int](indexer, factory,
		newMockBuffer[int](), newMockBuffer[int](), newMockBuffer[int]())

	if err := b.Push(ctx, []int{0, 1, 2, 3, 4, 5}); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// bucket 0 gets {0,3}, bucket 1 gets {1,4}, bucket 2 gets {2,5}
	if got := b.buffs[0].Size(); got != 2 {
		t.Errorf("bucket 0 size = %d, want 2", got)
	}
	if got := b.buffs[1].Size(); got != 2 {
		t.Errorf("bucket 1 size = %d, want 2", got)
	}
	if got := b.buffs[2].Size(); got != 2 {
		t.Errorf("bucket 2 size = %d, want 2", got)
	}
}

func TestBucketBuffer_NegativeIndexErrors(t *testing.T) {
	ctx := context.Background()

	indexer := func(x int) int { return -1 }
	factory := func() Buffer[int] { return newMockBuffer[int]() }

	b := NewBucketBuffer[int](indexer, factory, newMockBuffer[int]())

	err := b.Push(ctx, []int{42})
	if err == nil {
		t.Fatal("expected error for negative index, got nil")
	}
}

func TestBucketBuffer_GrowsBucketsOnDemand(t *testing.T) {
	ctx := context.Background()

	// indexer routes everything to bucket 5, but we start with 0 buckets
	indexer := func(x int) int { return 5 }
	factory := func() Buffer[int] { return newMockBuffer[int]() }

	b := NewBucketBuffer[int](indexer, factory) // no initial buffs

	if err := b.Push(ctx, []int{99}); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	if got := len(b.buffs); got != 6 {
		t.Fatalf("len(buffs) = %d, want 6 (indices 0..5 backfilled)", got)
	}
	// buckets 0-4 should exist but be empty
	for i := 0; i < 5; i++ {
		if got := b.buffs[i].Size(); got != 0 {
			t.Errorf("bucket %d size = %d, want 0", i, got)
		}
	}
	if got := b.buffs[5].Size(); got != 1 {
		t.Errorf("bucket 5 size = %d, want 1", got)
	}
}

func TestBucketBuffer_GrowthIsIdempotentAcrossMultiplePushes(t *testing.T) {
	ctx := context.Background()

	indexer := func(x int) int { return x }
	factory := func() Buffer[int] { return newMockBuffer[int]() }

	b := NewBucketBuffer[int](indexer, factory)

	// first push creates buckets 0..2, second push should only need to grow
	// as far as necessary and must not re-create/clobber existing buckets
	if err := b.Push(ctx, []int{2}); err != nil {
		t.Fatalf("first Push failed: %v", err)
	}
	if err := b.buffs[1].Push(ctx, 111); err != nil { // sentinel value in bucket 1
		t.Fatalf("seeding bucket 1 failed: %v", err)
	}

	if err := b.Push(ctx, []int{4}); err != nil {
		t.Fatalf("second Push failed: %v", err)
	}

	if got := len(b.buffs); got != 5 {
		t.Fatalf("len(buffs) = %d, want 5", got)
	}
	// bucket 1's sentinel must have survived the second growth
	v, err := b.buffs[1].Pop(ctx)
	if err != nil || v != 111 {
		t.Errorf("bucket 1 lost its element across growth: v=%v err=%v", v, err)
	}
}

func TestBucketBuffer_PushPropagatesBucketError(t *testing.T) {
	ctx := context.Background()

	indexer := func(x int) int { return 0 }
	factory := func() Buffer[int] { return newMockBuffer[int]() }

	failing := newMockBuffer[int]()
	failing.pushErr = errors.New("boom")

	b := NewBucketBuffer[int](indexer, factory, failing)

	err := b.Push(ctx, []int{1})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected propagated bucket error, got %v", err)
	}
}

func TestBucketBuffer_PopSkipsEmptyBuckets(t *testing.T) {
	ctx := context.Background()

	indexer := func(x int) int { return x }
	factory := func() Buffer[int] { return newMockBuffer[int]() }

	empty := newMockBuffer[int]()
	nonEmpty := newMockBuffer[int]()
	_ = nonEmpty.Push(ctx, 7)

	b := NewBucketBuffer[int](indexer, factory, empty, nonEmpty)

	out, err := b.Pop(ctx)
	if err != nil {
		t.Fatalf("Pop failed: %v", err)
	}
	if len(out) != 1 || out[0] != 7 {
		t.Errorf("Pop() = %v, want [7] (empty bucket skipped)", out)
	}
}

func TestBucketBuffer_Size(t *testing.T) {
	ctx := context.Background()

	indexer := func(x int) int { return x % 2 }
	factory := func() Buffer[int] { return newMockBuffer[int]() }

	b := NewBucketBuffer[int](indexer, factory, newMockBuffer[int](), newMockBuffer[int]())

	if got := b.Size(); got != 0 {
		t.Fatalf("Size() = %d, want 0 before any push", got)
	}

	if err := b.Push(ctx, []int{1, 2, 3, 4}); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	if got := b.Size(); got != 4 {
		t.Errorf("Size() = %d, want 4", got)
	}
}

func TestBucketBuffer_SizeOnNilBuffsIsZero(t *testing.T) {
	var b BucketBuffer[[]int, int]
	if got := b.Size(); got != 0 {
		t.Errorf("Size() on zero-value BucketBuffer = %d, want 0", got)
	}
}

func TestBucketBuffer_CloseClosesAllBucketsAndClearsState(t *testing.T) {
	ctx := context.Background()

	indexer := func(x int) int { return 0 }
	factory := func() Buffer[int] { return newMockBuffer[int]() }

	buck := newMockBuffer[int]()
	b := NewBucketBuffer[int](indexer, factory, buck)
	_ = b.Push(ctx, []int{1})

	b.Close()

	if !buck.closed {
		t.Error("expected underlying bucket to be closed")
	}
	if b.buffs != nil {
		t.Error("expected b.buffs to be nil after Close")
	}
	if got := b.Size(); got != 0 {
		t.Errorf("Size() after Close = %d, want 0", got)
	}
}

func TestBucketBuffer_ConcurrentPushIsSafe(t *testing.T) {
	ctx := context.Background()

	indexer := func(x int) int { return x % 4 }
	factory := func() Buffer[int] { return newMockBuffer[int]() }

	b := NewBucketBuffer[int](indexer, factory,
		newMockBuffer[int](), newMockBuffer[int](), newMockBuffer[int](), newMockBuffer[int]())

	var wg sync.WaitGroup
	const n = 100
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			if err := b.Push(ctx, []int{v}); err != nil {
				t.Errorf("Push(%d) failed: %v", v, err)
			}
		}(i)
	}
	wg.Wait()

	if got := b.Size(); got != n {
		t.Errorf("Size() = %d, want %d (race under concurrent push?)", got, n)
	}
}

// ---------------------------------------------------------------------------
// TimedBuffer-backed BucketBuffer tests — exercises the real TTL/eviction
// path and its interaction with bucket routing.
// ---------------------------------------------------------------------------

func TestBucketBuffer_WithTimedBuffer_RoutesAndPops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const ttl = time.Second // long enough not to expire during the test

	indexer := func(x int) int { return x % 2 }
	factory := func() Buffer[int] {
		return NewTimedBuffer[int](ctx, 16, 1, ttl)
	}

	b := NewBucketBuffer[int](indexer, factory,
		NewTimedBuffer[int](ctx, 16, 1, ttl),
		NewTimedBuffer[int](ctx, 16, 1, ttl),
	)
	defer b.Close()

	if err := b.Push(ctx, []int{10, 11, 12, 13}); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	if !waitFor(t, time.Second, func() bool { return b.Size() == 4 }) {
		t.Fatalf("Size() = %d, want 4 (after waiting)", b.Size())
	}

	out, err := b.Pop(ctx)
	if err != nil {
		t.Fatalf("Pop failed: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("Pop() returned %d elements, want 2 (one per bucket)", len(out))
	}
}

// waitFor polls cond until it's true or the timeout elapses.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return cond()
}

func TestBucketBuffer_WithTimedBuffer_ExpiredElementsAreSkippedOnPop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const ttl = 20 * time.Millisecond

	indexer := func(x int) int { return 0 }
	factory := func() Buffer[int] {
		return NewTimedBuffer[int](ctx, 16, 1, ttl)
	}

	tb := NewTimedBuffer[int](ctx, 16, 1, ttl)
	b := NewBucketBuffer[int](indexer, factory, tb)
	defer b.Close()

	if err := b.Push(ctx, []int{1}); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// let the single element expire
	time.Sleep(ttl + 30*time.Millisecond)

	if err := b.Push(ctx, []int{2}); err != nil {
		t.Fatalf("second Push failed: %v", err)
	}

	// TimedBuffer.Pop loops past expired entries internally; the fresh
	// element (2) should still come back even though 1 aged out first.
	popCtx, popCancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer popCancel()

	out, err := b.Pop(popCtx)
	if err != nil {
		t.Fatalf("Pop failed: %v", err)
	}
	if len(out) != 1 || out[0] != 2 {
		t.Errorf("Pop() = %v, want [2] (expired element skipped)", out)
	}
}

func TestBucketBuffer_WithTimedBuffer_CloseUnblocksPop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const ttl = time.Second

	indexer := func(x int) int { return 0 }
	factory := func() Buffer[int] {
		return NewTimedBuffer[int](ctx, 16, 1, ttl)
	}

	b := NewBucketBuffer[int](indexer, factory, NewTimedBuffer[int](ctx, 16, 1, ttl))

	done := make(chan struct{})
	go func() {
		defer close(done)
		b.Close()
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Close did not return in time")
	}

	if got := b.Size(); got != 0 {
		t.Errorf("Size() after Close = %d, want 0", got)
	}
}
