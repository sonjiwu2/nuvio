package scanner

import "time"

// Options configures a single scan run.
type Options struct {
	// Workers bounds how many directories can be read concurrently. Zero
	// selects a sensible default. Filesystem walks are I/O-bound, so more
	// workers is not always faster — network shares and spinning disks
	// degrade under high concurrency.
	Workers int

	// TopN is how many entries are kept in the largest-files and
	// largest-folders results. Zero selects a sensible default.
	TopN int

	// ProgressInterval throttles how often the progress callback fires.
	// Zero selects a sensible default.
	ProgressInterval time.Duration
}

const (
	defaultTopN             = 20
	defaultProgressInterval = 150 * time.Millisecond
	maxWorkers              = 16
)

func (o Options) withDefaults() Options {
	if o.Workers <= 0 {
		o.Workers = defaultWorkerCount()
	}
	if o.Workers > maxWorkers {
		o.Workers = maxWorkers
	}
	if o.TopN <= 0 {
		o.TopN = defaultTopN
	}
	if o.ProgressInterval <= 0 {
		o.ProgressInterval = defaultProgressInterval
	}
	return o
}

// Progress is a snapshot of scan counters, delivered on a throttled
// interval so the UI never has to process one event per file.
type Progress struct {
	FilesScanned int64  `json:"filesScanned"`
	DirsScanned  int64  `json:"dirsScanned"`
	BytesScanned int64  `json:"bytesScanned"`
	CurrentPath  string `json:"currentPath"`
}

// FileEntry describes a single file surfaced in the largest-files result.
type FileEntry struct {
	Path    string    `json:"path"`
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
}

// FolderEntry describes a single directory surfaced in the
// largest-folders result. Size is the recursive total of everything
// beneath it, not just its direct children.
type FolderEntry struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// ScanIssue records a single path Nuvio could not fully read. One
// inaccessible folder must never abort the rest of the scan — issues are
// aggregated here and surfaced to the user afterward.
type ScanIssue struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// Result is the outcome of a completed or cancelled scan.
type Result struct {
	Root       string        `json:"root"`
	TotalSize  int64         `json:"totalSize"`
	TotalFiles int64         `json:"totalFiles"`
	TotalDirs  int64         `json:"totalDirs"`
	TopFiles   []FileEntry   `json:"topFiles"`
	TopFolders []FolderEntry `json:"topFolders"`
	Issues     []ScanIssue   `json:"issues"`
	Cancelled  bool          `json:"cancelled"`
	// Duration is serialized as nanoseconds (Go's default time.Duration
	// JSON encoding, i.e. int64) — not milliseconds despite the field name.
	Duration time.Duration `json:"durationNs"`
}
