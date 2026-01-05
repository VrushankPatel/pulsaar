package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	api "github.com/VrushankPatel/pulsaar/api"
)

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
	exploreCmd.MarkFlagRequired("pod")

	readCmd := &cobra.Command{
		Use:   "read",
		Short: "Read file contents in a pod",
		RunE:  runRead,
	}

	readCmd.Flags().String("pod", "", "Pod name")
	readCmd.Flags().String("namespace", "default", "Namespace")
	readCmd.Flags().String("path", "", "Path to file")
	readCmd.MarkFlagRequired("pod")
	readCmd.MarkFlagRequired("path")

	streamCmd := &cobra.Command{
		Use:   "stream",
		Short: "Stream file contents in a pod",
		RunE:  runStream,
	}

	streamCmd.Flags().String("pod", "", "Pod name")
	streamCmd.Flags().String("namespace", "default", "Namespace")
	streamCmd.Flags().String("path", "", "Path to file")
	streamCmd.Flags().Int64("chunk-size", 64*1024, "Chunk size in bytes")
	streamCmd.MarkFlagRequired("pod")
	streamCmd.MarkFlagRequired("path")

	rootCmd.AddCommand(exploreCmd)
	rootCmd.AddCommand(readCmd)
	rootCmd.AddCommand(streamCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func runExplore(cmd *cobra.Command, args []string) error {
	pod, _ := cmd.Flags().GetString("pod")
	namespace, _ := cmd.Flags().GetString("namespace")
	path, _ := cmd.Flags().GetString("path")

	// Find a free local port
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		return fmt.Errorf("failed to find free port: %v", err)
	}
	localPort := lis.Addr().(*net.TCPAddr).Port
	lis.Close()

	// Start kubectl port-forward
	kubectlCmd := exec.Command("kubectl", "port-forward", fmt.Sprintf("%s/%s", namespace, pod), fmt.Sprintf("%d:50051", localPort))
	err = kubectlCmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start kubectl port-forward: %v", err)
	}
	defer kubectlCmd.Process.Kill()

	// Wait for port-forward to be ready
	time.Sleep(2 * time.Second)

	// Connect gRPC
	conn, err := grpc.Dial(fmt.Sprintf("localhost:%d", localPort), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})))
	if err != nil {
		return fmt.Errorf("failed to connect gRPC: %v", err)
	}
	defer conn.Close()

	client := api.NewPulsaarAgentClient(conn)

	resp, err := client.ListDirectory(context.Background(), &api.ListRequest{
		Path:         path,
		AllowedRoots: []string{"/"},
	})
	if err != nil {
		return fmt.Errorf("failed to list directory: %v", err)
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

	// Find a free local port
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		return fmt.Errorf("failed to find free port: %v", err)
	}
	localPort := lis.Addr().(*net.TCPAddr).Port
	lis.Close()

	// Start kubectl port-forward
	kubectlCmd := exec.Command("kubectl", "port-forward", fmt.Sprintf("%s/%s", namespace, pod), fmt.Sprintf("%d:50051", localPort))
	err = kubectlCmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start kubectl port-forward: %v", err)
	}
	defer kubectlCmd.Process.Kill()

	// Wait for port-forward to be ready
	time.Sleep(2 * time.Second)

	// Connect gRPC
	conn, err := grpc.Dial(fmt.Sprintf("localhost:%d", localPort), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})))
	if err != nil {
		return fmt.Errorf("failed to connect gRPC: %v", err)
	}
	defer conn.Close()

	client := api.NewPulsaarAgentClient(conn)

	resp, err := client.ReadFile(context.Background(), &api.ReadRequest{
		Path:         path,
		Offset:       0,
		Length:       0, // read up to max
		AllowedRoots: []string{"/"},
	})
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
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

	// Find a free local port
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		return fmt.Errorf("failed to find free port: %v", err)
	}
	localPort := lis.Addr().(*net.TCPAddr).Port
	lis.Close()

	// Start kubectl port-forward
	kubectlCmd := exec.Command("kubectl", "port-forward", fmt.Sprintf("%s/%s", namespace, pod), fmt.Sprintf("%d:50051", localPort))
	err = kubectlCmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start kubectl port-forward: %v", err)
	}
	defer kubectlCmd.Process.Kill()

	// Wait for port-forward to be ready
	time.Sleep(2 * time.Second)

	// Connect gRPC
	conn, err := grpc.Dial(fmt.Sprintf("localhost:%d", localPort), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})))
	if err != nil {
		return fmt.Errorf("failed to connect gRPC: %v", err)
	}
	defer conn.Close()

	client := api.NewPulsaarAgentClient(conn)

	stream, err := client.StreamFile(context.Background(), &api.StreamRequest{
		Path:         path,
		ChunkSize:    chunkSize,
		AllowedRoots: []string{"/"},
	})
	if err != nil {
		return fmt.Errorf("failed to stream file: %v", err)
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to receive stream: %v", err)
		}
		fmt.Print(string(resp.Data))
	}

	return nil
}
