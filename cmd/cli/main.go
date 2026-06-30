package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/emptypb"
	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	api "github.com/VrushankPatel/pulsaar/api"
	"github.com/VrushankPatel/pulsaar/internal/config"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type AccessError struct {
	Type    string // "auth", "rbac", "connectivity", "path"
	Message string
	Detail  string
}

func (e *AccessError) Error() string {
	hint := ""
	switch e.Type {
	case "auth":
		hint = "Check your kubeconfig and token validity"
	case "rbac":
		hint = "Ask cluster admin to grant 'get pods' in this namespace"
	case "connectivity":
		hint = "Verify agent is running: pulsaar health --pod POD -n NAMESPACE"
	case "path":
		hint = "Path may be outside allowed-roots. Check pod annotations"
	}
	if e.Detail != "" {
		return fmt.Sprintf("%s (Type: %s). Hint: %s. Detail: %s", e.Message, e.Type, hint, e.Detail)
	}
	return fmt.Sprintf("%s (Type: %s). Hint: %s", e.Message, e.Type, hint)
}

func wrapgRPCError(err error, path string) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if ok {
		switch st.Code() {
		case codes.PermissionDenied:
			return &AccessError{
				Type:    "path",
				Message: fmt.Sprintf("access denied to path: %s", path),
				Detail:  st.Message(),
			}
		case codes.ResourceExhausted:
			return &AccessError{
				Type:    "connectivity",
				Message: "rate limit exceeded on agent",
				Detail:  st.Message(),
			}
		case codes.InvalidArgument:
			return &AccessError{
				Type:    "path",
				Message: "invalid argument",
				Detail:  st.Message(),
			}
		default:
			return &AccessError{
				Type:    "connectivity",
				Message: "agent RPC error",
				Detail:  st.Message(),
			}
		}
	}
	return &AccessError{
		Type:    "connectivity",
		Message: "failed to connect or communicate with agent",
		Detail:  err.Error(),
	}
}

func isBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	nonPrintable := 0
	for _, b := range data {
		if (b < 32 && b != 9 && b != 10 && b != 13) || b > 126 {
			nonPrintable++
		}
	}
	ratio := float64(nonPrintable) / float64(len(data))
	return ratio > 0.05
}

func getConfig() (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}
	return config, nil
}

func getClientset() (*kubernetes.Clientset, error) {
	config, err := getConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

func getProxyURL(namespace, podName string) (string, error) {
	config, err := getConfig()
	if err != nil {
		return "", err
	}
	return config.Host + "/api/v1/namespaces/" + namespace + "/pods/" + podName + "/proxy/", nil
}

func checkUserAccess(namespace, pod string) error {
	config, err := getConfig()
	if err != nil {
		return &AccessError{
			Type:    "connectivity",
			Message: "unable to connect to Kubernetes cluster. Please check your kubeconfig or in-cluster configuration",
			Detail:  err.Error(),
		}
	}

	token := config.BearerToken
	if token == "" {
		return &AccessError{
			Type:    "auth",
			Message: "RBAC enforcement requires token-based authentication. Ensure you are using a token-based auth method (e.g., not client certs)",
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return &AccessError{
			Type:    "connectivity",
			Message: "failed to create Kubernetes client. Verify your cluster connection and credentials",
			Detail:  err.Error(),
		}
	}

	// TokenReview
	tr := &authenticationv1.TokenReview{
		Spec: authenticationv1.TokenReviewSpec{
			Token: token,
		},
	}
	result, err := clientset.AuthenticationV1().TokenReviews().Create(context.TODO(), tr, metav1.CreateOptions{})
	if err != nil {
		return &AccessError{
			Type:    "auth",
			Message: "failed to validate authentication token. Check your token and cluster connectivity",
			Detail:  err.Error(),
		}
	}
	if !result.Status.Authenticated {
		return &AccessError{
			Type:    "auth",
			Message: "token authentication failed. Please verify your token is valid and not expired",
		}
	}

	user := result.Status.User.Username

	// SubjectAccessReview
	sar := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: namespace,
				Verb:      "get",
				Resource:  "pods",
				Name:      pod,
			},
			User:   user,
			Groups: result.Status.User.Groups,
		},
	}
	sarResult, err := clientset.AuthorizationV1().SubjectAccessReviews().Create(context.TODO(), sar, metav1.CreateOptions{})
	if err != nil {
		return &AccessError{
			Type:    "rbac",
			Message: "failed to check RBAC permissions. Ensure you have the necessary permissions to access pods",
			Detail:  err.Error(),
		}
	}
	if !sarResult.Status.Allowed {
		return &AccessError{
			Type:    "rbac",
			Message: fmt.Sprintf("access denied to pod %s/%s. Check your RBAC permissions for 'get' verb on pods in namespace %s", namespace, pod, namespace),
		}
	}

	return nil
}

