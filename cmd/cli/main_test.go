package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	api "github.com/VrushankPatel/pulsaar/api"
)

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

func generateSelfSignedCert() (tls.Certificate, error) {
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

type server struct {
	api.UnimplementedPulsaarAgentServer
}

const maxReadSize int64 = 1024 * 1024 // 1MB

func isPathAllowed(path string, allowedRoots []string) bool {
	cleanPath := filepath.Clean(path)
	for _, root := range allowedRoots {
		if strings.HasPrefix(cleanPath, root) {
			return true
		}
	}
	return false
}

func (s *server) ListDirectory(ctx context.Context, req *api.ListRequest) (*api.ListResponse, error) {
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

func (s *server) ReadFile(ctx context.Context, req *api.ReadRequest) (*api.ReadResponse, error) {
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

func (s *server) Stat(ctx context.Context, req *api.StatRequest) (*api.StatResponse, error) {
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

func (s *server) Health(ctx context.Context, req *emptypb.Empty) (*api.HealthResponse, error) {
	return &api.HealthResponse{
		Ready:         true,
		Version:       version,
		StatusMessage: "Agent ready",
	}, nil
}

func TestEndToEnd(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "pulsaar_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to remove temp dir: %v", err)
		}
	}()

	// Create some files
	err = os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("content1"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(tempDir, "file2.txt"), []byte("content2"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Start server
	cert, err := generateSelfSignedCert()
	if err != nil {
		t.Fatal(err)
	}

	creds := credentials.NewServerTLSFromCert(&cert)

	lis, err := net.Listen("tcp", ":0") // free port
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lis.Close() }()

	port := lis.Addr().(*net.TCPAddr).Port

	s := grpc.NewServer(grpc.Creds(creds))
	api.RegisterPulsaarAgentServer(s, &server{})

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Logf("server error: %v", err)
		}
	}()
	defer s.Stop()

	time.Sleep(100 * time.Millisecond) // wait for server

	// Connect client
	conn, err := grpc.NewClient(fmt.Sprintf("localhost:%d", port), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	client := api.NewPulsaarAgentClient(conn)

	// Call ListDirectory
	resp, err := client.ListDirectory(context.Background(), &api.ListRequest{
		Path:         tempDir,
		AllowedRoots: []string{tempDir},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	if len(resp.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(resp.Entries))
	}

	names := make(map[string]bool)
	for _, entry := range resp.Entries {
		names[entry.Name] = true
	}
	if !names["file1.txt"] || !names["file2.txt"] {
		t.Errorf("expected file1.txt and file2.txt, got %v", names)
	}
}

func TestReadEndToEnd(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "pulsaar_read_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to remove temp dir: %v", err)
		}
	}()

	// Create a file with content
	content := "Hello, this is test content for reading."
	err = os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Start server
	cert, err := generateSelfSignedCert()
	if err != nil {
		t.Fatal(err)
	}

	creds := credentials.NewServerTLSFromCert(&cert)

	lis, err := net.Listen("tcp", ":0") // free port
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lis.Close() }()

	port := lis.Addr().(*net.TCPAddr).Port

	s := grpc.NewServer(grpc.Creds(creds))
	api.RegisterPulsaarAgentServer(s, &server{})

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Logf("server error: %v", err)
		}
	}()
	defer s.Stop()

	time.Sleep(100 * time.Millisecond) // wait for server

	// Connect client
	conn, err := grpc.NewClient(fmt.Sprintf("localhost:%d", port), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	client := api.NewPulsaarAgentClient(conn)

	// Call ReadFile
	resp, err := client.ReadFile(context.Background(), &api.ReadRequest{
		Path:         filepath.Join(tempDir, "test.txt"),
		Offset:       0,
		Length:       0,
		AllowedRoots: []string{tempDir},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	if string(resp.Data) != content {
		t.Errorf("expected content %q, got %q", content, string(resp.Data))
	}
	if !resp.Eof {
		t.Errorf("expected EOF true, got false")
	}
}

func TestStreamEndToEnd(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "pulsaar_stream_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to remove temp dir: %v", err)
		}
	}()

	// Create a file with content larger than chunk size
	content := "Hello, this is test content for streaming. " + strings.Repeat("More content. ", 100)
	err = os.WriteFile(filepath.Join(tempDir, "stream.txt"), []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Start server
	cert, err := generateSelfSignedCert()
	if err != nil {
		t.Fatal(err)
	}

	creds := credentials.NewServerTLSFromCert(&cert)

	lis, err := net.Listen("tcp", ":0") // free port
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lis.Close() }()

	port := lis.Addr().(*net.TCPAddr).Port

	s := grpc.NewServer(grpc.Creds(creds))
	api.RegisterPulsaarAgentServer(s, &server{})

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Logf("server error: %v", err)
		}
	}()
	defer s.Stop()

	time.Sleep(100 * time.Millisecond) // wait for server

	// Connect client
	conn, err := grpc.NewClient(fmt.Sprintf("localhost:%d", port), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	client := api.NewPulsaarAgentClient(conn)

	// Call StreamFile
	stream, err := client.StreamFile(context.Background(), &api.StreamRequest{
		Path:         filepath.Join(tempDir, "stream.txt"),
		ChunkSize:    64 * 1024, // 64KB
		AllowedRoots: []string{tempDir},
	})
	if err != nil {
		t.Fatal(err)
	}

	var receivedData []byte
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		receivedData = append(receivedData, resp.Data...)
	}

	// Assert
	if string(receivedData) != content {
		t.Errorf("expected content length %d, got %d", len(content), len(receivedData))
	}
}

