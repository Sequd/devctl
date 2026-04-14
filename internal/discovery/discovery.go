package discovery

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ekorunov/devctl/internal/config"
)

var composePatterns = []string{
	"docker-compose.yml",
	"docker-compose.yaml",
	"compose.yml",
	"compose.yaml",
}

var overridePatterns = []string{
	"docker-compose.override.yml",
	"docker-compose.override.yaml",
	"compose.override.yml",
	"compose.override.yaml",
}

// Discover scans dir for compose files and builds profiles automatically.
func Discover(dir string) (*config.Config, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var baseFiles []string
	var overrideFiles []string
	var extraFiles []string

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)

		if isOverride(lower) {
			overrideFiles = append(overrideFiles, name)
		} else if isBase(lower) {
			baseFiles = append(baseFiles, name)
		} else if isComposeFile(lower) {
			extraFiles = append(extraFiles, name)
		}
	}

	cfg := &config.Config{
		Project: filepath.Base(dir),
	}

	// Base + override → default profile
	if len(baseFiles) > 0 {
		files := make([]string, 0, len(baseFiles)+len(overrideFiles))
		files = append(files, baseFiles...)
		files = append(files, overrideFiles...)
		cfg.Profiles = append(cfg.Profiles, config.Profile{
			Name:    "default",
			Compose: files,
		})
	}

	// Extra compose files → separate profiles
	sort.Strings(extraFiles)
	for _, f := range extraFiles {
		name := profileNameFromFile(f)
		cfg.Profiles = append(cfg.Profiles, config.Profile{
			Name:    name,
			Compose: []string{f},
		})
	}

	return cfg, nil
}

func isBase(name string) bool {
	for _, p := range composePatterns {
		if name == p {
			return true
		}
	}
	return false
}

func isOverride(name string) bool {
	for _, p := range overridePatterns {
		if name == p {
			return true
		}
	}
	return false
}

func isComposeFile(name string) bool {
	prefixes := []string{"docker-compose.", "compose."}
	suffixes := []string{".yml", ".yaml"}
	for _, pre := range prefixes {
		for _, suf := range suffixes {
			if strings.HasPrefix(name, pre) && strings.HasSuffix(name, suf) {
				return true
			}
		}
	}
	return false
}

func profileNameFromFile(filename string) string {
	name := filename
	name = strings.TrimPrefix(name, "docker-compose.")
	name = strings.TrimPrefix(name, "compose.")
	name = strings.TrimSuffix(name, ".yml")
	name = strings.TrimSuffix(name, ".yaml")
	if name == "" {
		return filename
	}
	return name
}
