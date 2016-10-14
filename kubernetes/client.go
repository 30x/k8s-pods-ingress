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
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	client "k8s.io/kubernetes/pkg/client/unversioned"
)

// Check if running in cluster
func RunningInCluster() (bool)  {
	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token"); err == nil {
		return true;
	} else {
		return false;
	}
}

/*
GetClient returns a Kubernetes client.
*/
func GetClient() (*client.Client, error) {
	var kubeConfig restclient.Config

	// Set the Kubernetes configuration based on the environment
	if RunningInCluster() {
		config, err := restclient.InClusterConfig()

		if err != nil {
			return nil, fmt.Errorf("Failed to create in-cluster config: %v.", err)
		}

		kubeConfig = *config
	} else {
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{}
		config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
		tmpKubeConfig, err := config.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("Failed to load local kube config: %v", err)
		}
		kubeConfig = *tmpKubeConfig;
	}


	// Create the Kubernetes client based on the configuration
	return client.New(&kubeConfig)
}
