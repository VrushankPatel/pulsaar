package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	grpcPrometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	api "github.com/VrushankPatel/pulsaar/api"
	"github.com/VrushankPatel/pulsaar/internal/config"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var (
	auditLogErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pulsaar_audit_log_errors_total",
			Help: "Total number of audit log send failures",
		},
		[]string{"operation", "error_type"},
	)
	rateLimitExceededTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pulsaar_rate_limit_exceeded_total",
			Help: "Total number of rate limit rejections",
		},
		[]string{"client_ip"},
	)
)

func init() {
	prometheus.MustRegister(auditLogErrorsTotal, rateLimitExceededTotal)
}

type server struct {
	api.UnimplementedPulsaarAgentServer
	config *config.AgentConfig
}

var limiters sync.Map // map[string]*rate.Limiter

func getClientIP(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "unknown"
	}
	addr := p.Addr.String()
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	return host
}

func (s *server) getLimiterForIP(ctx context.Context) *rate.Limiter {
	clientIP := getClientIP(ctx)
	if clientIP == "unknown" {
		return rate.NewLimiter(rate.Inf, 1)
	}
	limiter, ok := limiters.Load(clientIP)
	if !ok {
		limit := rate.Limit(s.config.RateLimitOpsPerSec)
		limiter = rate.NewLimiter(limit, s.config.RateLimitOpsPerSec)
		limiters.Store(clientIP, limiter)
	}
	return limiter.(*rate.Limiter)
}

func auditLog(ctx context.Context, operation, path string) error {
	clientIP := getClientIP(ctx)
	timestamp := time.Now().Format(time.RFC3339)
	slog.Info("Audit request", "operation", operation, "path", path, "client_ip", clientIP, "timestamp", timestamp)

	url := os.Getenv("PULSAAR_AUDIT_AGGREGATOR_URL")
	if url != "" {
		hostname, _ := os.Hostname()
		data := map[string]any{
			"timestamp": timestamp,
			"operation": operation,
			"path":      path,
			"agent_id":  hostname,
			"client_ip": clientIP,
		}
		jsonData, err := json.Marshal(data)
		if err != nil {
			slog.Error("Failed to marshal audit log", "error", err, "operation", operation, "path", path)
			auditLogErrorsTotal.WithLabelValues(operation, "json_marshal_failed").Inc()
			return fmt.Errorf("failed to marshal audit log: %w", err)
		}

		resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
		if resp != nil {
			defer func() { _ = resp.Body.Close() }()
		}
		if err != nil {
			slog.Error("Failed to send audit log to aggregator", "error", err, "operation", operation, "path", path)
			auditLogErrorsTotal.WithLabelValues(operation, "http_post_failed").Inc()
			return fmt.Errorf("failed to send audit log: %w", err)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			slog.Error("Aggregator returned non-success status", "status", resp.Status, "operation", operation, "path", path)
			auditLogErrorsTotal.WithLabelValues(operation, "http_status_error").Inc()
			return fmt.Errorf("aggregator returned status: %s", resp.Status)
		}
	}
	return nil
}

