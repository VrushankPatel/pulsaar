package main

import (
	"context"
	"io"
	"log"
	"net"

	"os"
	"path/filepath"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	api "github.com/VrushankPatel/pulsaar/api"
)

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

func (s *server) Stat(ctx context.Context, req *api.StatRequest) (*api.StatResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "Stat not implemented")
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

func (s *server) StreamFile(req *api.StreamRequest, stream api.PulsaarAgent_StreamFileServer) error {
	return status.Errorf(codes.Unimplemented, "StreamFile not implemented")
}

func (s *server) Health(ctx context.Context, req *emptypb.Empty) (*api.HealthResponse, error) {
	return &api.HealthResponse{
		Ready:         true,
		Version:       "v0.1.0",
		StatusMessage: "Agent scaffold ready",
	}, nil
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	api.RegisterPulsaarAgentServer(s, &server{})

	log.Printf("Pulsaar agent listening on :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
