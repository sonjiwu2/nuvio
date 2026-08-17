package search

import (
	"time"

	"github.com/sonjiwu2/nuvio/internal/fswalk"
)

// Options configures a single search run.
type Options struct {
	// Query is matched against file names as a case-insensitive substring.
	// An empty Query is rejected by Find — searching for "everything" is
	// almost never what the user actually wants, and it would turn a
	// filename search into an expensive full re-scan.
	Query string

	// Workers bounds how many directories can be read concurrently. Zero
	// selects a sensible default, same reasoning as internal/scanner.
	Workers int

	// MaxMatches bounds how many matches are kept in memory. Zero selects
	// a sensible default. Once reached, the search stops early (further
	// scanning to count matches beyond the cap isn't useful) and the
	// result is marked Truncated.
	MaxMatches int

	// BatchInterval throttles how often newly found matches and progress
	// are delivered together. Zero selects a sensible default.
	BatchInterval time.Duration
}

const (
	defaultMaxMatches    = 1000
	defaultBatchInterval = 200 * time.Millisecond
	maxWorkers           = 16
)

func (o Options) withDefaults() Options {
	if o.Workers <= 0 {
		o.Workers = fswalk.DefaultWorkerCount()
	}
	if o.Workers > maxWorkers {
		o.Workers = maxWorkers
	}
	if o.MaxMatches <= 0 {
		o.MaxMatches = defaultMaxMatches
	}
	if o.BatchInterval <= 0 {
		o.BatchInterval = defaultBatchInterval
	}
	return o
}

// Match is a single file whose name matched the query.
type Match struct {
	Path    string    `json:"path"`
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
}

// Issue records a single path Nuvio could not fully read. One
// inaccessible folder must never abort the rest of the search.
type Issue struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// Progress is a snapshot of search counters, delivered on a throttled
// interval so the UI never has to process one event per file.
type Progress struct {
	FilesScanned int64  `json:"filesScanned"`
	DirsScanned  int64  `json:"dirsScanned"`
	MatchesFound int64  `json:"matchesFound"`
	CurrentPath  string `json:"currentPath"`
}

// Result is the outcome of a completed or cancelled search.
type Result struct {
	Root         string  `json:"root"`
	Query        string  `json:"query"`
	FilesScanned int64   `json:"filesScanned"`
	DirsScanned  int64   `json:"dirsScanned"`
	MatchCount   int64   `json:"matchCount"`
	Truncated    bool    `json:"truncated"`
	Issues       []Issue `json:"issues"`
	Cancelled    bool    `json:"cancelled"`
	// Duration is serialized as nanoseconds (Go's default time.Duration
	// JSON encoding) — not milliseconds despite the field name.
	Duration time.Duration `json:"durationNs"`
}
