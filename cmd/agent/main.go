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
	"sync"
	"time"

	"github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	api "github.com/VrushankPatel/pulsaar/api"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type server struct {
	api.UnimplementedPulsaarAgentServer
}

const maxReadSize int64 = 1024 * 1024 // 1MB

var limiters sync.Map // map[string]*rate.Limiter
var configuredAllowedRoots []string

func getLimiterForIP(ctx context.Context) *rate.Limiter {
	p, ok := peer.FromContext(ctx)
	if !ok {
		// Fallback: allow unlimited if can't determine peer
		return rate.NewLimiter(rate.Inf, 1)
	}
	addr := p.Addr.String()
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	limiter, ok := limiters.Load(host)
	if !ok {
		limiter = rate.NewLimiter(rate.Limit(10), 10) // 10 operations per second per IP
		limiters.Store(host, limiter)
	}
	return limiter.(*rate.Limiter)
}

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

func initConfiguredAllowedRoots() {
	namespace := getNamespace()
	podName := os.Getenv("PULSAAR_POD_NAME")
	if namespace != "" && podName != "" {
		roots := loadAllowedRootsFromPodAnnotations(namespace, podName)
		if roots != nil {
			configuredAllowedRoots = roots
			return
		}
	}
	if namespace != "" {
		roots := loadAllowedRootsFromConfigMap(namespace)
		if roots != nil {
			configuredAllowedRoots = roots
			return
		}
	}
	// Fallback to env
	roots := os.Getenv("PULSAAR_ALLOWED_ROOTS")
	if roots == "" {
		configuredAllowedRoots = []string{"/"}
	} else {
		configuredAllowedRoots = strings.Split(roots, ",")
		for i, root := range configuredAllowedRoots {
			configuredAllowedRoots[i] = strings.TrimSpace(root)
		}
	}
}

func getNamespace() string {
	if ns := os.Getenv("PULSAAR_NAMESPACE"); ns != "" {
		return ns
	}
	data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func loadAllowedRootsFromConfigMap(namespace string) []string {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil
	}
	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), "pulsaar-config", metav1.GetOptions{})
	if err != nil {
		return nil
	}
	rootsStr, ok := cm.Data["allowed-roots"]
	if !ok {
		return nil
	}
	if rootsStr == "" {
		return []string{}
	}
	roots := strings.Split(rootsStr, ",")
	for i, root := range roots {
		roots[i] = strings.TrimSpace(root)
	}
	return roots
}

func loadAllowedRootsFromPodAnnotations(namespace, podName string) []string {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil
	}
	pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return nil
	}
	rootsStr, ok := pod.Annotations["pulsaar.io/allowed-roots"]
	if !ok {
		return nil
	}
	if rootsStr == "" {
		return []string{}
	}
	roots := strings.Split(rootsStr, ",")
	for i, root := range roots {
		roots[i] = strings.TrimSpace(root)
	}
	return roots
}

