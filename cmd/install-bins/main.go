package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devakxhay/lighthouse/internal/bins"
	"github.com/devakxhay/lighthouse/internal/bins/tui"
	"github.com/devakxhay/lighthouse/internal/db"
)

func main() {
	dbPath := flag.String("db", "/var/lib/lighthouse/lighthouse.db", "Path to lighthouse sqlite database")
	flag.Parse()

	// Ensure caller is root
	if os.Getuid() != 0 {
		fmt.Fprintln(os.Stderr, "\x1b[1;31mError: This command must be run as root (using sudo).\x1b[0m")
		fmt.Fprintln(os.Stderr, "Example: sudo lighthouse-install-bins")
		os.Exit(1)
	}

	// Setup a silent/error-only logger for DB operations so we don't mess up TUI
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	database, err := db.New(*dbPath, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\x1b[1;31mError: failed to open database at %s: %v\x1b[0m\n", *dbPath, err)
		os.Exit(1)
	}

	specs := bins.GetSpecs(database)
	model := tui.NewModel(database, specs)

	p := tea.NewProgram(model)
	model.SetProgram(p)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "\x1b[1;31mError: bubbletea program failed: %v\x1b[0m\n", err)
		os.Exit(1)
	}
}
