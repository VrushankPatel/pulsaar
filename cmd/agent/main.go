package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

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

func (s *server) ReadFile(ctx context.Context, req *api.ReadRequest) (*api.ReadResponse, error) {
	log.Printf("Audit: ReadFile request for path: %s", req.Path)
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

func (s *server) StreamFile(req *api.StreamRequest, stream api.PulsaarAgent_StreamFileServer) error {
	log.Printf("Audit: StreamFile request for path: %s", req.Path)
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
	defer file.Close()

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
		Version:       "v0.1.0",
		StatusMessage: "Agent scaffold ready",
	}, nil
}

func main() {
	cert, err := generateSelfSignedCert()
	if err != nil {
		log.Fatalf("failed to generate cert: %v", err)
	}

	creds := credentials.NewServerTLSFromCert(&cert)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer(grpc.Creds(creds))
	api.RegisterPulsaarAgentServer(s, &server{})

	log.Printf("Pulsaar agent listening on :50051 with TLS")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