func TestStatEndToEnd(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "pulsaar_stat_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to remove temp dir: %v", err)
		}
	}()

	// Create a file
	err = os.WriteFile(filepath.Join(tempDir, "stat.txt"), []byte("stat content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Start server
	cert, err := generateSelfSignedCert()
	if err != nil {
		t.Fatal(err)
	}

	creds := credentials.NewServerTLSFromCert(&cert)

	lis, err := net.Listen("tcp", ":0") // free port
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lis.Close() }()

	port := lis.Addr().(*net.TCPAddr).Port

	s := grpc.NewServer(grpc.Creds(creds))
	api.RegisterPulsaarAgentServer(s, &server{})

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Logf("server error: %v", err)
		}
	}()
	defer s.Stop()

	time.Sleep(100 * time.Millisecond) // wait for server

	// Connect client
	conn, err := grpc.NewClient(fmt.Sprintf("localhost:%d", port), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	client := api.NewPulsaarAgentClient(conn)

	// Call Stat
	resp, err := client.Stat(context.Background(), &api.StatRequest{
		Path:         filepath.Join(tempDir, "stat.txt"),
		AllowedRoots: []string{tempDir},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	if resp.Info.Name != "stat.txt" {
		t.Errorf("expected name 'stat.txt', got %s", resp.Info.Name)
	}
	if resp.Info.IsDir {
		t.Errorf("expected IsDir false, got true")
	}
	if resp.Info.SizeBytes != 12 {
		t.Errorf("expected size 12, got %d", resp.Info.SizeBytes)
	}
}

func TestCreateTLSConfig(t *testing.T) {
	// Test default config
	config, err := createTLSConfig()
	if err != nil {
		t.Fatalf("failed to create config: %v", err)
	}
	if !config.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify true by default")
	}
}

func generateCACert() (tls.Certificate, *x509.Certificate, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Pulsaar Test CA"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour * 24 * 365),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, nil, err
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return tls.Certificate{}, nil, err
	}

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  priv,
	}, cert, nil
}

func generateSignedCert(caCert *x509.Certificate, caKey interface{}, commonName string, dnsNames []string) (tls.Certificate, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization: []string{"Pulsaar Test"},
			CommonName:   commonName,
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(time.Hour * 24 * 365),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:    dnsNames,
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, caCert, &priv.PublicKey, caKey)
	if err != nil {
		return tls.Certificate{}, err
	}

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  priv,
	}, nil
}