func injectEphemeralContainer(podName, namespace string, cfg *config.CLIConfig) error {
	clientset, err := getClientset()
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %v", err)
	}

	pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod: %v", err)
	}

	// Check if already has pulsaar-agent
	for _, c := range pod.Spec.Containers {
		if c.Name == "pulsaar-agent" {
			fmt.Fprintf(os.Stderr, "Info: pulsaar-agent container already running in pod\n")
			return nil
		}
	}
	for _, ec := range pod.Spec.EphemeralContainers {
		if ec.Name == "pulsaar-agent" {
			fmt.Fprintf(os.Stderr, "Info: pulsaar-agent ephemeral container already injected\n")
			return nil
		}
	}

	image := cfg.AgentImage
	if image == "" {
		image = "vrushankpatel/pulsaar-agent:latest"
	}

	fmt.Fprintf(os.Stderr, "Injecting ephemeral container with image %s...\n", image)

	ephemeralContainer := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:  "pulsaar-agent",
			Image: image,
			Ports: []corev1.ContainerPort{
				{
					ContainerPort: 50051,
					Name:          "grpc",
				},
			},
		},
	}

	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, ephemeralContainer)

	_, err = clientset.CoreV1().Pods(namespace).UpdateEphemeralContainers(context.TODO(), podName, pod, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to inject ephemeral container: %v; troubleshoot: check if the cluster allows ephemeral containers and try --connection-method port-forward instead", err)
	}

	fmt.Fprintf(os.Stderr, "Waiting for ephemeral container to start (timeout: 30s)...\n")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for ephemeral container to start; check pod logs with kubectl logs %s -n %s -c pulsaar-agent and verify the image exists and can be pulled by the node", podName, namespace)
		case <-ticker.C:
			pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				continue
			}
			for _, status := range pod.Status.EphemeralContainerStatuses {
				if status.Name == "pulsaar-agent" {
					if status.State.Running != nil {
						fmt.Fprintf(os.Stderr, "✓ Ephemeral container started successfully\n")
						return nil
					}
					if status.State.Waiting != nil {
						fmt.Fprintf(os.Stderr, "Container waiting: %s. Reason: %s\n", status.State.Waiting.Message, status.State.Waiting.Reason)
						reason := status.State.Waiting.Reason
						if reason == "ImagePullBackOff" || reason == "ErrImagePull" {
							return fmt.Errorf("ephemeral container failed to start: image pull error (%s). Suggestions: check image name and pull secrets", reason)
						}
					}
					if status.State.Terminated != nil {
						return fmt.Errorf("ephemeral container terminated: reason=%s, message=%s", status.State.Terminated.Reason, status.State.Terminated.Message)
					}
				}
			}
		}
	}
}

func createTLSConfig(cfg *config.CLIConfig) (*tls.Config, error) {
	config := &tls.Config{
		InsecureSkipVerify: true, // Default for MVP port-forward
	}

	if cfg.ClientCertFile != "" && cfg.ClientKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.ClientCertFile, cfg.ClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client cert: %v", err)
		}
		config.Certificates = []tls.Certificate{cert}
		config.InsecureSkipVerify = false // Use proper verification if client cert provided
	}

	if cfg.CaFile != "" {
		caCert, err := os.ReadFile(cfg.CaFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %v", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		config.RootCAs = caCertPool
		config.InsecureSkipVerify = false
	}

	return config, nil
}

