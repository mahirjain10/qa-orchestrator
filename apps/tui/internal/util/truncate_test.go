package util

import (
	"testing"
	"time"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"short string, no truncation", "hello", 10, "hello"},
		{"exact length, no truncation", "hello", 5, "hello"},
		{"truncate at boundary", "hello world", 8, "hello..."},
		{"maxLen 3", "hello", 3, "hel"},
		{"maxLen 2", "hello", 2, "he"},
		{"maxLen 1", "hello", 1, "h"},
		{"maxLen 0", "hello", 0, ""},
		{"empty string", "", 5, ""},
		{"unicode string", "héllo wörld", 8, "héll..."},
		{"long string", "this is a very long string that needs truncation", 20, "this is a very lo..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Truncate(tt.input, tt.maxLen)
			if got != tt.expected {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.expected)
			}
			if len(got) > tt.maxLen {
				t.Errorf("Truncate(%q, %d) returned length %d, exceeds maxLen %d", tt.input, tt.maxLen, len(got), tt.maxLen)
			}
		})
	}
}

func TestTruncateMiddle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"short string, no truncation", "hello", 10, "hello"},
		{"exact length, no truncation", "hello", 5, "hello"},
		{"truncate middle", "hello world foo bar", 16, "hello ...foo bar"},
		{"maxLen 7", "hello world", 7, "hello w"},
		{"maxLen 6", "hello world", 6, "hello "},
		{"empty string", "", 5, ""},
		{"uuid-like string", "abc123-def456-ghi789-jkl012", 20, "abc123-d...89-jkl012"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateMiddle(tt.input, tt.maxLen)
			if got != tt.expected {
				t.Errorf("TruncateMiddle(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.expected)
			}
			if len(got) > tt.maxLen {
				t.Errorf("TruncateMiddle(%q, %d) returned length %d, exceeds maxLen %d", tt.input, tt.maxLen, len(got), tt.maxLen)
			}
		})
	}
}

func TestTruncateStart(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"short string, no truncation", "hello", 10, "hello"},
		{"exact length, no truncation", "hello", 5, "hello"},
		{"truncate start of path", "/very/long/path/to/some/file.txt", 20, ".../to/some/file.txt"},
		{"maxLen 3", "hello", 3, "llo"},
		{"maxLen 2", "hello", 2, "lo"},
		{"empty string", "", 5, ""},
		{"long path", "/home/user/projects/myapp/src/components/Button.tsx", 30, "...p/src/components/Button.tsx"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateStart(tt.input, tt.maxLen)
			if got != tt.expected {
				t.Errorf("TruncateStart(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.expected)
			}
			if len(got) > tt.maxLen {
				t.Errorf("TruncateStart(%q, %d) returned length %d, exceeds maxLen %d", tt.input, tt.maxLen, len(got), tt.maxLen)
			}
		})
	}
}

func TestSafeWidth(t *testing.T) {
	tests := []struct {
		name     string
		width    int
		min      int
		expected int
	}{
		{"width above min", 100, 30, 100},
		{"width equals min", 30, 30, 30},
		{"width below min", 20, 30, 30},
		{"width zero", 0, 30, 30},
		{"width negative", -10, 30, 30},
		{"min zero", 50, 0, 50},
		{"both zero", 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SafeWidth(tt.width, tt.min)
			if got != tt.expected {
				t.Errorf("SafeWidth(%d, %d) = %d, want %d", tt.width, tt.min, got, tt.expected)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Duration
		expected string
	}{
		{"zero", 0, "0s"},
		{"milliseconds", 500 * time.Millisecond, "500ms"},
		{"seconds", 5 * time.Second, "5s"},
		{"minutes", 2*time.Minute + 30*time.Second, "2m30s"},
		{"hours", 1*time.Hour + 30*time.Minute, "1h30m0s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDuration(tt.input)
			if got != tt.expected {
				t.Errorf("FormatDuration(%v) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFormatDurationShort(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Duration
		expected string
	}{
		{"zero", 0, "0s"},
		{"5 seconds", 5 * time.Second, "5s"},
		{"30 seconds", 30 * time.Second, "30s"},
		{"1 minute", 1 * time.Minute, "01m"},
		{"1 minute 5 seconds", 65 * time.Second, "01m05s"},
		{"2 minutes 30 seconds", 150 * time.Second, "02m30s"},
		{"1 hour", 3600 * time.Second, "60m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDurationShort(tt.input)
			if got != tt.expected {
				t.Errorf("FormatDurationShort(%v) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFormatInt(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected string
	}{
		{"zero", 0, "00"},
		{"single digit", 5, "05"},
		{"two digits", 30, "30"},
		{"large number", 123, "123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatInt(tt.input)
			if got != tt.expected {
				t.Errorf("formatInt(%d) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
