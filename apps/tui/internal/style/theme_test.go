package style

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestTraceStatusChar(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{"success", "✓"},
		{"failed", "✗"},
		{"skipped", "○"},
		{"pending", "·"},
		{"unknown", "·"},
		{"", "·"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := TraceStatusChar(tt.status)
			if got != tt.expected {
				t.Errorf("TraceStatusChar(%q) = %q, want %q", tt.status, got, tt.expected)
			}
		})
	}
}

func TestTraceStatusStyle(t *testing.T) {
	tests := []struct {
		status    string
		wantColor lipgloss.Color
	}{
		{"success", BrightGreen},
		{"failed", Red},
		{"skipped", Gray},
		{"pending", DimGray},
		{"unknown", DimGray},
		{"", DimGray},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := TraceStatusStyle(tt.status)
			rendered := got.Render("test")
			if rendered == "" {
				t.Fatalf("TraceStatusStyle(%q) produced empty output", tt.status)
			}
		})
	}
}