func connectToAgent(cmd *cobra.Command, pod, namespace string) (*grpc.ClientConn, func(), error) {
	connectionMethod, _ := cmd.Flags().GetString("connection-method")

	// Load config
	cfg, err := config.LoadCLIConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load CLI configuration: %v", err)
	}

	tlsConfig, err := createTLSConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create TLS configuration. Check your certificate files and environment variables. Error: %v", err)
	}

	// Inject ephemeral container if needed
	err = injectEphemeralContainer(pod, namespace, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to inject Pulsaar agent into pod %s/%s. Ensure the pod supports ephemeral containers and you have permissions to update pods. Error: %v", namespace, pod, err)
	}

	switch connectionMethod {
	case "port-forward":
		// Find a free local port
		lis, err := net.Listen("tcp", ":0")
		if err != nil {
			return nil, nil, fmt.Errorf("unable to find a free local port for port-forwarding. This may indicate too many open connections. Error: %v", err)
		}
		localPort := lis.Addr().(*net.TCPAddr).Port
		if err := lis.Close(); err != nil {
			return nil, nil, fmt.Errorf("failed to close temporary listener. Error: %v", err)
		}

		// Start kubectl port-forward
		kubectlCmd := exec.Command("kubectl", "port-forward", fmt.Sprintf("%s/%s", namespace, pod), fmt.Sprintf("%d:50051", localPort))
		err = kubectlCmd.Start()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to start kubectl port-forward. Ensure kubectl is installed, accessible, and you have permissions to port-forward to the pod. Error: %v", err)
		}

		// Wait for port-forward to be ready
		time.Sleep(2 * time.Second)

		conn, err := grpc.NewClient(fmt.Sprintf("localhost:%d", localPort), grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
		if err != nil {
			_ = kubectlCmd.Process.Kill()
			return nil, nil, fmt.Errorf("failed to establish gRPC connection via port-forward. Check TLS configuration and agent availability. Error: %v", err)
		}

		return conn, func() { _ = kubectlCmd.Process.Kill() }, nil
	case "apiserver-proxy":
		proxyURL, err := getProxyURL(namespace, pod)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to construct apiserver proxy URL. Verify cluster configuration. Error: %v", err)
		}
		conn, err := grpc.NewClient(proxyURL, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to establish gRPC connection via apiserver proxy. Check TLS configuration and agent availability. Error: %v", err)
		}
		return conn, func() {}, nil
	default:
		return nil, nil, fmt.Errorf("unknown connection method '%s'. Supported methods: port-forward, apiserver-proxy", connectionMethod)
	}
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "pulsaar",
		Short: "Pulsaar CLI for safe file exploration in Kubernetes",
	}

	exploreCmd := &cobra.Command{
		Use:   "explore",
		Short: "Explore files in a pod",
		RunE:  runExplore,
	}

	exploreCmd.Flags().String("pod", "", "Pod name")
	exploreCmd.Flags().String("namespace", "default", "Namespace")
	exploreCmd.Flags().String("path", "/", "Path to explore")
	if err := exploreCmd.MarkFlagRequired("pod"); err != nil {
		panic(err)
	}

	readCmd := &cobra.Command{
		Use:   "read",
		Short: "Read file contents in a pod",
		RunE:  runRead,
	}

	readCmd.Flags().String("pod", "", "Pod name")
	readCmd.Flags().String("namespace", "default", "Namespace")
	readCmd.Flags().String("path", "", "Path to file")
	if err := readCmd.MarkFlagRequired("pod"); err != nil {
		panic(err)
	}
	if err := readCmd.MarkFlagRequired("path"); err != nil {
		panic(err)
	}

	streamCmd := &cobra.Command{
		Use:   "stream",
		Short: "Stream file contents in a pod",
		RunE:  runStream,
	}

	streamCmd.Flags().String("pod", "", "Pod name")
	streamCmd.Flags().String("namespace", "default", "Namespace")
	streamCmd.Flags().String("path", "", "Path to file")
	streamCmd.Flags().Int64("chunk-size", 64*1024, "Chunk size in bytes")
	if err := streamCmd.MarkFlagRequired("pod"); err != nil {
		panic(err)
	}
	if err := streamCmd.MarkFlagRequired("path"); err != nil {
		panic(err)
	}

	statCmd := &cobra.Command{
		Use:   "stat",
		Short: "Get file or directory info in a pod",
		RunE:  runStat,
	}

	statCmd.Flags().String("pod", "", "Pod name")
	statCmd.Flags().String("namespace", "default", "Namespace")
	statCmd.Flags().String("path", "", "Path to file or directory")
	if err := statCmd.MarkFlagRequired("pod"); err != nil {
		panic(err)
	}
	if err := statCmd.MarkFlagRequired("path"); err != nil {
		panic(err)
	}

	healthCmd := &cobra.Command{
		Use:   "health",
		Short: "Check health of a Pulsaar agent",
		RunE:  runHealth,
	}

	healthCmd.Flags().String("pod", "", "Pod name")
	healthCmd.Flags().String("namespace", "default", "Namespace")
	if err := healthCmd.MarkFlagRequired("pod"); err != nil {
		panic(err)
	}

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage Pulsaar configuration",
	}

	configSetDenylistCmd := &cobra.Command{
		Use:   "set-denylist --pod POD -n NAMESPACE --paths /path1,/path2",
		Short: "Set denied paths for a pod or namespace ConfigMap",
		RunE:  runConfigSetDenylist,
	}
	configSetDenylistCmd.Flags().String("pod", "", "Pod name (if modifying pod annotations)")
	configSetDenylistCmd.Flags().StringSlice("paths", []string{}, "Comma-separated denied paths")
	configSetDenylistCmd.Flags().Bool("configmap", false, "Update the namespace ConfigMap (pulsaar-denylist) instead of the pod annotation")
	configSetDenylistCmd.Flags().String("namespace", "default", "Namespace")

	configCmd.AddCommand(configSetDenylistCmd)

	rootCmd.AddCommand(exploreCmd)
	rootCmd.AddCommand(readCmd)
	rootCmd.AddCommand(streamCmd)
	rootCmd.AddCommand(statCmd)
	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(tuiCmd)

	completionCmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate completion script",
		Long: `To load completions:

Bash:

  $ source <(pulsaar completion bash)

  # To load completions for each session, execute once:

  # Linux:

  $ pulsaar completion bash > /etc/bash_completion.d/pulsaar

  # macOS:

  $ pulsaar completion bash > /usr/local/etc/bash_completion.d/pulsaar

Zsh:

  # If shell completion is not already enabled in your environment,

  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:

  $ pulsaar completion zsh > "${fpath[1]}/_pulsaar"

  # You will need to start a new shell for this setup to take effect.

fish:

  $ pulsaar completion fish | source

  # To load completions for each session, execute once:

  $ pulsaar completion fish > ~/.config/fish/completions/pulsaar.fish

PowerShell:

  PS> pulsaar completion powershell | Out-String | Invoke-Expression

  # To load completions for each session, execute once:

  #    pulsaar completion powershell > pulsaar.ps1

  # and source this file from your PowerShell profile.

`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		Run: func(cmd *cobra.Command, args []string) {
			switch args[0] {
			case "bash":
				if err := cmd.Root().GenBashCompletion(cmd.OutOrStdout()); err != nil {
					log.Fatal(err)
				}
			case "zsh":
				if err := cmd.Root().GenZshCompletion(cmd.OutOrStdout()); err != nil {
					log.Fatal(err)
				}
			case "fish":
				if err := cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true); err != nil {
					log.Fatal(err)
				}
			case "powershell":
				if err := cmd.Root().GenPowerShellCompletionWithDesc(cmd.OutOrStdout()); err != nil {
					log.Fatal(err)
				}
			}
		},
	}

	rootCmd.AddCommand(completionCmd)

	manCmd := &cobra.Command{
		Use:   "man",
		Short: "Generate man pages",
		RunE:  runMan,
	}

	rootCmd.AddCommand(manCmd)

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Version: %s\nCommit: %s\nDate: %s\n", version, commit, date)
		},
	}

	rootCmd.AddCommand(versionCmd)

	rootCmd.PersistentFlags().String("connection-method", "port-forward", "Connection method: port-forward or apiserver-proxy")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func runExplore(cmd *cobra.Command, args []string) error {
	pod, _ := cmd.Flags().GetString("pod")
	namespace, _ := cmd.Flags().GetString("namespace")
	path, _ := cmd.Flags().GetString("path")

	err := checkUserAccess(namespace, pod)
	if err != nil {
		return err
	}

	conn, cleanup, err := connectToAgent(cmd, pod, namespace)
	if err != nil {
		return err
	}
	defer cleanup()
	defer func() { _ = conn.Close() }()

	client := api.NewPulsaarAgentClient(conn)

	resp, err := client.ListDirectory(context.Background(), &api.ListRequest{
		Path:         path,
		AllowedRoots: []string{},
	})
	if err != nil {
		return wrapgRPCError(err, path)
	}

	for _, entry := range resp.Entries {
		fmt.Printf("%s %s %d %s\n", entry.Mode, entry.Name, entry.SizeBytes, entry.Mtime.AsTime().Format("2006-01-02 15:04:05"))
	}

	return nil
}

