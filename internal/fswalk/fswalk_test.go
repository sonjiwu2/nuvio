package fswalk

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func readSingleEntry(t *testing.T, dir string) os.DirEntry {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%q): %v", dir, err)
	}
	if len(entries) != 1 {
		t.Fatalf("ReadDir(%q) returned %d entries, want 1", dir, len(entries))
	}
	return entries[0]
}

func TestClassify_RegularFileIsKindFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	kind, err := Classify(filePath, readSingleEntry(t, dir))
	if err != nil {
		t.Fatalf("Classify returned error: %v", err)
	}
	if kind != KindFile {
		t.Errorf("kind = %v, want KindFile", kind)
	}
}

func TestClassify_RealDirectoryIsKindDirectory(t *testing.T) {
	dir := t.TempDir()
	subPath := filepath.Join(dir, "sub")
	if err := os.Mkdir(subPath, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	kind, err := Classify(subPath, readSingleEntry(t, dir))
	if err != nil {
		t.Fatalf("Classify returned error: %v", err)
	}
	if kind != KindDirectory {
		t.Errorf("kind = %v, want KindDirectory", kind)
	}
}

func TestClassify_JunctionIsKindFile(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("junctions are a Windows-specific reparse point type")
	}

	dir := t.TempDir()
	target := filepath.Join(dir, "real")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	link := filepath.Join(dir, "link")

	cmd := exec.Command("cmd", "/c", "mklink", "/J", link, target)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("junction creation not available in this environment: %v (%s)", err, out)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var linkEntry os.DirEntry
	for _, e := range entries {
		if e.Name() == "link" {
			linkEntry = e
		}
	}
	if linkEntry == nil {
		t.Fatal("link entry not found in directory listing")
	}

	kind, err := Classify(link, linkEntry)
	if err != nil {
		t.Fatalf("Classify returned error: %v", err)
	}
	if kind != KindFile {
		t.Errorf("kind = %v, want KindFile (junctions must not be classified as recursable directories)", kind)
	}
}

func TestPool_BoundsConcurrency(t *testing.T) {
	const workers = 3
	pool := NewPool(workers)
	var current, max atomic.Int32
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		pool.Go(&wg, func() {
			defer wg.Done()
			n := current.Add(1)
			for {
				m := max.Load()
				if n <= m || max.CompareAndSwap(m, n) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			current.Add(-1)
		})
	}
	wg.Wait()

	// Not workers exactly: the single test goroutine driving this loop is
	// itself an unpooled caller, so when it hits saturation it becomes one
	// extra concurrent thread of control running inline — see Pool's doc
	// comment. workers+1 is the real, intentional worst case.
	if got := max.Load(); got > workers+1 {
		t.Errorf("observed %d concurrent Pool.Go executions, want <= %d", got, workers+1)
	}
}

func TestPool_RunsInlineWhenSaturated(t *testing.T) {
	// A zero-capacity pool can never win the semaphore send, so every
	// call must fall through to running fn synchronously inline.
	pool := NewPool(0)

	var ran bool
	var wg sync.WaitGroup
	wg.Add(1)
	pool.Go(&wg, func() {
		defer wg.Done()
		ran = true
	})

	// If fn had instead been spawned on a goroutine, it could easily
	// still be false here — Go would have returned before the scheduler
	// even ran it. Asserting immediately, before any Wait, is what makes
	// this test actually distinguish "inline" from "spawned".
	if !ran {
		t.Error("fn had not run by the time Pool.Go returned, implying it was spawned on a goroutine despite zero pool capacity")
	}
	wg.Wait()
}
