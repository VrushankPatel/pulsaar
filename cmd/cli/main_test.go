package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
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
	"google.golang.org/protobuf/types/known/timestamppb"

	api "github.com/VrushankPatel/pulsaar/api"
)

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
	defer file.Close()

	data := make([]byte, readLen)
	n, err := file.ReadAt(data, req.Offset)
	if err != nil && err != io.EOF {
		return nil, status.Errorf(codes.Internal, "failed to read file: %v", err)
	}

	eof := int64(n) < readLen || err == io.EOF
	return &api.ReadResponse{Data: data[:n], Eof: eof}, nil
}

func TestEndToEnd(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "pulsaar_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

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
	defer lis.Close()

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
	conn, err := grpc.Dial(fmt.Sprintf("localhost:%d", port), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

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
	defer os.RemoveAll(tempDir)

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
	defer lis.Close()

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
	conn, err := grpc.Dial(fmt.Sprintf("localhost:%d", port), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

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
