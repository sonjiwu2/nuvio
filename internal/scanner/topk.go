package scanner

import (
	"container/heap"
	"sort"
	"sync"
)

// sized is implemented by anything the top-K collector can rank.
type sized interface {
	sizeBytes() int64
}

func (f FileEntry) sizeBytes() int64   { return f.Size }
func (f FolderEntry) sizeBytes() int64 { return f.Size }

// topK keeps the N largest items seen across an arbitrary number of
// concurrent Add calls, using O(N) memory regardless of how many items are
// offered. This is what lets a scan of millions of files report "largest
// 20" without ever holding all of them in memory at once.
type topK[T sized] struct {
	mu       sync.Mutex
	capacity int
	items    minHeap[T]
}

func newTopK[T sized](capacity int) *topK[T] {
	return &topK[T]{capacity: capacity}
}

func (k *topK[T]) Add(item T) {
	k.mu.Lock()
	defer k.mu.Unlock()

	if len(k.items) < k.capacity {
		heap.Push(&k.items, item)
		return
	}
	if len(k.items) > 0 && item.sizeBytes() > k.items[0].sizeBytes() {
		k.items[0] = item
		heap.Fix(&k.items, 0)
	}
}

// Sorted returns the collected items ordered largest-first.
func (k *topK[T]) Sorted() []T {
	k.mu.Lock()
	defer k.mu.Unlock()

	out := make([]T, len(k.items))
	copy(out, k.items)
	sort.Slice(out, func(i, j int) bool { return out[i].sizeBytes() > out[j].sizeBytes() })
	return out
}

// minHeap keeps its smallest element at index 0, so we can cheaply test
// "is this new item bigger than the smallest one we're keeping".
type minHeap[T sized] []T

func (h minHeap[T]) Len() int            { return len(h) }
func (h minHeap[T]) Less(i, j int) bool  { return h[i].sizeBytes() < h[j].sizeBytes() }
func (h minHeap[T]) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *minHeap[T]) Push(x interface{}) { *h = append(*h, x.(T)) }
func (h *minHeap[T]) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}
