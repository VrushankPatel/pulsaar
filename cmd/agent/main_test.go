package main

import (
	"context"
	"net"
	"os"
	"testing"

	"github.com/VrushankPatel/pulsaar/internal/config"
	"golang.org/x/time/rate"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	api "github.com/VrushankPatel/pulsaar/api"
)

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
	// Test self-signed generation (no args)
	cert, err := loadOrGenerateCert("", "")
	if err != nil {
		t.Fatalf("failed to generate cert: %v", err)
	}
	if len(cert.Certificate) == 0 {
		t.Error("expected certificate")
	}
}

func TestLoadCACertPool(t *testing.T) {
	// Test no CA file
	pool, err := loadCACertPool("")
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
	if resp.Version != version {
		t.Errorf("expected Version to be %s, got %s", version, resp.Version)
	}
	if resp.StatusMessage != "Agent ready" {
		t.Errorf("expected StatusMessage to be 'Agent ready', got %s", resp.StatusMessage)
	}
}

func TestRateLimiting(t *testing.T) {
	// Create a context with a peer IP
	ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}})
	ip := "127.0.0.1"

	// Temporarily set a low limit for this IP
	limiters.Store(ip, rate.NewLimiter(rate.Limit(1), 1)) // 1 per second
	defer limiters.Delete(ip)

	// With default config (allowed roots = /)
	cfg := &config.AgentConfig{AllowedRoots: []string{"/"}}
	s := &server{config: cfg}

	// First call should succeed
	_, err := s.ListDirectory(ctx, &api.ListRequest{
		Path:         "/",
		AllowedRoots: []string{"/"},
	})
	if err != nil {
		t.Fatalf("First ListDirectory call failed: %v", err)
	}

	// Second call immediately should fail due to rate limit
	_, err = s.ListDirectory(ctx, &api.ListRequest{
		Path:         "/",
		AllowedRoots: []string{"/"},
	})
	if err == nil {
		t.Error("Expected rate limit error, but got none")
	}
	if status.Code(err) != codes.ResourceExhausted {
		t.Errorf("Expected ResourceExhausted, got %v", status.Code(err))
	}
}
