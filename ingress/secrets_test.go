package ingress

import (
	"io/ioutil"
	"log"
	"testing"

	"github.com/30x/k8s-pods-ingress/kubernetes"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/watch"
)

func init() {
	log.SetOutput(ioutil.Discard)
}

/*
Test for github.com/30x/k8s-pods-ingress/ingress/secrets#GetIngressSecretList
*/
func TestGetIngressSecretList(t *testing.T) {
	kubeClient, err := kubernetes.GetClient()

	if err != nil {
		t.Fatalf("Failed to create k8s client: %v.", err)
	}

	secretList, err := GetIngressSecretList(kubeClient)

	if err != nil {
		t.Fatalf("Failed to get the ingress secrets: %v.", err)
	}

	for _, secret := range secretList.Items {
		if secret.Name != KeyIngressSecretName {
			t.Fatalf("Every secret should have a %s name", KeyIngressSecretName)
		}
	}
}

/*
Test for github.com/30x/k8s-pods-ingress/ingress/secrets#UpdateSecretCacheForEvents
*/
func TestUpdateSecretCacheForEvents(t *testing.T) {
	apiKeyStr := "API-Key"
	apiKey := []byte(apiKeyStr)
	cache := make(map[string]*api.Secret)
	namespace := "my-namespace"

	addedSecret := &api.Secret{
		ObjectMeta: api.ObjectMeta{
			Name:      KeyIngressSecretName,
			Namespace: "my-namespace",
		},
		Data: map[string][]byte{
			"api-key": apiKey,
		},
	}
	modifiedSecretNoRestart := &api.Secret{
		ObjectMeta: api.ObjectMeta{
			Name:      KeyIngressSecretName,
			Namespace: "my-namespace",
		},
		Data: map[string][]byte{
			"api-key": apiKey,
			"new-key": []byte("New-API-Key"),
		},
	}
	modifiedSecretRestart := &api.Secret{
		ObjectMeta: api.ObjectMeta{
			Name:      KeyIngressSecretName,
			Namespace: "my-namespace",
		},
		Data: map[string][]byte{
			"api-key": []byte("Updated-API-Key"),
		},
	}

	// Test add event
	needsRestart := UpdateSecretCacheForEvents(cache, []watch.Event{
		watch.Event{
			Type:   watch.Added,
			Object: addedSecret,
		},
	})

	if !needsRestart {
		t.Fatal("Server should require a restart")
	} else if _, ok := cache[namespace]; !ok {
		t.Fatal("Cache should reflect the added secret")
	}

	// Test modify event with unchanged api-key
	needsRestart = UpdateSecretCacheForEvents(cache, []watch.Event{
		watch.Event{
			Type:   watch.Modified,
			Object: modifiedSecretNoRestart,
		},
	})

	if needsRestart {
		t.Fatal("Server should not require a restart")
	}

	// Test modify event with changed api-key
	needsRestart = UpdateSecretCacheForEvents(cache, []watch.Event{
		watch.Event{
			Type:   watch.Modified,
			Object: modifiedSecretRestart,
		},
	})

	if !needsRestart {
		t.Fatal("Server should require a restart")
	}

	if apiKeyStr == string(cache[namespace].Data[KeyIngressAPIKey][:]) {
		t.Fatal("Cache should have the updated secret")
	}

	// Test delete event
	needsRestart = UpdateSecretCacheForEvents(cache, []watch.Event{
		watch.Event{
			Type:   watch.Deleted,
			Object: addedSecret,
		},
	})

	if !needsRestart {
		t.Fatal("Server should require a restart")
	} else if _, ok := cache[namespace]; ok {
		t.Fatal("Cache should not have the deleted secret")
	}
}