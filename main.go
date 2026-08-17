package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/sonjiwu2/nuvio/internal/logging"
	"github.com/sonjiwu2/nuvio/internal/persistence"
	"github.com/sonjiwu2/nuvio/internal/platform"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "nuvio: fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	logDir, err := platform.LogDir()
	if err != nil {
		return fmt.Errorf("resolve log directory: %w", err)
	}

	logger, closeLog, err := logging.New(logDir)
	if err != nil {
		return fmt.Errorf("initialise logger: %w", err)
	}
	defer func() {
		if err := closeLog.Close(); err != nil {
			fmt.Fprintln(os.Stderr, "nuvio: close log file:", err)
		}
	}()

	dataDir, err := platform.AppDataDir()
	if err != nil {
		return fmt.Errorf("resolve app data directory: %w", err)
	}

	db, err := persistence.Open(dataDir)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("close database", "error", err)
		}
	}()

	app := NewApp(logger, db)
	logger.Info("nuvio started")

	return wails.Run(&options.App{
		Title:     "Nuvio",
		Width:     1440,
		Height:    900,
		MinWidth:  1024,
		MinHeight: 700,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 247, G: 248, B: 250, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})
}
