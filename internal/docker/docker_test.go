package docker

import (
	"testing"
)

func TestParsePS(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Service
	}{
		{
			name:     "empty output",
			input:    "",
			expected: nil,
		},
		{
			name:  "single service running",
			input: "postgres\tproject-postgres-1\tUp 2 hours\thealthy\t0.0.0.0:5432->5432/tcp\n",
			expected: []Service{
				{
					Name:      "postgres",
					Container: "project-postgres-1",
					Status:    "Up 2 hours",
					Health:    "healthy",
					Ports:     "0.0.0.0:5432->5432/tcp",
					RunTime:   "2h",
				},
			},
		},
		{
			name: "multiple services mixed status",
			input: "api\tproject-api-1\tUp 5 minutes\thealthy\t0.0.0.0:8080->80/tcp\n" +
				"redis\tproject-redis-1\tUp 5 minutes\t\t0.0.0.0:6379->6379/tcp\n" +
				"worker\tproject-worker-1\tExited (1) 2 minutes ago\t\t\n",
			expected: []Service{
				{Name: "api", Container: "project-api-1", Status: "Up 5 minutes", Health: "healthy", Ports: "0.0.0.0:8080->80/tcp", RunTime: "5m"},
				{Name: "redis", Container: "project-redis-1", Status: "Up 5 minutes", Health: "", Ports: "0.0.0.0:6379->6379/tcp", RunTime: "5m"},
				{Name: "worker", Container: "project-worker-1", Status: "Exited (1) 2 minutes ago", Health: "", Ports: ""},
			},
		},
		{
			name:  "service with unhealthy status",
			input: "db\tproject-db-1\tUp 10 seconds\tunhealthy\t0.0.0.0:3306->3306/tcp\n",
			expected: []Service{
				{Name: "db", Container: "project-db-1", Status: "Up 10 seconds", Health: "unhealthy", Ports: "0.0.0.0:3306->3306/tcp", RunTime: "10s"},
			},
		},
		{
			name:  "minimal output — only name and container",
			input: "svc\tcontainer-1\n",
			expected: []Service{
				{Name: "svc", Container: "container-1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePS(tt.input)
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d services, got %d", len(tt.expected), len(got))
			}
			for i := range got {
				if got[i].Name != tt.expected[i].Name {
					t.Errorf("[%d] Name: got %q, want %q", i, got[i].Name, tt.expected[i].Name)
				}
				if got[i].Container != tt.expected[i].Container {
					t.Errorf("[%d] Container: got %q, want %q", i, got[i].Container, tt.expected[i].Container)
				}
				if got[i].Status != tt.expected[i].Status {
					t.Errorf("[%d] Status: got %q, want %q", i, got[i].Status, tt.expected[i].Status)
				}
				if got[i].Health != tt.expected[i].Health {
					t.Errorf("[%d] Health: got %q, want %q", i, got[i].Health, tt.expected[i].Health)
				}
				if got[i].Ports != tt.expected[i].Ports {
					t.Errorf("[%d] Ports: got %q, want %q", i, got[i].Ports, tt.expected[i].Ports)
				}
			}
		})
	}
}

func TestServiceIsRunning(t *testing.T) {
	tests := []struct {
		status   string
		expected bool
	}{
		{"Up 2 hours", true},
		{"running", true},
		{"Exited (1) 2 minutes ago", false},
		{"created", false},
		{"", false},
	}
	for _, tt := range tests {
		svc := Service{Status: tt.status}
		if got := svc.IsRunning(); got != tt.expected {
			t.Errorf("IsRunning(%q) = %v, want %v", tt.status, got, tt.expected)
		}
	}
}

func TestServiceHealth(t *testing.T) {
	svc := Service{Health: "healthy"}
	if !svc.IsHealthy() {
		t.Error("expected IsHealthy() = true")
	}
	if !svc.HasHealthCheck() {
		t.Error("expected HasHealthCheck() = true")
	}

	svc2 := Service{Health: ""}
	if svc2.HasHealthCheck() {
		t.Error("expected HasHealthCheck() = false for empty health")
	}

	svc3 := Service{Health: "unhealthy"}
	if svc3.IsHealthy() {
		t.Error("expected IsHealthy() = false for unhealthy")
	}
}

func TestExtractError(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "only progress lines — fallback to full output",
			input:    "Container project-api-1  Starting\nContainer project-api-1  Started\n",
			expected: "Container project-api-1  Starting\nContainer project-api-1  Started\n",
		},
		{
			name:     "error mixed with progress",
			input:    "Container project-api-1  Starting\nError: port already in use\nContainer project-db-1  Running\n",
			expected: "Error: port already in use",
		},
		{
			name:     "pure error",
			input:    "no such service: foobar\n",
			expected: "no such service: foobar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractError(tt.input)
			if got != tt.expected {
				t.Errorf("extractError():\ngot:  %q\nwant: %q", got, tt.expected)
			}
		})
	}
}

func TestComposeArgs(t *testing.T) {
	tests := []struct {
		name     string
		opts     ComposeOpts
		expected []string
	}{
		{
			name:     "no project, no files",
			opts:     ComposeOpts{},
			expected: []string{"compose"},
		},
		{
			name:     "with project",
			opts:     ComposeOpts{Project: "myproject"},
			expected: []string{"compose", "-p", "myproject"},
		},
		{
			name:     "with files",
			opts:     ComposeOpts{Files: []string{"docker-compose.yml", "docker-compose.override.yml"}},
			expected: []string{"compose", "-f", "docker-compose.yml", "-f", "docker-compose.override.yml"},
		},
		{
			name:     "with project and files",
			opts:     ComposeOpts{Project: "proj", Files: []string{"a.yml"}},
			expected: []string{"compose", "-p", "proj", "-f", "a.yml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := composeArgs(tt.opts)
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %v, got %v", tt.expected, got)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("[%d] got %q, want %q", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestShortDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"30 seconds", "30s"},
		{"About a minute", "~1m"},
		{"3 minutes", "3m"},
		{"About an hour", "~1h"},
		{"2 hours", "2h"},
		{"1 day", "1d"},
		{"5 days", "5d"},
		{"1 minute", "1m"},
		{"1 hour", "1h"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := shortDuration(tt.input)
			if got != tt.expected {
				t.Errorf("shortDuration(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
