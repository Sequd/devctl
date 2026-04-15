package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscover(t *testing.T) {
	dir := t.TempDir()

	// Create compose files
	files := []string{
		"docker-compose.yml",
		"docker-compose.override.yml",
		"docker-compose.api.yml",
		"docker-compose.monitoring.yml",
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("version: '3'\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Project != filepath.Base(dir) {
		t.Errorf("Project: got %q, want %q", cfg.Project, filepath.Base(dir))
	}

	// Should have 3 profiles: default, api, monitoring
	if len(cfg.Profiles) != 3 {
		t.Fatalf("expected 3 profiles, got %d: %+v", len(cfg.Profiles), cfg.Profiles)
	}

	// Default profile should have base + override
	def := cfg.Profiles[0]
	if def.Name != "default" {
		t.Errorf("first profile name: got %q, want %q", def.Name, "default")
	}
	if len(def.Compose) != 2 {
		t.Errorf("default compose files: got %d, want 2", len(def.Compose))
	}

	// Extra profiles should be sorted alphabetically
	if cfg.Profiles[1].Name != "api" {
		t.Errorf("second profile: got %q, want %q", cfg.Profiles[1].Name, "api")
	}
	if cfg.Profiles[2].Name != "monitoring" {
		t.Errorf("third profile: got %q, want %q", cfg.Profiles[2].Name, "monitoring")
	}
}

func TestDiscoverNoFiles(t *testing.T) {
	dir := t.TempDir()

	cfg, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Profiles) != 0 {
		t.Errorf("expected 0 profiles, got %d", len(cfg.Profiles))
	}
}

func TestProfileNameFromFile(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"docker-compose.api.yml", "api"},
		{"docker-compose.monitoring.yaml", "monitoring"},
		{"compose.dev.yml", "dev"},
		{"compose.yml", "yml"}, // strips "compose." prefix, leaves "yml"
	}

	for _, tt := range tests {
		got := profileNameFromFile(tt.input)
		if got != tt.expected && !(tt.expected == "" && got == tt.input) {
			t.Errorf("profileNameFromFile(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestIsComposeFile(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"docker-compose.yml", true},
		{"docker-compose.api.yml", true},
		{"compose.yaml", true},
		{"readme.md", false},
		{"dockerfile", false},
	}

	for _, tt := range tests {
		if got := isComposeFile(tt.name); got != tt.expected {
			t.Errorf("isComposeFile(%q) = %v, want %v", tt.name, got, tt.expected)
		}
	}
}