func TestMTLSCertificateLoading(t *testing.T) {
	// Generate CA
	caCert, caX509, err := generateCACert()
	if err != nil {
		t.Fatal(err)
	}

	// Generate server cert
	serverCert, err := generateSignedCert(caX509, caCert.PrivateKey, "localhost", []string{"localhost"})
	if err != nil {
		t.Fatal(err)
	}

	// Generate client cert
	clientCert, err := generateSignedCert(caX509, caCert.PrivateKey, "client", []string{})
	if err != nil {
		t.Fatal(err)
	}

	// Create temp dir for cert files
	tempDir, err := os.MkdirTemp("", "pulsaar_mtls_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to remove temp dir: %v", err)
		}
	}()

	// Write certs to files
	caCertFile := filepath.Join(tempDir, "ca.crt")
	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Certificate[0]})
	err = os.WriteFile(caCertFile, caCertPEM, 0644)
	if err != nil {
		t.Fatal(err)
	}

	serverCertFile := filepath.Join(tempDir, "server.crt")
	serverCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverCert.Certificate[0]})
	serverKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverCert.PrivateKey.(*rsa.PrivateKey))})
	serverCertData := append(serverCertPEM, serverKeyPEM...)
	err = os.WriteFile(serverCertFile, serverCertData, 0644)
	if err != nil {
		t.Fatal(err)
	}

	serverKeyFile := filepath.Join(tempDir, "server.key")
	err = os.WriteFile(serverKeyFile, serverKeyPEM, 0644)
	if err != nil {
		t.Fatal(err)
	}

	clientCertFile := filepath.Join(tempDir, "client.crt")
	clientCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientCert.Certificate[0]})
	clientKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientCert.PrivateKey.(*rsa.PrivateKey))})
	clientCertData := append(clientCertPEM, clientKeyPEM...)
	err = os.WriteFile(clientCertFile, clientCertData, 0644)
	if err != nil {
		t.Fatal(err)
	}

	clientKeyFile := filepath.Join(tempDir, "client.key")
	err = os.WriteFile(clientKeyFile, clientKeyPEM, 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Test agent cert loading
	originalCertFile := os.Getenv("PULSAAR_TLS_CERT_FILE")
	originalKeyFile := os.Getenv("PULSAAR_TLS_KEY_FILE")
	originalCAFile := os.Getenv("PULSAAR_TLS_CA_FILE")
	defer func() {
		_ = os.Setenv("PULSAAR_TLS_CERT_FILE", originalCertFile)
		_ = os.Setenv("PULSAAR_TLS_KEY_FILE", originalKeyFile)
		_ = os.Setenv("PULSAAR_TLS_CA_FILE", originalCAFile)
	}()

	_ = os.Setenv("PULSAAR_TLS_CERT_FILE", serverCertFile)
	_ = os.Setenv("PULSAAR_TLS_KEY_FILE", serverKeyFile)
	_ = os.Setenv("PULSAAR_TLS_CA_FILE", caCertFile)

	loadedCert, err := loadOrGenerateCert()
	if err != nil {
		t.Fatalf("failed to load cert: %v", err)
	}
	if len(loadedCert.Certificate) == 0 {
		t.Error("expected loaded certificate")
	}

	caPool, err := loadCACertPool()
	if err != nil {
		t.Fatalf("failed to load CA pool: %v", err)
	}
	if caPool == nil {
		t.Error("expected CA pool")
	}

	// Test CLI TLS config creation
	originalClientCertFile := os.Getenv("PULSAAR_CLIENT_CERT_FILE")
	originalClientKeyFile := os.Getenv("PULSAAR_CLIENT_KEY_FILE")
	originalCLI_CAFile := os.Getenv("PULSAAR_CA_FILE")
	defer func() {
		_ = os.Setenv("PULSAAR_CLIENT_CERT_FILE", originalClientCertFile)
		_ = os.Setenv("PULSAAR_CLIENT_KEY_FILE", originalClientKeyFile)
		_ = os.Setenv("PULSAAR_CA_FILE", originalCLI_CAFile)
	}()

	_ = os.Setenv("PULSAAR_CLIENT_CERT_FILE", clientCertFile)
	_ = os.Setenv("PULSAAR_CLIENT_KEY_FILE", clientKeyFile)
	_ = os.Setenv("PULSAAR_CA_FILE", caCertFile)

	cliConfig, err := createTLSConfig()
	if err != nil {
		t.Fatalf("failed to create CLI TLS config: %v", err)
	}
	if cliConfig.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify false with certs")
	}
	if len(cliConfig.Certificates) == 0 {
		t.Error("expected client certificates")
	}
	if cliConfig.RootCAs == nil {
		t.Error("expected root CAs")
	}
}

