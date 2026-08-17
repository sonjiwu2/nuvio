package topk

import (
	"sync"
	"testing"
)

type item struct {
	name string
	size int64
}

func (i item) Weight() int64 { return i.size }

func TestCollector_KeepsOnlyHeaviestUpToCapacity(t *testing.T) {
	c := New[item](3)
	sizes := []int64{5, 1, 9, 3, 7, 2, 8}
	for i, s := range sizes {
		c.Add(item{name: string(rune('a' + i)), size: s})
	}

	got := c.Sorted()
	if len(got) != 3 {
		t.Fatalf("len(Sorted()) = %d, want 3", len(got))
	}
	want := []int64{9, 8, 7}
	for i, w := range want {
		if got[i].size != w {
			t.Errorf("Sorted()[%d].size = %d, want %d", i, got[i].size, w)
		}
	}
}

func TestCollector_EmptyWhenNothingAdded(t *testing.T) {
	c := New[item](5)
	if got := c.Sorted(); len(got) != 0 {
		t.Errorf("Sorted() = %v, want empty", got)
	}
}

func TestCollector_KeepsFewerThanCapacityIfFewerAdded(t *testing.T) {
	c := New[item](10)
	c.Add(item{name: "a", size: 1})
	c.Add(item{name: "b", size: 2})

	if got := c.Sorted(); len(got) != 2 {
		t.Errorf("len(Sorted()) = %d, want 2", len(got))
	}
}

func TestCollector_ConcurrentAddIsRaceFree(t *testing.T) {
	c := New[item](10)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			c.Add(item{name: "f", size: int64(n)})
		}(i)
	}
	wg.Wait()

	if len(c.Sorted()) != 10 {
		t.Errorf("len(Sorted()) = %d, want 10", len(c.Sorted()))
	}
}
