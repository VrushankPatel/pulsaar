package main

import (
	"os"
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

func TestAuditLog(t *testing.T) {
	// Test audit log without aggregator
	auditLog("TestOperation", "/test/path")

	// Test with invalid aggregator URL (should not panic)
	original := os.Getenv("PULSAAR_AUDIT_AGGREGATOR_URL")
	os.Setenv("PULSAAR_AUDIT_AGGREGATOR_URL", "http://invalid-url-that-will-fail")
	auditLog("TestOperation2", "/test/path2")
	os.Setenv("PULSAAR_AUDIT_AGGREGATOR_URL", original)
}
