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

package router

import (
	"log"

	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/watch"
)

func ConvertSecretToModel(config *Config, secret *api.Secret) ([]byte) {
	apikey, _ := secret.Data[config.APIKeySecretDataField]
	return apikey
}
/*
GetRouterSecretList returns the router secrets.
*/
func GetRouterSecretList(config *Config, kubeClient *client.Client) (*api.SecretList, error) {
	// Query all secrets
	secretList, err := kubeClient.Secrets(api.NamespaceAll).List(api.ListOptions{})

	if err != nil {
		return nil, err
	}

	// Filter out the secrets that are not router API Key secrets or that do not have the proper secret key
	var filtered []api.Secret

	for _, secret := range secretList.Items {
		if secret.Name == config.APIKeySecret {
			_, ok := secret.Data[config.APIKeySecretDataField]

			if ok {
				filtered = append(filtered, secret)
			} else {
				log.Printf("    Router secret for namespace (%s) is not usable: Missing '%s' key\n", secret.Namespace, config.APIKeySecretDataField)
			}
		}
	}

	secretList.Items = filtered

	return secretList, nil
}

/*
UpdateSecretCacheForEvents updates the cache based on the secret events and returns if the changes warrant an nginx restart.
*/
func UpdateSecretCacheForEvents(config *Config, cache map[string][]byte, events []watch.Event) bool {
	needsRestart := false

	for _, event := range events {
		secret := event.Object.(*api.Secret)
		namespace := secret.Namespace

		log.Printf("  Secret (%s in %s namespace) event: %s\n", secret.Name, secret.Namespace, event.Type)

		// Process the event
		switch event.Type {
		case watch.Added:
			cache[namespace] = ConvertSecretToModel(config, secret)
			needsRestart = true

		case watch.Deleted:
			delete(cache, namespace)
			needsRestart = true

		case watch.Modified:
			cachedAPIKey, ok := cache[namespace]
			apiKey := ConvertSecretToModel(config, secret)

			if ok {

				if (apiKey == nil && cachedAPIKey != nil) || (apiKey != nil && cachedAPIKey == nil) {
					needsRestart = true
				} else if apiKey != nil && cachedAPIKey != nil && len(apiKey) != len(cachedAPIKey) {
					needsRestart = true
				} else {
					for i := range apiKey {
						if apiKey[i] != cachedAPIKey[i] {
							needsRestart = true

							break
						}
					}
				}
			}

			cache[namespace] = apiKey
		}

		if _, ok := cache[namespace]; ok {
			apiKey := ConvertSecretToModel(config, secret)

			if apiKey == nil {
				log.Printf("    Secret has an %s value: no\n", config.APIKeySecretDataField)
			} else {
				log.Printf("    Secret has an %s value: yes\n", config.APIKeySecretDataField)
			}
		}
	}

	return needsRestart
}