func runRead(cmd *cobra.Command, args []string) error {
	pod, _ := cmd.Flags().GetString("pod")
	namespace, _ := cmd.Flags().GetString("namespace")
	path, _ := cmd.Flags().GetString("path")

	err := checkUserAccess(namespace, pod)
	if err != nil {
		return err
	}

	conn, cleanup, err := connectToAgent(cmd, pod, namespace)
	if err != nil {
		return err
	}
	defer cleanup()
	defer func() { _ = conn.Close() }()

	client := api.NewPulsaarAgentClient(conn)

	resp, err := client.ReadFile(context.Background(), &api.ReadRequest{
		Path:         path,
		Offset:       0,
		Length:       0, // read up to max
		AllowedRoots: []string{},
	})
	if err != nil {
		return wrapgRPCError(err, path)
	}

	if isBinary(resp.Data) {
		fmt.Println("Warning: This file appears to be binary. Output may be corrupted.")
	}
	fmt.Print(string(resp.Data))
	if !resp.Eof {
		fmt.Println("\n... (file truncated)")
	}

	return nil
}

func runStream(cmd *cobra.Command, args []string) error {
	pod, _ := cmd.Flags().GetString("pod")
	namespace, _ := cmd.Flags().GetString("namespace")
	path, _ := cmd.Flags().GetString("path")
	chunkSize, _ := cmd.Flags().GetInt64("chunk-size")

	err := checkUserAccess(namespace, pod)
	if err != nil {
		return err
	}

	conn, cleanup, err := connectToAgent(cmd, pod, namespace)
	if err != nil {
		return err
	}
	defer cleanup()
	defer func() { _ = conn.Close() }()

	client := api.NewPulsaarAgentClient(conn)

	stream, err := client.StreamFile(context.Background(), &api.StreamRequest{
		Path:         path,
		ChunkSize:    chunkSize,
		AllowedRoots: []string{},
	})
	if err != nil {
		return wrapgRPCError(err, path)
	}

	warned := false
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return wrapgRPCError(err, path)
		}
		if !warned && isBinary(resp.Data) {
			fmt.Println("Warning: This file appears to be binary. Output may be corrupted.")
			warned = true
		}
		fmt.Print(string(resp.Data))
	}

	return nil
}

