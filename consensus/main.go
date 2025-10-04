package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// ConfigMap name to use for leader election
	leaderConfigMapName = "consensus-leader"
	// Namespace where the ConfigMap will be created
	namespace = "default"
	// How often the leader should renew its lease
	leaderRenewalInterval = 1 * time.Second
	// How long a lease is valid for
	leaseValidityDuration = 5 * time.Second
	// How often non-leaders check if they can become the leader
	leaderCheckInterval = 1 * time.Second
)

func main() {
	// Get the pod name from environment variable (set by Kubernetes)
	podName := os.Getenv("POD_NAME")
	if podName == "" {
		// For local testing, generate a random name
		podName = fmt.Sprintf("pod-%d", time.Now().Unix())
		fmt.Printf("POD_NAME not set, using generated name: %s\n", podName)
	}

	// Create Kubernetes client
	clientset, err := createKubernetesClient()
	if err != nil {
		fmt.Printf("Error creating Kubernetes client: %v\n", err)
		os.Exit(1)
	}

	// Create a context that can be canceled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle termination signals
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signalChan
		fmt.Println("Received termination signal")
		cancel()
		os.Exit(0)
	}()

	// Ensure the leader ConfigMap exists
	err = ensureLeaderConfigMap(ctx, clientset)
	if err != nil {
		fmt.Printf("Error ensuring leader ConfigMap: %v\n", err)
		os.Exit(1)
	}

	// Start the leader election process
	runLeaderElection(ctx, clientset, podName)
}

// createKubernetesClient creates a Kubernetes clientset
func createKubernetesClient() (*kubernetes.Clientset, error) {
	// Try to use in-cluster config first (when running in Kubernetes)
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig file for local development
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = os.ExpandEnv("$HOME/.kube/config")
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create Kubernetes config: %v", err)
		}
	}

	// Create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes clientset: %v", err)
	}

	return clientset, nil
}

// ensureLeaderConfigMap ensures the leader ConfigMap exists
func ensureLeaderConfigMap(ctx context.Context, clientset *kubernetes.Clientset) error {
	_, err := clientset.CoreV1().ConfigMaps(namespace).Get(ctx, leaderConfigMapName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// ConfigMap doesn't exist, create it
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: leaderConfigMapName,
				},
				Data: map[string]string{
					"leader":      "",
					"lastUpdated": "",
				},
			}
			_, err = clientset.CoreV1().ConfigMaps(namespace).Create(ctx, configMap, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create leader ConfigMap: %v", err)
			}
			fmt.Println("Created leader ConfigMap")
		} else {
			return fmt.Errorf("failed to check if leader ConfigMap exists: %v", err)
		}
	}
	return nil
}

// tryBecomeLeader attempts to become the leader by updating the ConfigMap
func tryBecomeLeader(ctx context.Context, clientset *kubernetes.Clientset, podName string) (bool, error) {
	// Get the current ConfigMap
	configMap, err := clientset.CoreV1().ConfigMaps(namespace).Get(ctx, leaderConfigMapName, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get leader ConfigMap: %v", err)
	}

	currentLeader := configMap.Data["leader"]
	lastUpdatedStr := configMap.Data["lastUpdated"]

	// If there's a current leader, check if its lease is still valid
	if currentLeader != "" && currentLeader != podName {
		// Parse the last updated time
		lastUpdated, err := time.Parse(time.RFC3339, lastUpdatedStr)
		if err == nil {
			// Check if the lease is still valid
			if time.Since(lastUpdated) < leaseValidityDuration {
				// Lease is still valid, can't become leader
				return false, nil
			}
		}
	}

	// Try to become the leader by updating the ConfigMap
	configMap.Data["leader"] = podName
	configMap.Data["lastUpdated"] = time.Now().Format(time.RFC3339)

	_, err = clientset.CoreV1().ConfigMaps(namespace).Update(ctx, configMap, metav1.UpdateOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to update leader ConfigMap: %v", err)
	}

	return true, nil
}

// renewLeadership renews the leadership by updating the lastUpdated field
func renewLeadership(ctx context.Context, clientset *kubernetes.Clientset, podName string) error {
	// Get the current ConfigMap
	configMap, err := clientset.CoreV1().ConfigMaps(namespace).Get(ctx, leaderConfigMapName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get leader ConfigMap: %v", err)
	}

	// Check if we're still the leader
	if configMap.Data["leader"] != podName {
		return fmt.Errorf("no longer the leader")
	}

	// Update the lastUpdated field
	configMap.Data["lastUpdated"] = time.Now().Format(time.RFC3339)

	_, err = clientset.CoreV1().ConfigMaps(namespace).Update(ctx, configMap, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update leader ConfigMap: %v", err)
	}

	return nil
}

// runLeaderElection runs the leader election process
func runLeaderElection(ctx context.Context, clientset *kubernetes.Clientset, podName string) {
	isLeader := false

	for {
		select {
		case <-ctx.Done():
			return
		default:
			if isLeader {
				// We're the leader, renew our leadership
				err := renewLeadership(ctx, clientset, podName)
				if err != nil {
					fmt.Printf("Failed to renew leadership: %v\n", err)
					isLeader = false
					time.Sleep(leaderCheckInterval)
					continue
				}

				// Do leader work
				fmt.Println("Doing Work...")
				time.Sleep(leaderRenewalInterval)
			} else {
				// Try to become the leader
				becameLeader, err := tryBecomeLeader(ctx, clientset, podName)
				if err != nil {
					fmt.Printf("Error trying to become leader: %v\n", err)
				} else if becameLeader {
					fmt.Println("Became the leader!")
					isLeader = true
				} else {
					// Not the leader, stand by
					fmt.Println("Standing by...")
					time.Sleep(leaderCheckInterval)
				}
			}
		}
	}
}
