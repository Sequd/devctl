package ui

import (
	"testing"
)

func TestFormatPorts(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty",
			input:    "",
			expected: "",
		},
		{
			name:     "same host and container port",
			input:    "0.0.0.0:5432->5432/tcp",
			expected: ":5432",
		},
		{
			name:     "different ports",
			input:    "0.0.0.0:8080->80/tcp",
			expected: ":8080->80",
		},
		{
			name:     "multiple ports",
			input:    "0.0.0.0:5432->5432/tcp, 0.0.0.0:8080->80/tcp",
			expected: ":5432 :8080->80",
		},
		{
			name:     "ipv6 binding",
			input:    ":::5432->5432/tcp",
			expected: ":5432",
		},
		{
			name:     "no mapping (exposed only)",
			input:    "3306/tcp",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatPorts(tt.input)
			if got != tt.expected {
				t.Errorf("formatPorts(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
