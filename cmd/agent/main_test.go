package main

import (
	"context"
	"os"
	"testing"

	"google.golang.org/protobuf/types/known/emptypb"
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
		{"/appfile", []string{"/app"}, false},
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
	if err := os.Setenv("PULSAAR_AUDIT_AGGREGATOR_URL", "http://invalid-url-that-will-fail"); err != nil {
		t.Fatalf("failed to set env: %v", err)
	}
	auditLog("TestOperation2", "/test/path2")
	if err := os.Setenv("PULSAAR_AUDIT_AGGREGATOR_URL", original); err != nil {
		t.Fatalf("failed to restore env: %v", err)
	}
}

func TestLoadOrGenerateCert(t *testing.T) {
	// Test self-signed generation (no env)
	cert, err := loadOrGenerateCert()
	if err != nil {
		t.Fatalf("failed to generate cert: %v", err)
	}
	if len(cert.Certificate) == 0 {
		t.Error("expected certificate")
	}
}

func TestLoadCACertPool(t *testing.T) {
	// Test no CA file
	pool, err := loadCACertPool()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pool != nil {
		t.Error("expected nil pool when no CA file")
	}
}

func TestHealth(t *testing.T) {
	s := &server{}
	resp, err := s.Health(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("Health returned error: %v", err)
	}
	if !resp.Ready {
		t.Error("expected Ready to be true")
	}
	if resp.Version != "v1.0.0" {
		t.Errorf("expected Version to be v1.0.0, got %s", resp.Version)
	}
	if resp.StatusMessage != "Agent ready" {
		t.Errorf("expected StatusMessage to be 'Agent ready', got %s", resp.StatusMessage)
	}
}
