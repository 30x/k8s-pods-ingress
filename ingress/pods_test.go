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
Test for github.com/30x/k8s-pods-ingress/ingress/pods#GetMicroservicePodList
*/
func TestGetMicroservicePodList(t *testing.T) {
	kubeClient, err := kubernetes.GetClient()

	if err != nil {
		t.Fatalf("Failed to create k8s client: %v.", err)
	}

	podsList, err := GetMicroservicePodList(kubeClient)

	if err != nil {
		t.Fatalf("Failed to get the microservices pods: %v.", err)
	}

	for _, pod := range podsList.Items {
		val, ok := pod.Labels[KeyMicroserviceL]

		if !ok {
			t.Fatalf("Every pod should have a %s label", KeyMicroserviceL)
		}

		if val != "true" {
			t.Fatalf("Every pod's %s label should be set to \"true\"", KeyMicroserviceL)
		}
	}
}

/*
Test for github.com/30x/k8s-pods-ingress/ingress/pods#IsPodRoutable where pod has an invalid pathPort annotation
*/
func TestIsPodRoutableInvalidPathPort(t *testing.T) {
	if IsPodRoutable(api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				"trafficHosts": "test.github.com",
				"pathPort":     "invalid",
			},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
		},
	}) {
		t.Fatal("Pod has an invalid pathPort annotation so it is not routable")
	}
}

/*
Test for github.com/30x/k8s-pods-ingress/ingress/pods#IsPodRoutable where pod has no trafficHosts annotation
*/
func TestIsPodRoutableNoTrafficHosts(t *testing.T) {
	if IsPodRoutable(api.Pod{
		Status: api.PodStatus{
			Phase: api.PodRunning,
		},
	}) {
		t.Fatal("Pod has no trafficHosts annotation so it is not routable")
	}
}

/*
Test for github.com/30x/k8s-pods-ingress/ingress/pods#IsPodRoutable where pod has an invalid trafficHosts annotation
*/
func TestIsPodRoutableInvalidTrafficHosts(t *testing.T) {
	if IsPodRoutable(api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				"trafficHosts": "test.github.com test.",
				"pathPort":     "invalid",
			},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
		},
	}) {
		t.Fatal("Pod has an invalid trafficHosts annotation so it is not routable")
	}
}

/*
Test for github.com/30x/k8s-pods-ingress/ingress/pods#IsPodRoutable where pod is not running
*/
func TestIsPodRoutableNotRunning(t *testing.T) {
	if IsPodRoutable(api.Pod{
		Status: api.PodStatus{
			Phase: api.PodPending,
		},
	}) {
		t.Fatal("Pod is not running so it is not routable")
	}
}

/*
Test for github.com/30x/k8s-pods-ingress/ingress/pods#IsPodRoutable where pod is valid and should be routable
*/
func TestIsPodRoutableValidPod(t *testing.T) {
	if !IsPodRoutable(api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				"trafficHosts": "test.github.com",
			},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
		},
	}) {
		t.Fatal("Pod is valid and should be routable")
	}
}

/*
Test for github.com/30x/k8s-pods-ingress/ingress/pods#UpdatePodCacheForEvents
*/
func TestUpdatePodCacheForEvents(t *testing.T) {
	annotations := map[string]string{
		"trafficHosts": "test.github.com",
	}
	labels := map[string]string{
		"microservice": "true",
	}
	addedPod := api.Pod{
		ObjectMeta: api.ObjectMeta{
			Name:        "added",
			Annotations: annotations,
			Labels:      labels,
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
			PodIP: "10.244.1.17",
		},
	}
	deletedPod := api.Pod{
		ObjectMeta: api.ObjectMeta{
			Name:        "deleted",
			Annotations: annotations,
			Labels:      labels,
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
			PodIP: "10.244.1.18",
		},
	}
	modifiedPod1 := api.Pod{
		ObjectMeta: api.ObjectMeta{
			Name:        "modifiedPod1",
			Annotations: annotations,
			Labels:      labels,
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
			PodIP: "10.244.1.19",
		},
	}
	modifiedPod2 := api.Pod{
		ObjectMeta: api.ObjectMeta{
			Name:        "modifiedPod2",
			Annotations: annotations,
			Labels:      labels,
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
			PodIP: "10.244.1.20",
		},
	}
	modifiedPod3 := api.Pod{
		ObjectMeta: api.ObjectMeta{
			Name:        "modifiedPod3",
			Annotations: annotations,
			Labels:      labels,
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
			PodIP: "10.244.1.21",
		},
	}
	unroutablePod := api.Pod{
		ObjectMeta: api.ObjectMeta{
			Name:   "unroutable",
			Labels: labels,
		},
		Status: api.PodStatus{
			Phase: api.PodPending,
		},
	}
	cache := map[string]api.Pod{
		"deleted":      deletedPod,
		"modifiedPod1": modifiedPod1,
		"modifiedPod2": modifiedPod2,
		"modifiedPod3": modifiedPod3,
	}
	events := []watch.Event{
		// Added but unroutable so it should not be in the cache
		watch.Event{
			Type:   watch.Added,
			Object: &unroutablePod,
		},
		// Added and routable so it should be in the cache
		watch.Event{
			Type:   watch.Added,
			Object: &addedPod,
		},
		// Deleted and should be removed fromt he cache
		watch.Event{
			Type:   watch.Deleted,
			Object: &deletedPod,
		},
		// Modified and missing the microservice label so it should not be in the cache
		watch.Event{
			Type: watch.Modified,
			Object: &api.Pod{
				ObjectMeta: api.ObjectMeta{
					Name:        "modifiedPod1",
					Annotations: annotations,
				},
				Status: api.PodStatus{
					Phase: api.PodRunning,
					PodIP: "10.244.1.19",
				},
			},
		},
		// Modified and the microservice label is set to false so it should not be in the cache
		watch.Event{
			Type: watch.Modified,
			Object: &api.Pod{
				ObjectMeta: api.ObjectMeta{
					Name:        "modifiedPod2",
					Annotations: annotations,
					Labels: map[string]string{
						"microservice": "false",
					},
				},
				Status: api.PodStatus{
					Phase: api.PodRunning,
					PodIP: "10.244.1.20",
				},
			},
		},
		// Modified and routable so it should be in the cache
		watch.Event{
			Type: watch.Modified,
			Object: &api.Pod{
				ObjectMeta: api.ObjectMeta{
					Name: "modifiedPod3",
					Annotations: map[string]string{
						"trafficHosts": "prod.github.com",
						"publicPaths":  "/v1/api",
					},
					Labels: labels,
				},
				Status: api.PodStatus{
					Phase: api.PodRunning,
					PodIP: "10.244.1.21",
				},
			},
		},
	}

	needsRestart := UpdatePodCacheForEvents(cache, events)

	if !needsRestart {
		t.Fatal("The server should need a restart")
	}

	if _, ok := cache["added"]; !ok {
		t.Fatal("Cache should include the \"added\" pod")
	}

	for _, name := range []string{"deleted", "modifiedPod1", "modifiedPod2"} {
		if _, ok := cache[name]; ok {
			t.Fatalf("Cache should include the \"%s\" pod", name)
		}
	}

	if _, ok := cache["modifiedPod3"].Annotations["publicPaths"]; !ok {
		t.Fatalf("The \"modifiedPod3\" \"publicPaths\" annotation should had been updated")
	}
}
