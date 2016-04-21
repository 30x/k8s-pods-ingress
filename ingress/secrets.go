package ingress

import (
	"log"

	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/watch"
)

const (
	// KeyIngressSecretName is the name of the secret to identify as an ingress secret
	KeyIngressSecretName = "ingress"
	// KeyIngressAPIKey is the name of the secret data to use as the API Key
	KeyIngressAPIKey = "api-key"
)

/*
GetIngressSecretList returns the ingress secrets.
*/
func GetIngressSecretList(kubeClient *client.Client) (*api.SecretList, error) {
	// Query all secrets
	secretList, err := kubeClient.Secrets(api.NamespaceAll).List(api.ListOptions{})

	if err != nil {
		return nil, err
	}

	// Filter out the secrets that are not ingress API Key secrets or that do not have the proper secret key
	var filtered []api.Secret

	for _, secret := range secretList.Items {
		if secret.Name == KeyIngressSecretName {
			_, ok := secret.Data[KeyIngressAPIKey]

			if ok {
				filtered = append(filtered, secret)
			} else {
				log.Printf("    Ingress secret for namespace (%s) is not usable: Missing '%s' key\n", secret.Namespace, KeyIngressAPIKey)
			}
		}
	}

	secretList.Items = filtered

	return secretList, nil
}

/*
UpdateSecretCacheForEvents updates the cache based on the secret events and returns if the changes warrant an nginx restart.
*/
func UpdateSecretCacheForEvents(cache map[string]*api.Secret, events []watch.Event) bool {
	needsRestart := false

	for _, event := range events {
		// Coerce the event target to a Secret
		secret := event.Object.(*api.Secret)
		namespace := secret.Namespace

		log.Printf("  Secret (%s in %s namespace) event: %s\n", secret.Name, secret.Namespace, event.Type)

		// Process the event
		switch event.Type {
		case watch.Added:
			cache[namespace] = secret
			needsRestart = true

		case watch.Deleted:
			delete(cache, namespace)
			needsRestart = true

		case watch.Modified:
			cached, ok := cache[namespace]
			apiKey, _ := secret.Data[KeyIngressAPIKey]

			if ok {
				cachedAPIKey, _ := cached.Data[KeyIngressAPIKey]

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

			cache[namespace] = secret
		}

		if _, ok := cache[namespace]; ok {
			apiKey, _ := secret.Data[KeyIngressAPIKey]

			if apiKey == nil {
				log.Printf("    Secret has an %s value: no\n", KeyIngressAPIKey)
			} else {
				log.Printf("    Secret has an %s value: yes\n", KeyIngressAPIKey)
			}
		}
	}

	return needsRestart
}
