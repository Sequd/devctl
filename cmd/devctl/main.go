package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ekorunov/devctl/internal/config"
	"github.com/ekorunov/devctl/internal/discovery"
	"github.com/ekorunov/devctl/internal/ui"
)

func main() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Parse subcommand
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "init" {
		runInit(dir)
		return
	}

	// Allow overriding directory via argument
	if len(args) > 0 {
		dir, err = filepath.Abs(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}

	// Try loading config
	cfg, err := config.Load(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	// Fallback to auto-detection
	hasConfig := cfg != nil
	if cfg == nil {
		cfg, err = discovery.Discover(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "discovery error: %v\n", err)
			os.Exit(1)
		}
	}

	if len(cfg.Profiles) == 0 {
		fmt.Fprintln(os.Stderr, "No compose files or profiles found.")
		fmt.Fprintln(os.Stderr, "Run 'devctl init' to create a config, or add docker-compose.yml to the project.")
		os.Exit(1)
	}

	model := ui.New(cfg, dir, hasConfig)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runInit(dir string) {
	// Check if config already exists
	existing, _ := config.Load(dir)
	if existing != nil {
		fmt.Fprintln(os.Stderr, "Config already exists: .devtool/docker.yaml")
		os.Exit(1)
	}

	// Discover compose files
	cfg, err := discovery.Discover(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "discovery error: %v\n", err)
		os.Exit(1)
	}

	if len(cfg.Profiles) == 0 {
		fmt.Fprintln(os.Stderr, "No compose files found in current directory.")
		fmt.Fprintln(os.Stderr, "Add docker-compose.yml or compose.yml first.")
		os.Exit(1)
	}

	// Write config
	path, err := config.Init(dir, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created %s\n\n", path)
	fmt.Println("Detected profiles:")
	for _, p := range cfg.Profiles {
		fmt.Printf("  - %s\n", p.Name)
		for _, f := range p.Compose {
			fmt.Printf("      %s\n", f)
		}
	}
	fmt.Println("\nEdit the file to customize profiles, then run 'devctl' to start.")
}
