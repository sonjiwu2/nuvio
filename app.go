package main

import (
	"context"
	"database/sql"
	"log/slog"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/sonjiwu2/nuvio/internal/duplicates"
	"github.com/sonjiwu2/nuvio/internal/operations"
	"github.com/sonjiwu2/nuvio/internal/rules"
	"github.com/sonjiwu2/nuvio/internal/scanner"
	"github.com/sonjiwu2/nuvio/internal/search"
)

// App is the Wails application bridge. It stays thin: it owns the runtime
// context and exposes bound methods to the frontend, but delegates all real
// work to internal/ domain packages. Bound methods for a given feature live
// in their own app_<feature>.go file (app_scan.go, app_search.go, ...) to
// keep this file from becoming a dumping ground as Nuvio grows.
type App struct {
	ctx        context.Context
	logger     *slog.Logger
	scans      *scanner.Coordinator
	searchC    *search.Coordinator
	rulesStore *rules.Store
	previews   *rules.Coordinator
	dupes      *duplicates.Coordinator
	journal    *operations.Journal
	applies    *operations.Coordinator
}

// NewApp creates a new App application struct. db is Nuvio's single
// shared SQLite connection, already open and migrated by main.go.
func NewApp(logger *slog.Logger, db *sql.DB) *App {
	journal := operations.NewJournal(db)
	return &App{
		logger:     logger,
		scans:      scanner.NewCoordinator(),
		searchC:    search.NewCoordinator(),
		rulesStore: rules.NewStore(db),
		previews:   rules.NewCoordinator(),
		dupes:      duplicates.NewCoordinator(),
		journal:    journal,
		applies:    operations.NewCoordinator(journal),
	}
}

// startup is called when the app starts. The context is saved so we can
// call Wails runtime methods (e.g. dialogs, events) from bound methods.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// PickFolder opens a native "choose a folder" dialog and returns the
// chosen absolute path, or "" if the user cancelled the dialog.
func (a *App) PickFolder() (string, error) {
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Choose a folder",
	})
}