func TestMTLSEndToEnd(t *testing.T) {
	// Generate CA
	caCert, caX509, err := generateCACert()
	if err != nil {
		t.Fatal(err)
	}

	// Generate server cert
	serverCert, err := generateSignedCert(caX509, caCert.PrivateKey, "localhost", []string{"localhost"})
	if err != nil {
		t.Fatal(err)
	}

	// Generate client cert
	clientCert, err := generateSignedCert(caX509, caCert.PrivateKey, "client", []string{})
	if err != nil {
		t.Fatal(err)
	}

	// Create temp dir for cert files
	tempDir, err := os.MkdirTemp("", "pulsaar_mtls_e2e_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to remove temp dir: %v", err)
		}
	}()

	// Write certs to files
	caCertFile := filepath.Join(tempDir, "ca.crt")
	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Certificate[0]})
	err = os.WriteFile(caCertFile, caCertPEM, 0644)
	if err != nil {
		t.Fatal(err)
	}

	serverCertFile := filepath.Join(tempDir, "server.crt")
	serverCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverCert.Certificate[0]})
	serverKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverCert.PrivateKey.(*rsa.PrivateKey))})
	serverCertData := append(serverCertPEM, serverKeyPEM...)
	err = os.WriteFile(serverCertFile, serverCertData, 0644)
	if err != nil {
		t.Fatal(err)
	}

	serverKeyFile := filepath.Join(tempDir, "server.key")
	err = os.WriteFile(serverKeyFile, serverKeyPEM, 0644)
	if err != nil {
		t.Fatal(err)
	}

	clientCertFile := filepath.Join(tempDir, "client.crt")
	clientCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientCert.Certificate[0]})
	clientKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientCert.PrivateKey.(*rsa.PrivateKey))})
	clientCertData := append(clientCertPEM, clientKeyPEM...)
	err = os.WriteFile(clientCertFile, clientCertData, 0644)
	if err != nil {
		t.Fatal(err)
	}

	clientKeyFile := filepath.Join(tempDir, "client.key")
	err = os.WriteFile(clientKeyFile, clientKeyPEM, 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Set env vars for agent
	originalCertFile := os.Getenv("PULSAAR_TLS_CERT_FILE")
	originalKeyFile := os.Getenv("PULSAAR_TLS_KEY_FILE")
	originalCAFile := os.Getenv("PULSAAR_TLS_CA_FILE")
	_ = os.Setenv("PULSAAR_TLS_CERT_FILE", serverCertFile)
	_ = os.Setenv("PULSAAR_TLS_KEY_FILE", serverKeyFile)
	_ = os.Setenv("PULSAAR_TLS_CA_FILE", caCertFile)
	defer func() {
		_ = os.Setenv("PULSAAR_TLS_CERT_FILE", originalCertFile)
		_ = os.Setenv("PULSAAR_TLS_KEY_FILE", originalKeyFile)
		_ = os.Setenv("PULSAAR_TLS_CA_FILE", originalCAFile)
	}()

	// Start server
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lis.Close() }()

	port := lis.Addr().(*net.TCPAddr).Port

	cert, err := loadOrGenerateCert()
	if err != nil {
		t.Fatal(err)
	}

	caCertPool, err := loadCACertPool()
	if err != nil {
		t.Fatal(err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	if caCertPool != nil {
		tlsConfig.ClientCAs = caCertPool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	creds := credentials.NewTLS(tlsConfig)

	s := grpc.NewServer(grpc.Creds(creds))
	api.RegisterPulsaarAgentServer(s, &server{})

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Logf("server error: %v", err)
		}
	}()
	defer s.Stop()

	time.Sleep(100 * time.Millisecond)

	// Set env vars for client
	originalClientCertFile := os.Getenv("PULSAAR_CLIENT_CERT_FILE")
	originalClientKeyFile := os.Getenv("PULSAAR_CLIENT_KEY_FILE")
	originalCLI_CAFile := os.Getenv("PULSAAR_CA_FILE")
	_ = os.Setenv("PULSAAR_CLIENT_CERT_FILE", clientCertFile)
	_ = os.Setenv("PULSAAR_CLIENT_KEY_FILE", clientKeyFile)
	_ = os.Setenv("PULSAAR_CA_FILE", caCertFile)
	defer func() {
		_ = os.Setenv("PULSAAR_CLIENT_CERT_FILE", originalClientCertFile)
		_ = os.Setenv("PULSAAR_CLIENT_KEY_FILE", originalClientKeyFile)
		_ = os.Setenv("PULSAAR_CA_FILE", originalCLI_CAFile)
	}()

	cliConfig, err := createTLSConfig()
	if err != nil {
		t.Fatal(err)
	}

	// Connect client
	conn, err := grpc.NewClient(fmt.Sprintf("localhost:%d", port), grpc.WithTransportCredentials(credentials.NewTLS(cliConfig)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	client := api.NewPulsaarAgentClient(conn)

	// Test Health
	resp, err := client.Health(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Ready {
		t.Error("expected ready true")
	}
}

func TestIsBinary(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"empty", []byte{}, false},
		{"text", []byte("hello world"), false},
		{"text with newlines", []byte("hello\nworld"), false},
		{"binary null", []byte{0, 1, 2}, true},
		{"mixed", []byte("hello\x00world"), true},
		{"high ascii", []byte("hello\x80world"), true},
		{"control chars", []byte("hello\x01world"), true},
		{"tab ok", []byte("hello\tworld"), false},
		{"newline ok", []byte("hello\nworld"), false},
		{"carriage return ok", []byte("hello\rworld"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBinary(tt.data)
			if got != tt.want {
				t.Errorf("isBinary(%q) = %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}