func runStat(cmd *cobra.Command, args []string) error {
	pod, _ := cmd.Flags().GetString("pod")
	namespace, _ := cmd.Flags().GetString("namespace")
	path, _ := cmd.Flags().GetString("path")

	err := checkUserAccess(namespace, pod)
	if err != nil {
		return err
	}

	conn, cleanup, err := connectToAgent(cmd, pod, namespace)
	if err != nil {
		return err
	}
	defer cleanup()
	defer func() { _ = conn.Close() }()

	client := api.NewPulsaarAgentClient(conn)

	resp, err := client.Stat(context.Background(), &api.StatRequest{
		Path:         path,
		AllowedRoots: []string{"/"},
	})
	if err != nil {
		return wrapgRPCError(err, path)
	}

	fmt.Printf("Name: %s\n", resp.Info.Name)
	fmt.Printf("IsDir: %t\n", resp.Info.IsDir)
	fmt.Printf("Size: %d bytes\n", resp.Info.SizeBytes)
	fmt.Printf("Mode: %s\n", resp.Info.Mode)
	fmt.Printf("Modified: %s\n", resp.Info.Mtime.AsTime().Format("2006-01-02 15:04:05"))

	return nil
}

func runHealth(cmd *cobra.Command, args []string) error {
	pod, _ := cmd.Flags().GetString("pod")
	namespace, _ := cmd.Flags().GetString("namespace")

	err := checkUserAccess(namespace, pod)
	if err != nil {
		return err
	}

	conn, cleanup, err := connectToAgent(cmd, pod, namespace)
	if err != nil {
		return err
	}
	defer cleanup()
	defer func() { _ = conn.Close() }()

	client := api.NewPulsaarAgentClient(conn)

	resp, err := client.Health(context.Background(), &emptypb.Empty{})
	if err != nil {
		return wrapgRPCError(err, "")
	}

	fmt.Printf("Ready: %t\n", resp.Ready)
	fmt.Printf("Version: %s\n", resp.Version)
	fmt.Printf("Status: %s\n", resp.StatusMessage)
	fmt.Printf("Commit: %s\n", resp.Commit)
	fmt.Printf("Date: %s\n", resp.Date)

	return nil
}

