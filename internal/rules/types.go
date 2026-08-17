package rules

import (
	"time"

	"github.com/sonjiwu2/nuvio/internal/fswalk"
)

// Rule maps a file extension to a destination folder.
type Rule struct {
	ID                string    `json:"id"`
	Extension         string    `json:"extension"`
	DestinationFolder string    `json:"destinationFolder"`
	CreatedAt         time.Time `json:"createdAt"`
}

// PreviewOptions configures a single dry-run preview.
type PreviewOptions struct {
	// Workers bounds how many directories can be read concurrently. Zero
	// selects a sensible default, same reasoning as internal/scanner.
	Workers int

	// MaxEntries bounds how many preview entries are kept in memory. Zero
	// selects a sensible default. Once reached, the preview stops early,
	// the same way internal/search stops once MaxMatches is reached.
	MaxEntries int

	// BatchInterval throttles how often newly found entries and progress
	// are delivered together. Zero selects a sensible default.
	BatchInterval time.Duration
}

const (
	defaultMaxEntries    = 1000
	defaultBatchInterval = 200 * time.Millisecond
	maxWorkers           = 16
)

func (o PreviewOptions) withDefaults() PreviewOptions {
	if o.Workers <= 0 {
		o.Workers = fswalk.DefaultWorkerCount()
	}
	if o.Workers > maxWorkers {
		o.Workers = maxWorkers
	}
	if o.MaxEntries <= 0 {
		o.MaxEntries = defaultMaxEntries
	}
	if o.BatchInterval <= 0 {
		o.BatchInterval = defaultBatchInterval
	}
	return o
}

// PreviewEntry describes one file that matched a rule during a dry-run
// preview — what WOULD happen, not what did.
type PreviewEntry struct {
	SourcePath      string `json:"sourcePath"`
	Name            string `json:"name"`
	Size            int64  `json:"size"`
	DestinationPath string `json:"destinationPath"`
	RuleID          string `json:"ruleId"`
}

// PreviewIssue records a single path Nuvio could not fully read. One
// inaccessible folder must never abort the rest of the preview.
type PreviewIssue struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// PreviewProgress is a snapshot of preview counters, delivered on a
// throttled interval so the UI never has to process one event per file.
type PreviewProgress struct {
	FilesScanned int64  `json:"filesScanned"`
	DirsScanned  int64  `json:"dirsScanned"`
	MatchesFound int64  `json:"matchesFound"`
	CurrentPath  string `json:"currentPath"`
}

// PreviewResult is the outcome of a completed or cancelled preview.
type PreviewResult struct {
	Root         string         `json:"root"`
	FilesScanned int64          `json:"filesScanned"`
	DirsScanned  int64          `json:"dirsScanned"`
	MatchCount   int64          `json:"matchCount"`
	TotalSize    int64          `json:"totalSize"`
	Truncated    bool           `json:"truncated"`
	Issues       []PreviewIssue `json:"issues"`
	Cancelled    bool           `json:"cancelled"`
	// Duration is serialized as nanoseconds (Go's default time.Duration
	// JSON encoding) — not milliseconds despite the field name.
	Duration time.Duration `json:"durationNs"`
}
