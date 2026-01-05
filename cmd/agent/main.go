package main

import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	api "github.com/VrushankPatel/pulsaar/api"
)

type server struct {
	api.UnimplementedPulsaarAgentServer
}

func (s *server) ListDirectory(ctx context.Context, req *api.ListRequest) (*api.ListResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "ListDirectory not implemented")
}

func (s *server) Stat(ctx context.Context, req *api.StatRequest) (*api.StatResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "Stat not implemented")
}

func (s *server) ReadFile(ctx context.Context, req *api.ReadRequest) (*api.ReadResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "ReadFile not implemented")
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
