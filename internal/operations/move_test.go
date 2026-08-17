package operations

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func TestMoveOne_MovesFileWhenNoConflict(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "a.txt")
	dest := filepath.Join(root, "dest", "a.txt")
	writeFile(t, source, "hello")

	actual, outcome, err := moveOne(source, dest, ConflictSkip)
	if err != nil {
		t.Fatalf("moveOne returned error: %v", err)
	}
	if outcome != OutcomeMoved {
		t.Errorf("outcome = %v, want OutcomeMoved", outcome)
	}
	if actual != dest {
		t.Errorf("actual = %q, want %q", actual, dest)
	}
	if exists(source) {
		t.Error("source still exists after move")
	}
	if got := readFile(t, dest); got != "hello" {
		t.Errorf("destination content = %q, want %q", got, "hello")
	}
}

func TestMoveOne_SkipPolicyLeavesBothFilesUntouched(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "a.txt")
	dest := filepath.Join(root, "b.txt")
	writeFile(t, source, "source content")
	writeFile(t, dest, "existing content")

	actual, outcome, err := moveOne(source, dest, ConflictSkip)
	if err != nil {
		t.Fatalf("moveOne returned error: %v", err)
	}
	if outcome != OutcomeSkipped {
		t.Errorf("outcome = %v, want OutcomeSkipped", outcome)
	}
	if actual != "" {
		t.Errorf("actual = %q, want empty", actual)
	}
	if got := readFile(t, source); got != "source content" {
		t.Error("source was modified despite skip policy")
	}
	if got := readFile(t, dest); got != "existing content" {
		t.Error("destination was overwritten despite skip policy")
	}
}

func TestMoveOne_DefaultPolicyIsSkip(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "a.txt")
	dest := filepath.Join(root, "b.txt")
	writeFile(t, source, "source")
	writeFile(t, dest, "existing")

	_, outcome, err := moveOne(source, dest, "")
	if err != nil {
		t.Fatalf("moveOne returned error: %v", err)
	}
	if outcome != OutcomeSkipped {
		t.Errorf("outcome = %v, want OutcomeSkipped (zero-value policy must be the safe default)", outcome)
	}
}

func TestMoveOne_KeepBothRenamesIncomingFile(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "a.txt")
	dest := filepath.Join(root, "b.txt")
	writeFile(t, source, "incoming")
	writeFile(t, dest, "existing")

	actual, outcome, err := moveOne(source, dest, ConflictKeepBoth)
	if err != nil {
		t.Fatalf("moveOne returned error: %v", err)
	}
	if outcome != OutcomeMoved {
		t.Fatalf("outcome = %v, want OutcomeMoved", outcome)
	}
	want := filepath.Join(root, "b (1).txt")
	if actual != want {
		t.Errorf("actual = %q, want %q", actual, want)
	}
	if got := readFile(t, dest); got != "existing" {
		t.Error("original destination file was modified")
	}
	if got := readFile(t, want); got != "incoming" {
		t.Error("renamed file does not have the incoming content")
	}
	if exists(source) {
		t.Error("source still exists after move")
	}
}

func TestMoveOne_ReplacePolicyOverwritesDestination(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "a.txt")
	dest := filepath.Join(root, "b.txt")
	writeFile(t, source, "new content")
	writeFile(t, dest, "old content")

	actual, outcome, err := moveOne(source, dest, ConflictReplace)
	if err != nil {
		t.Fatalf("moveOne returned error: %v", err)
	}
	if outcome != OutcomeMoved {
		t.Fatalf("outcome = %v, want OutcomeMoved", outcome)
	}
	if actual != dest {
		t.Errorf("actual = %q, want %q", actual, dest)
	}
	if got := readFile(t, dest); got != "new content" {
		t.Errorf("destination content = %q, want %q (replace must overwrite)", got, "new content")
	}
	if exists(source) {
		t.Error("source still exists after move")
	}
}

func TestMoveOne_CreatesDestinationDirectory(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "a.txt")
	dest := filepath.Join(root, "deep", "nested", "path", "a.txt")
	writeFile(t, source, "content")

	_, outcome, err := moveOne(source, dest, ConflictSkip)
	if err != nil {
		t.Fatalf("moveOne returned error: %v", err)
	}
	if outcome != OutcomeMoved {
		t.Fatalf("outcome = %v, want OutcomeMoved", outcome)
	}
	if !exists(dest) {
		t.Error("destination file does not exist after move into new nested directory")
	}
}

func TestMoveOne_FailsCleanlyWhenDestinationDirCannotBeCreated(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "a.txt")
	writeFile(t, source, "content")

	// blocker is a FILE, so MkdirAll trying to create a directory
	// underneath it must fail on every platform — a portable way to
	// force a failure without relying on chmod.
	blocker := filepath.Join(root, "blocker")
	writeFile(t, blocker, "im a file, not a directory")
	dest := filepath.Join(blocker, "sub", "a.txt")

	_, outcome, err := moveOne(source, dest, ConflictSkip)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if outcome != OutcomeFailed {
		t.Errorf("outcome = %v, want OutcomeFailed", outcome)
	}
	if !exists(source) {
		t.Error("source was removed despite the move failing — a failed move must never lose the original file")
	}
}

func TestUniquePath_FindsFirstFreeSlot(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "file.txt")
	writeFile(t, base, "x")
	writeFile(t, filepath.Join(root, "file (1).txt"), "x")

	got := uniquePath(base)
	want := filepath.Join(root, "file (2).txt")
	if got != want {
		t.Errorf("uniquePath() = %q, want %q", got, want)
	}
}

func TestCopyThenRemove_MovesFileAndVerifiesSize(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "a.txt")
	dest := filepath.Join(root, "b.txt")
	writeFile(t, source, "some content to copy")

	if err := copyThenRemove(source, dest); err != nil {
		t.Fatalf("copyThenRemove returned error: %v", err)
	}
	if exists(source) {
		t.Error("source still exists after copyThenRemove")
	}
	if got := readFile(t, dest); got != "some content to copy" {
		t.Errorf("destination content = %q, want %q", got, "some content to copy")
	}
}

func TestCopyThenRemove_RefusesToOverwriteExistingDestination(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "a.txt")
	dest := filepath.Join(root, "b.txt")
	writeFile(t, source, "new")
	writeFile(t, dest, "must survive")

	if err := copyThenRemove(source, dest); err == nil {
		t.Fatal("expected an error copying onto an existing file (O_EXCL), got nil")
	}
	if got := readFile(t, dest); got != "must survive" {
		t.Error("destination content was overwritten despite the copy being expected to fail")
	}
	if !exists(source) {
		t.Error("source was removed despite the copy failing")
	}
}
