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

func validateRoutes(t *testing.T, desc string, expected, actual []*Route) {
	aCount := 0
	eCount := 0

	if actual != nil {
		aCount = len(actual)
	}

	if expected != nil {
		eCount = len(expected)
	}

	// First check that we have the proper number of routes
	if aCount != eCount {
		t.Fatalf("Expected %d routes but found %d routes: %s\n", eCount, aCount, desc)
	}

	// Validate each route positionally
	find := func(items []*Route, item *Route) *Route {
		var route *Route

		for _, cRoute := range items {
			if item.Incoming.Host == cRoute.Incoming.Host &&
				item.Incoming.Path == cRoute.Incoming.Path &&
				item.Outgoing.IP == cRoute.Outgoing.IP &&
				item.Outgoing.Port == cRoute.Outgoing.Port {
				route = cRoute

				break
			}
		}

		return route
	}

	for _, route := range expected {
		if find(actual, route) == nil {
			t.Fatalf("Unable to find route (%s): %s\n", route, desc)
		}
	}
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
Test for github.com/30x/k8s-pods-ingress/ingress/pods#GetRoutes where the pod is not running
*/
func TestGetRoutesNotRunning(t *testing.T) {
	validateRoutes(t, "pod not running", []*Route{}, GetRoutes(&api.Pod{
		Status: api.PodStatus{
			Phase: api.PodPending,
		},
	}))
}

/*
Test for github.com/30x/k8s-pods-ingress/ingress/pods#GetRoutes where the pod has no trafficHosts annotation
*/
func TestGetRoutesNoTrafficHosts(t *testing.T) {
	validateRoutes(t, "pod has no trafficHosts annotation", []*Route{}, GetRoutes(&api.Pod{
		Status: api.PodStatus{
			Phase: api.PodRunning,
		},
	}))
}

/*
Test for github.com/30x/k8s-pods-ingress/ingress/pods#GetRoutes where the pod has an invalid trafficHosts annotation
*/
func TestGetRoutesInvalidTrafficHosts(t *testing.T) {
	validateRoutes(t, "pod has an invalid trafficHosts host", []*Route{}, GetRoutes(&api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				"trafficHosts": "test.github.com test.",
			},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
		},
	}))
}

/*
Test for github.com/30x/k8s-pods-ingress/ingress/pods#GetRoutes where the pod has an invalid port value in the publicPaths annotation
*/
func TestGetRoutesInvalidPublicPathsPort(t *testing.T) {
	// Not a valid integer
	validateRoutes(t, "pod has an invalid publicPaths port (invalid integer)", []*Route{}, GetRoutes(&api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				"trafficHosts": "test.github.com",
				"publicPaths":  "abcdef:/",
			},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
		},
	}))

	// Port is less than 0
	validateRoutes(t, "pod has an invalid publicPaths port (port < 0)", []*Route{}, GetRoutes(&api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				"trafficHosts": "test.github.com",
				"publicPaths":  "-1:/",
			},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
		},
	}))

	// Port is greater than 65535
	validateRoutes(t, "pod has an invalid publicPaths port (port > 65536)", []*Route{}, GetRoutes(&api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				"trafficHosts": "test.github.com",
				"publicPaths":  "77777:/",
			},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
		},
	}))
}

/*
Test for github.com/30x/k8s-pods-ingress/ingress/pods#GetRoutes where the pod has an invalid path value in the publicPaths annotation
*/
func TestGetRoutesInvalidPublicPathsPath(t *testing.T) {
	// "%ZZ" is not a valid path segment
	validateRoutes(t, "pod has an invalid publicPaths path", []*Route{}, GetRoutes(&api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				"trafficHosts": "test.github.com",
				"publicPaths":  "3000:/people/%ZZ",
			},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
		},
	}))
}

/*
Test for github.com/30x/k8s-pods-ingress/ingress/pods#GetRoutes where the pod has no publicPaths annotation
*/
func TestGetRoutesValidPods(t *testing.T) {
	host1 := "test.github.com"
	host2 := "www.github.com"
	ip := "10.244.1.17"
	path1 := "/"
	path2 := "/admin"
	port1 := "3000"
	port2 := "3001"

	// A single host and path
	validateRoutes(t, "single host and path", []*Route{
		&Route{
			Incoming: &Incoming{
				Host: host1,
				Path: path1,
			},
			Outgoing: &Outgoing{
				IP:   ip,
				Port: port1,
			},
		},
	}, GetRoutes(&api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				"trafficHosts": host1,
				"publicPaths":  port1 + ":" + path1,
			},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
			PodIP: ip,
		},
	}))

	// A single host and multiple paths
	validateRoutes(t, "single host and multiple paths", []*Route{
		&Route{
			Incoming: &Incoming{
				Host: host1,
				Path: path1,
			},
			Outgoing: &Outgoing{
				IP:   ip,
				Port: port1,
			},
		},
		&Route{
			Incoming: &Incoming{
				Host: host1,
				Path: path2,
			},
			Outgoing: &Outgoing{
				IP:   ip,
				Port: port2,
			},
		},
	}, GetRoutes(&api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				"trafficHosts": host1,
				"publicPaths":  port1 + ":" + path1 + " " + port2 + ":" + path2,
			},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
			PodIP: ip,
		},
	}))

	// Multiple hosts and single path
	validateRoutes(t, "multiple hosts and single path", []*Route{
		&Route{
			Incoming: &Incoming{
				Host: host1,
				Path: path1,
			},
			Outgoing: &Outgoing{
				IP:   ip,
				Port: port1,
			},
		},
		&Route{
			Incoming: &Incoming{
				Host: host2,
				Path: path1,
			},
			Outgoing: &Outgoing{
				IP:   ip,
				Port: port1,
			},
		},
	}, GetRoutes(&api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				"trafficHosts": host1 + " " + host2,
				"publicPaths":  port1 + ":" + path1,
			},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
			PodIP: ip,
		},
	}))

	// Multiple hosts and multiple paths
	validateRoutes(t, "multiple hosts and multiple paths", []*Route{
		&Route{
			Incoming: &Incoming{
				Host: host1,
				Path: path1,
			},
			Outgoing: &Outgoing{
				IP:   ip,
				Port: port1,
			},
		},
		&Route{
			Incoming: &Incoming{
				Host: host1,
				Path: path2,
			},
			Outgoing: &Outgoing{
				IP:   ip,
				Port: port2,
			},
		},
		&Route{
			Incoming: &Incoming{
				Host: host2,
				Path: path1,
			},
			Outgoing: &Outgoing{
				IP:   ip,
				Port: port1,
			},
		},
		&Route{
			Incoming: &Incoming{
				Host: host2,
				Path: path2,
			},
			Outgoing: &Outgoing{
				IP:   ip,
				Port: port2,
			},
		},
	}, GetRoutes(&api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				"trafficHosts": host1 + " " + host2,
				"publicPaths":  port1 + ":" + path1 + " " + port2 + ":" + path2,
			},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
			PodIP: ip,
		},
	}))
}

