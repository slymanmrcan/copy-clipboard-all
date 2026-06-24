package main

import "testing"

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		limit    int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"merhaba dünya", 9, "merhaba d..."}, // 'ü' starts at index 8 (byte). Let's see what happens.
		{"🚀🚀🚀🚀🚀", 3, "🚀🚀🚀..."},
	}

	for _, tt := range tests {
		actual := truncate(tt.input, tt.limit)
		if actual != tt.expected {
			t.Errorf("truncate(%q, %d) = %q; expected %q", tt.input, tt.limit, actual, tt.expected)
		}
	}
}
