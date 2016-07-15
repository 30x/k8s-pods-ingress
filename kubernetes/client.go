/*
Copyright © 2016 Apigee Corporation

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kubernetes

import (
	"fmt"
	"os"

	"k8s.io/kubernetes/pkg/client/restclient"
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
	var kubeConfig restclient.Config

	// Set the Kubernetes configuration based on the environment
	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token"); err == nil {
		config, err := restclient.InClusterConfig()

		if err != nil {
			return nil, fmt.Errorf("Failed to create in-cluster config: %v.", err)
		}

		kubeConfig = *config
	} else {
		kubeConfig = restclient.Config{
			Host: os.Getenv("KUBE_HOST"),
		}

		if kubeConfig.Host == "" {
			return nil, fmt.Errorf(ErrNeedsKubeHostSet)
		}
	}

	// Create the Kubernetes client based on the configuration
	return client.New(&kubeConfig)
}
