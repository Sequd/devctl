package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

type Profile struct {
	Name     string   `yaml:"name"`
	Compose  []string `yaml:"compose"`
	Services []string `yaml:"services,omitempty"`
}

type Config struct {
	Project  string    `yaml:"project"`
	Profiles []Profile `yaml:"profiles"`
}

const configPath = ".devtool/docker.yaml"

// Load reads the config from .devtool/docker.yaml in the given directory.
// Returns nil, nil if the file doesn't exist (caller should use discovery).
func Load(dir string) (*Config, error) {
	path := filepath.Join(dir, configPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.Project == "" {
		cfg.Project = filepath.Base(dir)
	}

	for i := range cfg.Profiles {
		if len(cfg.Profiles[i].Compose) == 0 {
			return nil, fmt.Errorf("profile %q has no compose files", cfg.Profiles[i].Name)
		}
	}

	return &cfg, nil
}

// Path returns the absolute path to the config file.
func Path(dir string) string {
	return filepath.Join(dir, configPath)
}

// Save writes the config to .devtool/docker.yaml with instructional comments.
func Save(dir string, cfg *Config) error {
	dotDir := filepath.Join(dir, ".devtool")
	if err := os.MkdirAll(dotDir, 0o755); err != nil {
		return fmt.Errorf("create .devtool: %w", err)
	}

	content, err := renderConfig(cfg)
	if err != nil {
		return fmt.Errorf("render config: %w", err)
	}

	path := filepath.Join(dotDir, "docker.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

var configTemplate = template.Must(template.New("config").Parse(`# devctl configuration
# Docs: run 'devctl' and type :help config
#
# project   - display name (shown in TUI header)
# profiles  - list of named environments
#   name     - profile name (shown in sidebar)
#   compose  - compose files (passed as -f to docker compose)
#   services - (optional) only start these services on 'up'
#              if omitted, all services from compose files are started

project: {{ .Project }}

profiles:
{{- range .Profiles }}
  - name: {{ .Name }}
    compose:
    {{- range .Compose }}
      - {{ . }}
    {{- end }}
    {{- if .Services }}
    # limit which services to start (remove to start all)
    services:
    {{- range .Services }}
      - {{ . }}
    {{- end }}
    {{- end }}
{{ end -}}
`))

func renderConfig(cfg *Config) (string, error) {
	var b strings.Builder
	if err := configTemplate.Execute(&b, cfg); err != nil {
		return "", err
	}
	return b.String(), nil
}
