package duplicates

import (
	"time"

	"github.com/sonjiwu2/nuvio/internal/fswalk"
)

// Options configures a single duplicate scan.
type Options struct {
	// Workers bounds how many directories can be read, and how many files
	// can be hashed, concurrently. Zero selects a sensible default.
	Workers int

	// TopN is how many duplicate groups are kept, ranked by reclaimable
	// space (the size that would be freed by keeping one copy and
	// removing the rest). Zero selects a sensible default. Unlike
	// internal/search's MaxMatches, this does not stop the scan early —
	// every file must be seen once before it's known which ones are
	// actually duplicated, so TopN only bounds the final report.
	TopN int

	// MinFileSize skips files smaller than this. Hashing tiny files finds
	// "duplicates" (empty files, near-empty config stubs) that aren't
	// worth a user's attention and waste time better spent on files that
	// matter. Zero selects a sensible default.
	MinFileSize int64

	// ProgressInterval throttles how often the progress callback fires.
	// Zero selects a sensible default.
	ProgressInterval time.Duration
}

const (
	defaultTopN             = 50
	defaultMinFileSize      = 4096 // 4 KiB
	defaultProgressInterval = 200 * time.Millisecond
	maxWorkers              = 16
)

func (o Options) withDefaults() Options {
	if o.Workers <= 0 {
		o.Workers = fswalk.DefaultWorkerCount()
	}
	if o.Workers > maxWorkers {
		o.Workers = maxWorkers
	}
	if o.TopN <= 0 {
		o.TopN = defaultTopN
	}
	if o.MinFileSize <= 0 {
		o.MinFileSize = defaultMinFileSize
	}
	if o.ProgressInterval <= 0 {
		o.ProgressInterval = defaultProgressInterval
	}
	return o
}

// Phase identifies which stage of the pipeline a Progress snapshot was
// taken during.
type Phase string

const (
	PhaseScanning Phase = "scanning"
	PhaseHashing  Phase = "hashing"
)

// Progress is a snapshot of scan counters, delivered on a throttled
// interval so the UI never has to process one event per file.
type Progress struct {
	Phase        Phase  `json:"phase"`
	FilesScanned int64  `json:"filesScanned"`
	FilesHashed  int64  `json:"filesHashed"`
	CurrentPath  string `json:"currentPath"`
}

// File is one member of a duplicate Group.
type File struct {
	Path    string    `json:"path"`
	ModTime time.Time `json:"modTime"`
}

// Group is a set of files with identical content.
type Group struct {
	Hash  string `json:"hash"`
	Size  int64  `json:"size"`
	Files []File `json:"files"`
}

// Reclaimable is the space freed by keeping exactly one copy from the
// group and removing the rest.
func (g Group) Reclaimable() int64 {
	if len(g.Files) <= 1 {
		return 0
	}
	return g.Size * int64(len(g.Files)-1)
}

// Weight makes Group usable with internal/topk's Collector, ranking
// duplicate groups by reclaimable space rather than raw file size.
func (g Group) Weight() int64 { return g.Reclaimable() }

// Issue records a single path Nuvio could not fully read. One
// inaccessible folder or file must never abort the rest of the scan.
type Issue struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// Result is the outcome of a completed or cancelled duplicate scan.
type Result struct {
	Root             string  `json:"root"`
	FilesScanned     int64   `json:"filesScanned"`
	FilesHashed      int64   `json:"filesHashed"`
	Groups           []Group `json:"groups"`
	TotalReclaimable int64   `json:"totalReclaimable"`
	Truncated        bool    `json:"truncated"`
	Issues           []Issue `json:"issues"`
	Cancelled        bool    `json:"cancelled"`
	// Duration is serialized as nanoseconds (Go's default time.Duration
	// JSON encoding) — not milliseconds despite the field name.
	Duration time.Duration `json:"durationNs"`
}
