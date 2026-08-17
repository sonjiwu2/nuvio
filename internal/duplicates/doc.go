// Package duplicates finds groups of files with identical content inside
// a chosen folder: size grouping, then a partial-content hash to cheaply
// reject false candidates, then a full hash to confirm — see CLAUDE.md
// section 93. It never deletes, moves, or otherwise touches a file; like
// internal/rules' Preview, actually reclaiming the space a duplicate
// group wastes is deferred until Nuvio has a Safe Operation Layer
// (journal, trash, undo — CLAUDE.md sections 3, 17, 18, 94).
package duplicates
