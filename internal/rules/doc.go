// Package rules implements Nuvio's Organize feature: extension-based
// rules ("*.pdf -> Documents/PDFs") persisted in SQLite, and a dry-run
// Preview that reports what those rules would do to a chosen folder.
//
// Preview never moves, renames, or deletes anything. CLAUDE.md's Safe
// Operation Layer (an operation journal, a real trash/recovery path,
// named-conflict handling, and undo) is a prerequisite this project sets
// for any code path that actually mutates the user's files — see
// CLAUDE.md sections 3, 17, 18, and 94 ("Только после этого разрешены
// серьёзные автоматические file modifications"). That layer does not
// exist yet, so Preview is deliberately the only capability this package
// offers today: rules can be defined and reviewed, never applied.
package rules
