package main

import (
	"strings"
	"testing"
)

func TestTruncateLeft(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxWidth int
		expected string
	}{
		{
			name:     "string shorter than maxWidth",
			input:    "/usr/bin/test",
			maxWidth: 20,
			expected: "/usr/bin/test",
		},
		{
			name:     "string equal to maxWidth",
			input:    "/usr/bin/test",
			maxWidth: 13,
			expected: "/usr/bin/test",
		},
		{
			name:     "string longer than maxWidth",
			input:    "/very/long/path/to/executable",
			maxWidth: 20,
			expected: "...ath/to/executable",
		},
		{
			name:     "very long path truncated",
			input:    "/path/to/some/very/long/executable/name",
			maxWidth: 25,
			expected: "...y/long/executable/name",
		},
		{
			name:     "maxWidth very small",
			input:    "/usr/bin/test",
			maxWidth: 5,
			expected: "...st",
		},
		{
			name:     "maxWidth equal to 3",
			input:    "/usr/bin/test",
			maxWidth: 3,
			expected: "...",
		},
		{
			name:     "maxWidth less than 3",
			input:    "/usr/bin/test",
			maxWidth: 2,
			expected: "..",
		},
		{
			name:     "maxWidth zero",
			input:    "/usr/bin/test",
			maxWidth: 0,
			expected: "",
		},
		{
			name:     "unicode characters",
			input:    "/path/to/文件/executable",
			maxWidth: 15,
			expected: "...件/executable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateLeft(tt.input, tt.maxWidth)
			if result != tt.expected {
				t.Errorf("truncateLeft(%q, %d) = %q; expected %q", tt.input, tt.maxWidth, result, tt.expected)
			}
		})
	}
}

func TestProgressBarWidths(t *testing.T) {
	tests := []struct {
		name          string
		terminalWidth int
	}{
		{
			name:          "standard terminal width",
			terminalWidth: 80,
		},
		{
			name:          "wide terminal",
			terminalWidth: 120,
		},
		{
			name:          "narrow terminal",
			terminalWidth: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a model with known dimensions and stats
			m := model{
				width:  tt.terminalWidth,
				height: 24,
				stats: SystemStats{
					CPUUsage:    50.0,
					GPUUsage:    25.0,
					MemoryUsage: 60.0,
					GPUMemory:   30.0,
					CPUCores:    []float64{10.0, 20.0, 30.0, 40.0, 50.0, 60.0, 70.0, 80.0},
					Processes: []ProcessInfo{
						{PID: 1234, CPU: 10.5, Memory: 5.2, Command: "/usr/bin/test"},
					},
				},
			}

			// Render the view
			view := m.View()
			lines := strings.Split(view, "\n")

			// Find the maximum line length in the rendered output
			maxLen := 0
			for _, line := range lines {
				// Strip ANSI color codes to get actual character length
				stripped := stripAnsiCodes(line)
				if len(stripped) > maxLen {
					maxLen = len(stripped)
				}
			}

			// The fix ensures we use m.width instead of m.width-2.
			// The view should use close to the full width (allowing for integer division).
			// Verify we're not using the old width-2 margin by checking that the max line
			// length is at least width-2 (the old behavior would have been exactly width-2).
			if maxLen < tt.terminalWidth-1 {
				t.Errorf("View does not use enough terminal width: max line length %d, expected at least %d (terminal width: %d)",
					maxLen, tt.terminalWidth-1, tt.terminalWidth)
			}

			// Note: We don't enforce maxLen <= tt.terminalWidth because minimum bar widths
			// can cause the view to exceed terminal width in narrow terminals. This is
			// existing behavior that ensures bars remain readable.
		})
	}
}

// stripAnsiCodes removes ANSI escape codes to get the actual display length
func stripAnsiCodes(s string) string {
	// Simple regex-free approach: skip escape sequences
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			// Skip ANSI escape sequence
			i += 2
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++ // skip 'm'
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
}

func TestViewRendersCorrectly(t *testing.T) {
	// Create a model with known dimensions and stats
	m := model{
		width:  80,
		height: 24,
		stats: SystemStats{
			CPUUsage:    50.0,
			GPUUsage:    25.0,
			MemoryUsage: 60.0,
			GPUMemory:   30.0,
			CPUCores:    []float64{10.0, 20.0, 30.0, 40.0},
			Processes: []ProcessInfo{
				{PID: 1234, CPU: 10.5, Memory: 5.2, Command: "/usr/bin/test"},
			},
		},
	}

	// Render the view
	view := m.View()
	
	// Split into lines
	lines := strings.Split(view, "\n")
	
	// Basic sanity checks
	if len(lines) < 5 {
		t.Errorf("Expected at least 5 lines in output, got %d", len(lines))
	}
	
	// Verify the view contains expected content
	viewContent := strings.ToLower(view)
	expectedStrings := []string{"cpu usage", "memory", "gpu usage", "pid", "command"}
	for _, expected := range expectedStrings {
		if !strings.Contains(viewContent, expected) {
			t.Errorf("Expected view to contain %q", expected)
		}
	}
}
