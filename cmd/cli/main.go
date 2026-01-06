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
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/types/known/emptypb"
	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	api "github.com/VrushankPatel/pulsaar/api"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

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
		return fmt.Errorf("unable to connect to Kubernetes cluster. Please check your kubeconfig or in-cluster configuration. Error: %v", err)
	}

	token := config.BearerToken
	if token == "" {
		return fmt.Errorf("RBAC enforcement requires token-based authentication. Ensure you are using a token-based auth method (e.g., not client certs)")
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client. Verify your cluster connection and credentials. Error: %v", err)
	}

	// TokenReview
	tr := &authenticationv1.TokenReview{
		Spec: authenticationv1.TokenReviewSpec{
			Token: token,
		},
	}
	result, err := clientset.AuthenticationV1().TokenReviews().Create(context.TODO(), tr, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to validate authentication token. Check your token and cluster connectivity. Error: %v", err)
	}
	if !result.Status.Authenticated {
		return fmt.Errorf("token authentication failed. Please verify your token is valid and not expired")
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
		return fmt.Errorf("failed to check RBAC permissions. Ensure you have the necessary permissions to access pods. Error: %v", err)
	}
	if !sarResult.Status.Allowed {
		return fmt.Errorf("access denied to pod %s/%s. Check your RBAC permissions for 'get' verb on pods in namespace %s", namespace, pod, namespace)
	}

	return nil
}

func injectEphemeralContainer(podName, namespace string) error {
	clientset, err := getClientset()
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %v", err)
	}

	pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod: %v", err)
	}

	// Check if already has pulsaar-agent container
	for _, c := range pod.Spec.Containers {
		if c.Name == "pulsaar-agent" {
			return nil
		}
	}
	for _, ec := range pod.Spec.EphemeralContainers {
		if ec.Name == "pulsaar-agent" {
			return nil
		}
	}

	// Add ephemeral container
	image := os.Getenv("PULSAAR_AGENT_IMAGE")
	if image == "" {
		image = "pulsaar/agent:latest"
	}

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

	// Patch the pod
	_, err = clientset.CoreV1().Pods(namespace).UpdateEphemeralContainers(context.TODO(), podName, pod, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update ephemeral containers: %v", err)
	}

	// Wait for the container to be running
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err = wait.PollUntilContextTimeout(ctx, 1*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		for _, status := range pod.Status.EphemeralContainerStatuses {
			if status.Name == "pulsaar-agent" && status.State.Running != nil {
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("failed to wait for ephemeral container: %v", err)
	}

	return nil
}

func createTLSConfig() (*tls.Config, error) {
	config := &tls.Config{
		InsecureSkipVerify: true, // Default for MVP port-forward
	}

	clientCertFile := os.Getenv("PULSAAR_CLIENT_CERT_FILE")
	clientKeyFile := os.Getenv("PULSAAR_CLIENT_KEY_FILE")
	caFile := os.Getenv("PULSAAR_CA_FILE")

	if clientCertFile != "" && clientKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client cert: %v", err)
		}
		config.Certificates = []tls.Certificate{cert}
		config.InsecureSkipVerify = false // Use proper verification if client cert provided
	}

	if caFile != "" {
		caCert, err := os.ReadFile(caFile)
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
	tlsConfig, err := createTLSConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create TLS configuration. Check your certificate files and environment variables (PULSAAR_CLIENT_CERT_FILE, PULSAAR_CLIENT_KEY_FILE, PULSAAR_CA_FILE). Error: %v", err)
	}

	// Inject ephemeral container if needed
	err = injectEphemeralContainer(pod, namespace)
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

	rootCmd.AddCommand(exploreCmd)
	rootCmd.AddCommand(readCmd)
	rootCmd.AddCommand(streamCmd)
	rootCmd.AddCommand(statCmd)
	rootCmd.AddCommand(healthCmd)

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

	rootCmd.Flags().String("connection-method", "port-forward", "Connection method: port-forward or apiserver-proxy")

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
		return fmt.Errorf("failed to list directory '%s' in pod %s/%s. This may be due to permission restrictions, invalid path, or agent connectivity issues. Error: %v", path, namespace, pod, err)
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
		return fmt.Errorf("failed to read file '%s' in pod %s/%s. Check if the file exists, is within allowed paths, and you have read permissions. Error: %v", path, namespace, pod, err)
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
		return fmt.Errorf("failed to stream file '%s' in pod %s/%s. Ensure the file is readable and within size limits. Error: %v", path, namespace, pod, err)
	}

	warned := false
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error while streaming file '%s': %v", path, err)
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
		return fmt.Errorf("failed to get info for path '%s' in pod %s/%s. Verify the path exists and is accessible. Error: %v", path, namespace, pod, err)
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
		return fmt.Errorf("failed to get health from pod %s/%s. Error: %v", namespace, pod, err)
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
