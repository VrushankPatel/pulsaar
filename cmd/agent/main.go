package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	api "github.com/VrushankPatel/pulsaar/api"
)

type server struct {
	api.UnimplementedPulsaarAgentServer
}

const maxReadSize int64 = 1024 * 1024 // 1MB

var limiter = rate.NewLimiter(rate.Limit(10), 10) // 10 operations per second

func loadOrGenerateCert() (tls.Certificate, error) {
	certFile := os.Getenv("PULSAAR_TLS_CERT_FILE")
	keyFile := os.Getenv("PULSAAR_TLS_KEY_FILE")

	if certFile != "" && keyFile != "" {
		return tls.LoadX509KeyPair(certFile, keyFile)
	}

	// Fallback to self-signed for MVP
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Pulsaar MVP"},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(time.Hour * 24 * 365),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:    []string{"localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  priv,
	}, nil
}

func loadCACertPool() (*x509.CertPool, error) {
	caFile := os.Getenv("PULSAAR_TLS_CA_FILE")
	if caFile == "" {
		return nil, nil // No client cert verification
	}

	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	return caCertPool, nil
}

func isPathAllowed(path string, allowedRoots []string) bool {
	cleanPath := filepath.Clean(path)
	for _, root := range allowedRoots {
		cleanRoot := filepath.Clean(root)
		if cleanPath == cleanRoot || strings.HasPrefix(cleanPath, cleanRoot+"/") {
			return true
		}
	}
	return false
}

func auditLog(operation, path string) {
	log.Printf("Audit: %s request for path: %s", operation, path)
	if url := os.Getenv("PULSAAR_AUDIT_AGGREGATOR_URL"); url != "" {
		hostname, _ := os.Hostname()
		data := map[string]any{
			"timestamp": time.Now().Format(time.RFC3339),
			"operation": operation,
			"path":      path,
			"agent_id":  hostname,
		}
		jsonData, _ := json.Marshal(data)
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
		if resp != nil {
			defer func() { _ = resp.Body.Close() }()
		}
		if err != nil {
			log.Printf("Failed to send audit log: %v", err)
		}
	}
}

func (s *server) ListDirectory(ctx context.Context, req *api.ListRequest) (*api.ListResponse, error) {
	if !limiter.Allow() {
		return nil, status.Errorf(codes.ResourceExhausted, "rate limit exceeded")
	}
	auditLog("ListDirectory", req.Path)
	if !isPathAllowed(req.Path, req.AllowedRoots) {
		return nil, status.Errorf(codes.PermissionDenied, "path not allowed")
	}

	entries, err := os.ReadDir(req.Path)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read directory: %v", err)
	}

	var fileInfos []*api.FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		fileInfos = append(fileInfos, &api.FileInfo{
			Name:      entry.Name(),
			IsDir:     entry.IsDir(),
			SizeBytes: info.Size(),
			Mode:      info.Mode().String(),
			Mtime:     timestamppb.New(info.ModTime()),
		})
	}

	return &api.ListResponse{Entries: fileInfos}, nil
}

func (s *server) Stat(ctx context.Context, req *api.StatRequest) (*api.StatResponse, error) {
	if !limiter.Allow() {
		return nil, status.Errorf(codes.ResourceExhausted, "rate limit exceeded")
	}
	auditLog("Stat", req.Path)
	if !isPathAllowed(req.Path, req.AllowedRoots) {
		return nil, status.Errorf(codes.PermissionDenied, "path not allowed")
	}

	info, err := os.Stat(req.Path)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to stat file: %v", err)
	}

	return &api.StatResponse{
		Info: &api.FileInfo{
			Name:      filepath.Base(req.Path),
			IsDir:     info.IsDir(),
			SizeBytes: info.Size(),
			Mode:      info.Mode().String(),
			Mtime:     timestamppb.New(info.ModTime()),
		},
	}, nil
}

func (s *server) ReadFile(ctx context.Context, req *api.ReadRequest) (*api.ReadResponse, error) {
	if !limiter.Allow() {
		return nil, status.Errorf(codes.ResourceExhausted, "rate limit exceeded")
	}
	auditLog("ReadFile", req.Path)
	if !isPathAllowed(req.Path, req.AllowedRoots) {
		return nil, status.Errorf(codes.PermissionDenied, "path not allowed")
	}

	readLen := req.Length
	if readLen == 0 {
		readLen = maxReadSize
	}
	if readLen > maxReadSize {
		return nil, status.Errorf(codes.InvalidArgument, "read length exceeds limit")
	}

	file, err := os.Open(req.Path)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to open file: %v", err)
	}
	defer func() { _ = file.Close() }()

	data := make([]byte, readLen)
	n, err := file.ReadAt(data, req.Offset)
	if err != nil && err != io.EOF {
		return nil, status.Errorf(codes.Internal, "failed to read file: %v", err)
	}

	eof := int64(n) < readLen || err == io.EOF
	return &api.ReadResponse{Data: data[:n], Eof: eof}, nil
}

func (s *server) StreamFile(req *api.StreamRequest, stream api.PulsaarAgent_StreamFileServer) error {
	if !limiter.Allow() {
		return status.Errorf(codes.ResourceExhausted, "rate limit exceeded")
	}
	auditLog("StreamFile", req.Path)
	if !isPathAllowed(req.Path, req.AllowedRoots) {
		return status.Errorf(codes.PermissionDenied, "path not allowed")
	}

	chunkSize := req.ChunkSize
	if chunkSize == 0 {
		chunkSize = 64 * 1024 // 64KB default
	}
	if chunkSize > maxReadSize {
		return status.Errorf(codes.InvalidArgument, "chunk size exceeds limit")
	}

	file, err := os.Open(req.Path)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to open file: %v", err)
	}
	defer func() { _ = file.Close() }()

	buf := make([]byte, chunkSize)
	for {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return status.Errorf(codes.Internal, "failed to read file: %v", err)
		}
		if n == 0 {
			break
		}
		eof := err == io.EOF
		if err := stream.Send(&api.ReadResponse{Data: buf[:n], Eof: eof}); err != nil {
			return err
		}
		if eof {
			break
		}
	}
	return nil
}

func (s *server) Health(ctx context.Context, req *emptypb.Empty) (*api.HealthResponse, error) {
	return &api.HealthResponse{
		Ready:         true,
		Version:       "v1.0.0",
		StatusMessage: "Agent ready",
	}, nil
}

func main() {
	cert, err := loadOrGenerateCert()
	if err != nil {
		log.Fatalf("failed to load or generate cert: %v", err)
	}

	caCertPool, err := loadCACertPool()
	if err != nil {
		log.Fatalf("failed to load CA cert pool: %v", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	if caCertPool != nil {
		tlsConfig.ClientCAs = caCertPool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	creds := credentials.NewTLS(tlsConfig)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer(
		grpc.Creds(creds),
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
	)
	api.RegisterPulsaarAgentServer(s, &server{})
	grpc_prometheus.Register(s)

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Printf("Metrics server listening on :9090")
		log.Fatal(http.ListenAndServe(":9090", nil))
	}()

	log.Printf("Pulsaar agent listening on :50051 with TLS")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