func (s *server) ListDirectory(ctx context.Context, req *api.ListRequest) (*api.ListResponse, error) {
	if !s.getLimiterForIP(ctx).Allow() {
		clientIP := getClientIP(ctx)
		rateLimitExceededTotal.WithLabelValues(clientIP).Inc()
		slog.Warn("Rate limit exceeded", "client_ip", clientIP)
		return nil, status.Errorf(codes.ResourceExhausted, "Rate limit exceeded. Please wait before retrying.")
	}

	if err := auditLog(ctx, "ListDirectory", req.Path); err != nil {
		return nil, status.Errorf(codes.Internal, "audit failure")
	}

	if !s.config.IsPathAllowed(req.Path, req.AllowedRoots) {
		slog.Warn("Access denied", "path", req.Path, "client_ip", getClientIP(ctx))
		return nil, status.Errorf(codes.PermissionDenied, "access denied")
	}

	entries, err := os.ReadDir(req.Path)
	if err != nil {
		slog.Error("Failed to list directory contents", "path", req.Path, "error", err, "client_ip", getClientIP(ctx))
		return nil, status.Errorf(codes.Internal, "internal error")
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
	if !s.getLimiterForIP(ctx).Allow() {
		clientIP := getClientIP(ctx)
		rateLimitExceededTotal.WithLabelValues(clientIP).Inc()
		slog.Warn("Rate limit exceeded", "client_ip", clientIP)
		return nil, status.Errorf(codes.ResourceExhausted, "Rate limit exceeded. Please wait before retrying.")
	}

	if err := auditLog(ctx, "Stat", req.Path); err != nil {
		return nil, status.Errorf(codes.Internal, "audit failure")
	}

	if !s.config.IsPathAllowed(req.Path, req.AllowedRoots) {
		slog.Warn("Access denied", "path", req.Path, "client_ip", getClientIP(ctx))
		return nil, status.Errorf(codes.PermissionDenied, "access denied")
	}

	info, err := os.Stat(req.Path)
	if err != nil {
		slog.Error("Failed to get information for path", "path", req.Path, "error", err, "client_ip", getClientIP(ctx))
		return nil, status.Errorf(codes.Internal, "internal error")
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
	if !s.getLimiterForIP(ctx).Allow() {
		clientIP := getClientIP(ctx)
		rateLimitExceededTotal.WithLabelValues(clientIP).Inc()
		slog.Warn("Rate limit exceeded", "client_ip", clientIP)
		return nil, status.Errorf(codes.ResourceExhausted, "Rate limit exceeded. Please wait before retrying.")
	}

	if err := auditLog(ctx, "ReadFile", req.Path); err != nil {
		return nil, status.Errorf(codes.Internal, "audit failure")
	}

	if !s.config.IsPathAllowed(req.Path, req.AllowedRoots) {
		slog.Warn("Access denied", "path", req.Path, "client_ip", getClientIP(ctx))
		return nil, status.Errorf(codes.PermissionDenied, "access denied")
	}

	readLen := req.Length
	if readLen == 0 {
		readLen = config.MaxReadSize
	}
	if readLen > config.MaxReadSize {
		slog.Warn("Requested read length exceeds maximum allowed size", "requested", readLen, "max", config.MaxReadSize, "client_ip", getClientIP(ctx))
		return nil, status.Errorf(codes.InvalidArgument, "Requested read length (%d bytes) exceeds the maximum allowed size of %d bytes", readLen, config.MaxReadSize)
	}

	file, err := os.Open(req.Path)
	if err != nil {
		slog.Error("Failed to open file for reading", "path", req.Path, "error", err, "client_ip", getClientIP(ctx))
		return nil, status.Errorf(codes.Internal, "internal error")
	}
	defer func() { _ = file.Close() }()

	data := make([]byte, readLen)
	n, err := file.ReadAt(data, req.Offset)
	if err != nil && err != io.EOF {
		slog.Error("Failed to read file", "path", req.Path, "error", err, "client_ip", getClientIP(ctx))
		return nil, status.Errorf(codes.Internal, "internal error")
	}

	eof := int64(n) < readLen || err == io.EOF
	return &api.ReadResponse{Data: data[:n], Eof: eof}, nil
}

func (s *server) StreamFile(req *api.StreamRequest, stream api.PulsaarAgent_StreamFileServer) error {
	ctx := stream.Context()
	if !s.getLimiterForIP(ctx).Allow() {
		clientIP := getClientIP(ctx)
		rateLimitExceededTotal.WithLabelValues(clientIP).Inc()
		slog.Warn("Rate limit exceeded", "client_ip", clientIP)
		return status.Errorf(codes.ResourceExhausted, "Rate limit exceeded. Please wait before retrying.")
	}

	if err := auditLog(ctx, "StreamFile", req.Path); err != nil {
		return status.Errorf(codes.Internal, "audit failure")
	}

	if !s.config.IsPathAllowed(req.Path, req.AllowedRoots) {
		slog.Warn("Access denied", "path", req.Path, "client_ip", getClientIP(ctx))
		return status.Errorf(codes.PermissionDenied, "access denied")
	}

	chunkSize := req.ChunkSize
	if chunkSize == 0 {
		chunkSize = 64 * 1024 // 64KB default
	}
	if chunkSize > config.MaxReadSize {
		slog.Warn("Requested chunk size exceeds maximum allowed size", "requested", chunkSize, "max", config.MaxReadSize, "client_ip", getClientIP(ctx))
		return status.Errorf(codes.InvalidArgument, "Requested chunk size (%d bytes) exceeds the maximum allowed size of %d bytes", chunkSize, config.MaxReadSize)
	}

	file, err := os.Open(req.Path)
	if err != nil {
		slog.Error("Failed to open file for streaming", "path", req.Path, "error", err, "client_ip", getClientIP(ctx))
		return status.Errorf(codes.Internal, "internal error")
	}
	defer func() { _ = file.Close() }()

	buf := make([]byte, chunkSize)
	for {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			slog.Error("Failed to read file during streaming", "path", req.Path, "error", err, "client_ip", getClientIP(ctx))
			return status.Errorf(codes.Internal, "internal error")
		}
		if n == 0 {
			break
		}
		eof := err == io.EOF
		if err := stream.Send(&api.ReadResponse{Data: buf[:n], Eof: eof}); err != nil {
			slog.Error("Failed to send stream chunk to client", "error", err, "client_ip", getClientIP(ctx))
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
		Version:       version,
		StatusMessage: "Agent ready",
		Commit:        commit,
		Date:          date,
	}, nil
}

func main() {
	// Configure structured JSON logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.LoadAgentConfig()
	if err != nil {
		slog.Warn("Failed to load full agent config", "error", err)
	}

	cert, err := cfg.LoadTLSCertificate()
	if err != nil {
		slog.Error("Failed to load or generate cert", "error", err)
		os.Exit(1)
	}

	caCertPool, err := cfg.LoadCACertPool()
	if err != nil {
		slog.Error("Failed to load CA cert pool", "error", err)
		os.Exit(1)
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
		slog.Error("Failed to listen", "error", err)
		os.Exit(1)
	}

	s := grpc.NewServer(
		grpc.Creds(creds),
		grpc.UnaryInterceptor(grpcPrometheus.UnaryServerInterceptor),
		grpc.StreamInterceptor(grpcPrometheus.StreamServerInterceptor),
	)
	api.RegisterPulsaarAgentServer(s, &server{config: cfg})
	grpcPrometheus.Register(s)

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		slog.Info("Metrics server listening on :9090")
		if err := http.ListenAndServe(":9090", nil); err != nil {
			slog.Error("Failed to start metrics server", "error", err)
		}
	}()

	slog.Info("Pulsaar agent listening on :50051 with TLS", "version", version, "commit", commit)
	if err := s.Serve(lis); err != nil {
		slog.Error("Failed to serve gRPC", "error", err)
		os.Exit(1)
	}
}