func isPathAllowed(path string, allowedRoots []string) bool {
	cleanPath := filepath.Clean(path)
	for _, root := range allowedRoots {
		cleanRoot := filepath.Clean(root)
		if cleanRoot == "/" || cleanPath == cleanRoot || strings.HasPrefix(cleanPath, cleanRoot+"/") {
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
	if !getLimiterForIP(ctx).Allow() {
		return nil, status.Errorf(codes.ResourceExhausted, "Rate limit exceeded. Please wait before retrying.")
	}
	auditLog("ListDirectory", req.Path)
	allowedRoots := req.AllowedRoots
	if len(allowedRoots) == 0 {
		allowedRoots = configuredAllowedRoots
	}
	if !isPathAllowed(req.Path, allowedRoots) {
		return nil, status.Errorf(codes.PermissionDenied, "Access to path '%s' is not allowed. Allowed roots: %v", req.Path, allowedRoots)
	}

	entries, err := os.ReadDir(req.Path)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to list contents of directory '%s': %v", req.Path, err)
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
	if !getLimiterForIP(ctx).Allow() {
		return nil, status.Errorf(codes.ResourceExhausted, "Rate limit exceeded. Please wait before retrying.")
	}
	auditLog("Stat", req.Path)
	allowedRoots := req.AllowedRoots
	if len(allowedRoots) == 0 {
		allowedRoots = configuredAllowedRoots
	}
	if !isPathAllowed(req.Path, allowedRoots) {
		return nil, status.Errorf(codes.PermissionDenied, "Access to path '%s' is not allowed. Allowed roots: %v", req.Path, allowedRoots)
	}

	info, err := os.Stat(req.Path)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to get information for path '%s': %v", req.Path, err)
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
	if !getLimiterForIP(ctx).Allow() {
		return nil, status.Errorf(codes.ResourceExhausted, "Rate limit exceeded. Please wait before retrying.")
	}
	auditLog("ReadFile", req.Path)
	allowedRoots := req.AllowedRoots
	if len(allowedRoots) == 0 {
		allowedRoots = configuredAllowedRoots
	}
	if !isPathAllowed(req.Path, allowedRoots) {
		return nil, status.Errorf(codes.PermissionDenied, "Access to path '%s' is not allowed. Allowed roots: %v", req.Path, allowedRoots)
	}

	readLen := req.Length
	if readLen == 0 {
		readLen = maxReadSize
	}
	if readLen > maxReadSize {
		return nil, status.Errorf(codes.InvalidArgument, "Requested read length (%d bytes) exceeds the maximum allowed size of %d bytes", readLen, maxReadSize)
	}

	file, err := os.Open(req.Path)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to open file '%s' for reading: %v", req.Path, err)
	}
	defer func() { _ = file.Close() }()

	data := make([]byte, readLen)
	n, err := file.ReadAt(data, req.Offset)
	if err != nil && err != io.EOF {
		return nil, status.Errorf(codes.Internal, "Unable to read file '%s': %v", req.Path, err)
	}

	eof := int64(n) < readLen || err == io.EOF
	return &api.ReadResponse{Data: data[:n], Eof: eof}, nil
}

func (s *server) StreamFile(req *api.StreamRequest, stream api.PulsaarAgent_StreamFileServer) error {
	if !getLimiterForIP(stream.Context()).Allow() {
		return status.Errorf(codes.ResourceExhausted, "Rate limit exceeded. Please wait before retrying.")
	}
	auditLog("StreamFile", req.Path)
	allowedRoots := req.AllowedRoots
	if len(allowedRoots) == 0 {
		allowedRoots = configuredAllowedRoots
	}
	if !isPathAllowed(req.Path, allowedRoots) {
		return status.Errorf(codes.PermissionDenied, "Access to path '%s' is not allowed. Allowed roots: %v", req.Path, allowedRoots)
	}

	chunkSize := req.ChunkSize
	if chunkSize == 0 {
		chunkSize = 64 * 1024 // 64KB default
	}
	if chunkSize > maxReadSize {
		return status.Errorf(codes.InvalidArgument, "Requested chunk size (%d bytes) exceeds the maximum allowed size of %d bytes", chunkSize, maxReadSize)
	}

	file, err := os.Open(req.Path)
	if err != nil {
		return status.Errorf(codes.Internal, "Unable to open file '%s' for streaming: %v", req.Path, err)
	}
	defer func() { _ = file.Close() }()

	buf := make([]byte, chunkSize)
	for {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return status.Errorf(codes.Internal, "Unable to read file '%s' during streaming: %v", req.Path, err)
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
		Version:       version,
		StatusMessage: "Agent ready",
		Commit:        commit,
		Date:          date,
	}, nil
}

func main() {
	initConfiguredAllowedRoots()

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
		if err := http.ListenAndServe(":9090", nil); err != nil {
			log.Printf("Failed to start metrics server: %v", err)
		}
	}()

	log.Printf("Pulsaar agent listening on :50051 with TLS")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