func runMan(cmd *cobra.Command, args []string) error {
	header := &doc.GenManHeader{
		Title:   "PULSAAR",
		Section: "1",
	}
	err := os.MkdirAll("man", 0755)
	if err != nil {
		return err
	}
	return doc.GenManTree(cmd.Root(), header, "man")
}

func runConfigSetDenylist(cmd *cobra.Command, args []string) error {
	paths, _ := cmd.Flags().GetStringSlice("paths")
	pod, _ := cmd.Flags().GetString("pod")
	namespace, _ := cmd.Flags().GetString("namespace")
	useConfigMap, _ := cmd.Flags().GetBool("configmap")

	// Validate paths
	for _, p := range paths {
		if !filepath.IsAbs(p) {
			return fmt.Errorf("path must be absolute: %s", p)
		}
	}

	clientset, err := getClientset()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	pathsStr := strings.Join(paths, ",")

	if useConfigMap {
		cmName := "pulsaar-denylist"
		cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), cmName, metav1.GetOptions{})
		if err != nil {
			// If ConfigMap doesn't exist, create it
			newCm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cmName,
					Namespace: namespace,
				},
				Data: map[string]string{
					"denied-paths": pathsStr,
				},
			}
			_, err = clientset.CoreV1().ConfigMaps(namespace).Create(context.TODO(), newCm, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create ConfigMap %s: %w", cmName, err)
			}
			fmt.Fprintf(os.Stderr, "ConfigMap %s updated with denied paths: %s\n", cmName, pathsStr)
			return nil
		}

		if cm.Data == nil {
			cm.Data = make(map[string]string)
		}
		cm.Data["denied-paths"] = pathsStr
		_, err = clientset.CoreV1().ConfigMaps(namespace).Update(context.TODO(), cm, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update ConfigMap %s: %w", cmName, err)
		}
		fmt.Fprintf(os.Stderr, "ConfigMap %s updated with denied paths: %s\n", cmName, pathsStr)
	} else {
		if pod == "" {
			return fmt.Errorf("--pod flag is required when setting denylist via pod annotations")
		}
		pObj, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), pod, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get pod %s/%s: %w", namespace, pod, err)
		}

		if pObj.Annotations == nil {
			pObj.Annotations = make(map[string]string)
		}
		pObj.Annotations["pulsaar.io/denied-paths"] = pathsStr
		_, err = clientset.CoreV1().Pods(namespace).Update(context.TODO(), pObj, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update annotations on pod %s: %w", pod, err)
		}
		fmt.Fprintf(os.Stderr, "Pod %s/%s annotations updated with denied paths: %s\n", namespace, pod, pathsStr)
	}
	return nil
}
