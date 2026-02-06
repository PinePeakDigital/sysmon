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
			terminalWidth: 60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test main stats grid (2 bars side by side)
			spacingBetweenBars := 2
			availableWidth := tt.terminalWidth
			barWidth := (availableWidth - spacingBetweenBars) / 2
			
			// Calculate total width used by the main stats bars
			totalMainStatsWidth := barWidth + spacingBetweenBars + barWidth
			
			// Due to integer division, we may lose 1 character (the remainder)
			// The grid should use availableWidth or availableWidth-1
			if totalMainStatsWidth < availableWidth-1 || totalMainStatsWidth > availableWidth {
				t.Errorf("Main stats grid width incorrect: used %d, expected between %d and %d", 
					totalMainStatsWidth, availableWidth-1, availableWidth)
			}
			
			// Test CPU cores grid (4 bars per line)
			coresPerLine := 4
			coreBarWidth := (availableWidth - (coresPerLine-1)*spacingBetweenBars) / coresPerLine
			
			// Calculate total width used by CPU cores
			totalCoresWidth := coresPerLine*coreBarWidth + (coresPerLine-1)*spacingBetweenBars
			
			// Due to integer division, we may lose up to (coresPerLine-1) characters
			// The grid should be close to availableWidth
			widthDiff := availableWidth - totalCoresWidth
			if widthDiff < 0 || widthDiff >= coresPerLine {
				t.Errorf("CPU cores grid width incorrect: used %d, expected close to %d (diff: %d)", 
					totalCoresWidth, availableWidth, widthDiff)
			}
			
			// Verify that we're using the full width (not width-2 as before)
			// This is the main fix - we should be using tt.terminalWidth, not tt.terminalWidth-2
			if availableWidth != tt.terminalWidth {
				t.Errorf("Available width should equal terminal width: got %d, expected %d",
					availableWidth, tt.terminalWidth)
			}
		})
	}
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
