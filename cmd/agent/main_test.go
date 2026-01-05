package main

import (
	"testing"
)

func TestIsPathAllowed(t *testing.T) {
	tests := []struct {
		path         string
		allowedRoots []string
		expected     bool
	}{
		{"/app/file.txt", []string{"/app"}, true},
		{"/tmp/file.txt", []string{"/app"}, false},
		{"/app/../etc/passwd", []string{"/app"}, false},
		{"/app/sub/file.txt", []string{"/app"}, true},
		{"/app", []string{"/app"}, true},
	}

	for _, tt := range tests {
		result := isPathAllowed(tt.path, tt.allowedRoots)
		if result != tt.expected {
			t.Errorf("isPathAllowed(%s, %v) = %v; want %v", tt.path, tt.allowedRoots, result, tt.expected)
		}
	}
}
