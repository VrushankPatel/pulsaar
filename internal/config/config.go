package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Common constants
const (
	MaxReadSize int64 = 1024 * 1024 // 1MB
)

// AgentConfig holds configuration for the agent
type AgentConfig struct {
	AllowedRoots []string
	DeniedPaths  []string
	TLSCertFile  string
	TLSKeyFile   string
	TLSCaFile    string
}

// CLIConfig holds configuration for the CLI
type CLIConfig struct {
	ClientCertFile string
	ClientKeyFile  string
	CaFile         string
	AgentImage     string
}

// LoadAgentConfig loads the agent configuration from environment variables and Kubernetes resources
func LoadAgentConfig() (*AgentConfig, error) {
	config := &AgentConfig{}

	// Load TLS paths
	config.TLSCertFile = os.Getenv("PULSAAR_TLS_CERT_FILE")
	config.TLSKeyFile = os.Getenv("PULSAAR_TLS_KEY_FILE")
	config.TLSCaFile = os.Getenv("PULSAAR_TLS_CA_FILE")

	// Load Denied Paths
	config.DeniedPaths = loadDeniedPaths()

	// Load Allowed Roots
	var err error
	config.AllowedRoots, err = loadAllowedRoots()
	if err != nil {
		// Log error but fallback to default if strictly necessary, or return error.
		// For now, consistent with original behavior, we return error if k8s fails BUT
		// original code fell back to env vars if k8s failed or wasn't present?
		// Re-reading original code:
		// It tries PodAnnotations, then ConfigMap, then Env Var.
		// If k8s client fails, it continues to next method (Env var fallback is at the end).
		// So we should handle errors gracefully and fall through.
		// Let's implement loadAllowedRoots to match that logic.
	}

	// Fallback logic is inside loadAllowedRoots, so if it returns nil/empty without error, it means strictly empty?
	// The original code sets default to "/" if nothing found.
	if len(config.AllowedRoots) == 0 {
		config.AllowedRoots = []string{"/"}
	}

	return config, nil
}

// IsPathAllowed checks if the given path is allowed based on the configuration and optional overrides.
// overrideAllowedRoots can be passed to provide specific allowed roots for a request.
// If overrideAllowedRoots is empty, the configured AllowedRoots are used.
func (c *AgentConfig) IsPathAllowed(path string, overrideAllowedRoots []string) bool {
	cleanPath := filepath.Clean(path)

	// First, check denylist
	for _, deny := range c.DeniedPaths {
		cleanDeny := filepath.Clean(deny)
		if cleanDeny == "/" || cleanPath == cleanDeny || strings.HasPrefix(cleanPath, cleanDeny+"/") {
			return false
		}
	}

	// Determine effective allowed roots
	allowedRoots := overrideAllowedRoots
	if len(allowedRoots) == 0 {
		allowedRoots = c.AllowedRoots
	}

	// Then, check allowlist
	for _, root := range allowedRoots {
		cleanRoot := filepath.Clean(root)
		if cleanRoot == "/" || cleanPath == cleanRoot || strings.HasPrefix(cleanPath, cleanRoot+"/") {
			return true
		}
	}
	return false
}

// LoadCLIConfig loads the CLI configuration from environment variables
func LoadCLIConfig() (*CLIConfig, error) {
	config := &CLIConfig{}

	config.ClientCertFile = os.Getenv("PULSAAR_CLIENT_CERT_FILE")
	config.ClientKeyFile = os.Getenv("PULSAAR_CLIENT_KEY_FILE")
	config.CaFile = os.Getenv("PULSAAR_CA_FILE")
	config.AgentImage = os.Getenv("PULSAAR_AGENT_IMAGE")
	if config.AgentImage == "" {
		config.AgentImage = "pulsaar/agent:latest"
	}

	return config, nil
}

func loadDeniedPaths() []string {
	denylist := os.Getenv("PULSAAR_DENIED_PATHS")
	if denylist == "" {
		return []string{}
	}
	paths := strings.Split(denylist, ",")
	for i, d := range paths {
		paths[i] = strings.TrimSpace(d)
	}
	return paths
}

func loadAllowedRoots() ([]string, error) {
	namespace := getNamespace()

	// 1. Pod Annotations
	podName := os.Getenv("PULSAAR_POD_NAME")
	if namespace != "" && podName != "" {
		roots, err := loadAllowedRootsFromPodAnnotations(namespace, podName)
		if err == nil && roots != nil {
			return roots, nil
		}
	}

	// 2. ConfigMap
	if namespace != "" {
		roots, err := loadAllowedRootsFromConfigMap(namespace)
		if err == nil && roots != nil {
			return roots, nil
		}
	}

	// 3. Environment Variable
	rootsStr := os.Getenv("PULSAAR_ALLOWED_ROOTS")
	if rootsStr != "" {
		roots := strings.Split(rootsStr, ",")
		for i, root := range roots {
			roots[i] = strings.TrimSpace(root)
		}
		return roots, nil
	}

	// Default
	return []string{"/"}, nil
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

func loadAllowedRootsFromConfigMap(namespace string) ([]string, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), "pulsaar-config", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	rootsStr, ok := cm.Data["allowed-roots"]
	if !ok {
		return nil, nil // Key not found, but no error accessing CM
	}
	if rootsStr == "" {
		return []string{}, nil
	}
	roots := strings.Split(rootsStr, ",")
	for i, root := range roots {
		roots[i] = strings.TrimSpace(root)
	}
	return roots, nil
}

func loadAllowedRootsFromPodAnnotations(namespace, podName string) ([]string, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	rootsStr, ok := pod.Annotations["pulsaar.io/allowed-roots"]
	if !ok {
		return nil, nil // Annotation not found
	}
	if rootsStr == "" {
		return []string{}, nil
	}
	roots := strings.Split(rootsStr, ",")
	for i, root := range roots {
		roots[i] = strings.TrimSpace(root)
	}
	return roots, nil
}
