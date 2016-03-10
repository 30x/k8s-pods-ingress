package kubernetes

import (
	"fmt"
	"os"

	client "k8s.io/kubernetes/pkg/client/unversioned"
)

const (
	// ErrNeedsKubeHostSet is the error used when the KUBE_HOST is not set and ran outside of Kubernetes
	ErrNeedsKubeHostSet = "When ran outside of Kubernetes, the KUBE_HOST environment variable is required"
)

/*
GetClient returns a Kubernetes client.
*/
func GetClient() (*client.Client, error) {
	var kubeConfig client.Config

	// Set the Kubernetes configuration based on the environment
	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token"); err == nil {
		config, err := client.InClusterConfig()

		if err != nil {
			return nil, fmt.Errorf("Failed to create in-cluster config: %v.", err)
		}

		kubeConfig = *config
	} else {
		kubeConfig = client.Config{
			Host: os.Getenv("KUBE_HOST"),
		}

		if kubeConfig.Host == "" {
			return nil, fmt.Errorf(ErrNeedsKubeHostSet)
		}
	}

	// Create the Kubernetes client based on the configuration
	return client.New(&kubeConfig)
}
