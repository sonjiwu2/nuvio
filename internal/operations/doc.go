// Package operations is Nuvio's Safe Operation Layer: the only code path
// allowed to actually move a user's files. Every other feature that
// wants to touch the filesystem (internal/rules' Organize, a future
// Duplicates cleanup) goes through Apply, never os.Rename or os.Remove
// directly — see CLAUDE.md sections 3, 17, and 18.
//
// Every successful move is journaled to SQLite before Apply returns, so
// Undo can reverse a batch afterward. Undo re-checks safety at the time
// it runs, not just at the time of the original move: if the destination
// file is gone, or the original source location has since been reoccupied
// by a different file, that item is skipped rather than silently
// clobbering something the user created since — see CLAUDE.md section 21.
//
// Conflicts (something already at the destination) are resolved by an
// explicit ConflictPolicy chosen by the caller; the zero value is Skip,
// the safe default, never Replace.
package operations
