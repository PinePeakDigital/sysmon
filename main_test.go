package main

import (
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
