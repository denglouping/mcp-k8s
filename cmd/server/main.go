package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

const (
	sseMode   = "sse"
	stdioMode = "stdio"
)

func main() {
	kubeconfigPath := flag.String("kubeconfig", "/root/.kube/config", "Path to Kubernetes configuration file (uses default config if not specified)")
	mode := flag.String("mode", stdioMode, "Mode to run mcp server, sse or stdio)")
	addr := flag.String("addr", "0.0.0.0:6216", "Addr to run in sse mode")
	flag.Parse()

	if kubeconfigPath == nil && *kubeconfigPath == "" {
		klog.Fatalf("Kubeconfig path: %s", *kubeconfigPath)
	}

	restConfig, err := clientcmd.BuildConfigFromFlags("", *kubeconfigPath)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		klog.Fatalf("Error creating clientset: %v", err)
	}

	// Create MCP server
	s := server.NewMCPServer(
		"Demo 🚀",
		"1.0.0",
	)

	// Add tool
	tool := mcp.NewTool("hello_world",
		mcp.WithDescription("Say hello to someone"),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Name of the person to greet"),
		),
	)

	podTool := mcp.NewTool("pod_inspect",
		mcp.WithDescription("Collect pod info, and base on it we may figure out what's wrong with the pod"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Name of the pod to inspect")),
		mcp.WithString("namespace", mcp.Required(), mcp.Description("Namespace of the pod to inspect")),
	)

	// Add tool handler
	s.AddTool(tool, helloHandler)
	s.AddTool(podTool, newPodInspectHandler(clientset))
	// Start the stdio server

	startServer(*mode, *addr, s)

}

func newPodInspectHandler(clientset *kubernetes.Clientset) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, ok := request.Params.Arguments["name"].(string)
		if !ok {
			return nil, errors.New("name must be a string")
		}

		pod, err := clientset.CoreV1().Pods(request.Params.Arguments["namespace"].(string)).Get(ctx, name, v1.GetOptions{})
		if err != nil {
			return nil, err
		}

		if pod.Status.ContainerStatuses == nil {
			return nil, errors.New("pod has no container statuses")
		}

		for _, container := range pod.Status.ContainerStatuses {
			if container.RestartCount > 0 {
				return mcp.NewToolResultText(fmt.Sprintf("Pod %s in namespace %s restart with status %s", pod.Name, pod.Namespace, container.LastTerminationState)), nil

			}

			return mcp.NewToolResultText(fmt.Sprintf("Pod %s in namespace %s has %d containers", pod.Name, pod.Namespace, len(pod.Spec.Containers))), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Pod %s in namespace %s not found", pod.Name, pod.Namespace)), nil
	}
}

func startServer(mode string, addr string, s *server.MCPServer) {
	if mode == sseMode {
		sse := server.NewSSEServer(s)
		if err := sse.Start(addr); err != nil {
			fmt.Printf("Server error: %v\n", err)
		}
	} else {
		if err := server.ServeStdio(s); err != nil {
			fmt.Printf("Server error: %v\n", err)
		}
	}

}

func helloHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, ok := request.Params.Arguments["name"].(string)
	if !ok {
		return nil, errors.New("name must be a string")
	}

	return mcp.NewToolResultText(fmt.Sprintf("Hello, %s!, this is greeting from mcp.", name)), nil
}
