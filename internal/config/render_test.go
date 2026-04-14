package config

import (
	"fmt"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRenderConfig(t *testing.T) {
	cfg := &Config{
		Project: "test",
		Profiles: []Profile{
			{Name: "dev", Compose: []string{"docker-compose.yml"}, Services: []string{"api", "redis"}},
			{Name: "full", Compose: []string{"docker-compose.yml", "docker-compose.prod.yml"}},
		},
	}
	out, err := renderConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(out)

	var parsed Config
	if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatal("re-parse failed:", err)
	}

	if parsed.Project != "test" {
		t.Fatalf("project: got %q, want %q", parsed.Project, "test")
	}
	if len(parsed.Profiles) != 2 {
		t.Fatalf("profiles: got %d, want 2", len(parsed.Profiles))
	}
	if len(parsed.Profiles[0].Services) != 2 {
		t.Fatalf("services: got %d, want 2", len(parsed.Profiles[0].Services))
	}
}
