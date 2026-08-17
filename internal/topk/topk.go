// Package topk provides a bounded "keep the N largest items" collector,
// shared by any feature that ranks results by some weight while walking
// a tree too large to hold in memory (internal/scanner's largest
// files/folders, internal/duplicates' largest duplicate groups).
package topk

import (
	"container/heap"
	"sort"
	"sync"
)

// Weighted is implemented by anything the collector can rank. The method
// is named generically — Weight, not Size — because the ranking value
// isn't always a file size: internal/duplicates ranks groups by
// reclaimable space, for instance.
type Weighted interface {
	Weight() int64
}

// Collector keeps the N largest items (by Weight) seen across an
// arbitrary number of concurrent Add calls, using O(N) memory regardless
// of how many items are offered. This is what lets a scan of millions of
// files report "largest 20" without ever holding all of them in memory
// at once.
type Collector[T Weighted] struct {
	mu       sync.Mutex
	capacity int
	items    minHeap[T]
}

// New creates a Collector that keeps at most capacity items.
func New[T Weighted](capacity int) *Collector[T] {
	return &Collector[T]{capacity: capacity}
}

// Add offers item for inclusion. It is kept if there is spare capacity,
// or if it outweighs the current lightest kept item (which is then
// evicted).
func (c *Collector[T]) Add(item T) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.items) < c.capacity {
		heap.Push(&c.items, item)
		return
	}
	if len(c.items) > 0 && item.Weight() > c.items[0].Weight() {
		c.items[0] = item
		heap.Fix(&c.items, 0)
	}
}

// Sorted returns the collected items ordered heaviest-first.
func (c *Collector[T]) Sorted() []T {
	c.mu.Lock()
	defer c.mu.Unlock()

	out := make([]T, len(c.items))
	copy(out, c.items)
	sort.Slice(out, func(i, j int) bool { return out[i].Weight() > out[j].Weight() })
	return out
}

// minHeap keeps its lightest element at index 0, so Collector can cheaply
// test "is this new item heavier than the lightest one we're keeping".
type minHeap[T Weighted] []T

func (h minHeap[T]) Len() int            { return len(h) }
func (h minHeap[T]) Less(i, j int) bool  { return h[i].Weight() < h[j].Weight() }
func (h minHeap[T]) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *minHeap[T]) Push(x interface{}) { *h = append(*h, x.(T)) }
func (h *minHeap[T]) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}