/*
Test for github.com/30x/k8s-pods-ingress/ingress/pods#UpdatePodCacheForEvents
*/
func TestUpdatePodCacheForEvents(t *testing.T) {
	annotations := map[string]string{
		"trafficHosts": "test.github.com",
		"publicPaths":  "80:/",
	}
	cache := map[string]*PodWithRoutes{}
	labels := map[string]string{
		"microservice": "true",
	}
	podName := "test-pod"

	modifiedPodMicroserviceFalse := &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Labels: map[string]string{
				"microservice": "false",
			},
			Name: podName,
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
			PodIP: "10.244.1.17",
		},
	}
	modifiedPodWithRoutes := &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: annotations,
			Labels:      labels,
			Name:        podName,
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
			PodIP: "10.244.1.17",
		},
	}
	unroutablePod := &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: annotations,
			Labels:      labels,
			Name:        podName,
		},
		Status: api.PodStatus{
			Phase: api.PodPending,
		},
	}

	// Test adding an unroutable pod
	needsRestart := UpdatePodCacheForEvents(cache, []watch.Event{
		watch.Event{
			Type:   watch.Added,
			Object: unroutablePod,
		},
	})

	if needsRestart {
		t.Fatal("Server should not need a restart")
	} else if _, ok := cache[podName]; !ok {
		t.Fatal("Cache should reflect the added pod")
	}

	// Test modifying a pod to make it routable
	needsRestart = UpdatePodCacheForEvents(cache, []watch.Event{
		watch.Event{
			Type:   watch.Modified,
			Object: modifiedPodWithRoutes,
		},
	})

	if !needsRestart {
		t.Fatal("Server should need a restart")
	}

	// Test modifying a pod that does not change routes
	needsRestart = UpdatePodCacheForEvents(cache, []watch.Event{
		watch.Event{
			Type:   watch.Modified,
			Object: modifiedPodWithRoutes,
		},
	})

	if needsRestart {
		t.Fatal("Server should not need a restart")
	}

	// Test modifying a pod to set the microservice label to false
	needsRestart = UpdatePodCacheForEvents(cache, []watch.Event{
		watch.Event{
			Type:   watch.Modified,
			Object: modifiedPodMicroserviceFalse,
		},
	})

	if !needsRestart {
		t.Fatal("Server should need a restart")
	} else if len(cache) > 0 {
		t.Fatal("Cache should reflect the modified (but removed) pod")
	}

	// Test modifying a pod to remove its microservice label
	_ = UpdatePodCacheForEvents(cache, []watch.Event{
		watch.Event{
			Type:   watch.Added,
			Object: modifiedPodWithRoutes,
		},
	})

	if len(cache) != 1 {
		t.Fatal("There was an issue updating the cache")
	}

	needsRestart = UpdatePodCacheForEvents(cache, []watch.Event{
		watch.Event{
			Type:   watch.Modified,
			Object: modifiedPodMicroserviceFalse,
		},
	})

	if !needsRestart {
		t.Fatal("Server should need a restart")
	} else if len(cache) > 0 {
		t.Fatal("Cache should reflect the modified (but removed) pod")
	}

	// Test deleting a pod
	_ = UpdatePodCacheForEvents(cache, []watch.Event{
		watch.Event{
			Type:   watch.Added,
			Object: modifiedPodWithRoutes,
		},
	})

	if len(cache) != 1 {
		t.Fatal("There was an issue updating the cache")
	}

	needsRestart = UpdatePodCacheForEvents(cache, []watch.Event{
		watch.Event{
			Type:   watch.Deleted,
			Object: modifiedPodWithRoutes,
		},
	})

	if !needsRestart {
		t.Fatal("Server should need a restart")
	} else if len(cache) > 0 {
		t.Fatal("Cache should reflect the deleted pod")
	}
}
