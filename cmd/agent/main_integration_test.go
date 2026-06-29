package main

import (
	"context"
	"crypto/tls"
	"net"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	api "github.com/VrushankPatel/pulsaar/api"
	"github.com/VrushankPatel/pulsaar/internal/config"
)

func TestIntegrationAgentGRPCTLS(t *testing.T) {
	// 1. Setup config using system temp dir
	cfg := &config.AgentConfig{
		AllowedRoots:       []string{os.TempDir()},
		RateLimitOpsPerSec: 100,
	}

	// 2. Load certs using consolidated config methods
	cert, err := cfg.LoadTLSCertificate()
	if err != nil {
		t.Fatalf("failed to load/generate TLS cert: %v", err)
	}

	// 3. Create gRPC Server with TLS
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	creds := credentials.NewTLS(tlsConfig)
	s := grpc.NewServer(grpc.Creds(creds))
	agentServer := &server{config: cfg}
	api.RegisterPulsaarAgentServer(s, agentServer)

	// Listen on random loopback port
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := lis.Addr().String()

	go func() {
		_ = s.Serve(lis)
	}()
	defer s.GracefulStop()

	// Wait for server to bind
	time.Sleep(100 * time.Millisecond)

	// 4. Create gRPC Client with TLS
	// InsecureSkipVerify is required because the server uses a generated self-signed cert
	clientTLSConfig := &tls.Config{
		InsecureSkipVerify: true,
	}
	clientCreds := credentials.NewTLS(clientTLSConfig)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(clientCreds))
	if err != nil {
		t.Fatalf("failed to connect to server: %v", err)
	}
	defer conn.Close()

	client := api.NewPulsaarAgentClient(conn)

	// 5. Perform ListDirectory test
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.ListDirectory(ctx, &api.ListRequest{
		Path: os.TempDir(),
	})
	if err != nil {
		t.Fatalf("failed ListDirectory gRPC call: %v", err)
	}

	if resp == nil {
		t.Error("expected non-nil response")
	}
}
