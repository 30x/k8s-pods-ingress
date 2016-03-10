package ingress

import (
	"log"
	"strconv"

	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"
)

const (
	// KeyMicroserviceL is the label used to identify microservices
	KeyMicroserviceL = "microservice"
	// KeyPathPortA is the annotation used to identify the pod port used for the microservice
	KeyPathPortA = "pathPort"
	// KeyPublicPathsA is the annotation used to identify the list of traffic paths associated with the microservice
	KeyPublicPathsA = "publicPaths"
	// KeyTrafficHostsA is the annotation used to identify the list of traffic hosts associated with the microservice
	KeyTrafficHostsA = "trafficHosts"
)

var MicroserviceLabelSelector labels.Selector

func filterPods(pods []api.Pod) []api.Pod {
	var filtered []api.Pod

	for _, pod := range pods {
		if IsPodRoutable(pod) {
			filtered = append(filtered, pod)
		}
	}

	return filtered
}

func init() {
	// Create the label selector for microservices
	selector, err := labels.Parse("microservice = true")

	if err != nil {
		log.Fatalf("Failed to create label selector: %v.", err)
	}

	MicroserviceLabelSelector = selector
}

/*
GetMicroservicePodList returns the microservices pods list.
*/
func GetMicroservicePodList(kubeClient *client.Client) (*api.PodList, error) {
	// Query the initial list of Pods
	podList, err := kubeClient.Pods(api.NamespaceDefault).List(api.ListOptions{
		FieldSelector: fields.Everything(),
		LabelSelector: MicroserviceLabelSelector,
	})

	if err != nil {
		return nil, err
	}

	// Filter the pods
	podList.Items = filterPods(podList.Items)

	return podList, nil
}

/*
IsPodRoutable returns whether or not the pod is routable.
*/
func IsPodRoutable(pod api.Pod) bool {
	routable := true

	// Do not process pods that are not running
	if pod.Status.Phase != api.PodRunning {
		log.Printf("  Pod (%s) is not routable: Not running (%s)\n", pod.Name, pod.Status.Phase)
		routable = false
	}

	if routable {
		_, ok := pod.ObjectMeta.Annotations[KeyTrafficHostsA]

		// This pod does not have the trafficHosts annotation set
		if !ok {
			log.Printf("  Pod (%s) is not routable: Missing '%s' annotation\n", pod.Name, KeyTrafficHostsA)
			routable = false
		}
	}

	if routable {
		annotation, ok := pod.ObjectMeta.Annotations[KeyPathPortA]

		if ok {
			_, err := strconv.Atoi(annotation)

			if err != nil {
				log.Printf("  Pod (%s) is not routable: Invalid '%s' value (%s): %v.\n",
					pod.Name, KeyPathPortA, annotation, err)
				routable = false
			}
		}
	}

	return routable
}

/*
Updates the provided cache based on the pod events and returns if the cache changes warrant an nginx restart.
*/
func UpdatePodCacheForEvents(cache map[string]api.Pod, events []watch.Event) bool {
	needsRestart := false

	for _, event := range events {
		// Coerce the event target to a Pod
		pod := event.Object.(*api.Pod)

		// Quick return if the pod is not routable
		if !IsPodRoutable(*pod) {
			needsRestart = true
			delete(cache, pod.Name)
			continue
		}

		// Process the event
		switch event.Type {
		case watch.Added:
			log.Printf("  Pod added: %s", pod.Name)

			needsRestart = true
			cache[pod.Name] = *pod

		case watch.Deleted:
			log.Printf("  Pod deleted: %s", pod.Name)

			needsRestart = true
			delete(cache, pod.Name)

		case watch.Modified:
			log.Printf("  Pod updated: %s", pod.Name)

			// Check if the pod still has the microservice label
			if val, ok := pod.ObjectMeta.Labels[KeyMicroserviceL]; ok {
				if val != "true" {
					log.Print("    Pod is no longer a microservice")

					// Pod no longer the `microservices` label set to true
					// so we need to remove it from the cache
					needsRestart = true
					delete(cache, pod.Name)
				} else {
					// If the annotations we're interested in change, rebuild
					if pod.Annotations[KeyMicroserviceL] != cache[pod.Name].Annotations[KeyMicroserviceL] ||
						pod.Annotations[KeyTrafficHostsA] != cache[pod.Name].Annotations[KeyTrafficHostsA] ||
						pod.Annotations[KeyPublicPathsA] != cache[pod.Name].Annotations[KeyPublicPathsA] ||
						pod.Annotations[KeyPathPortA] != cache[pod.Name].Annotations[KeyPathPortA] {
						needsRestart = true
					}

					// Add/Update the cache entry
					cache[pod.Name] = *pod
				}
			} else {
				log.Print("    Pod is no longer a microservice")

				// Pod no longer has the `microservices` label so we need to
				// remove it from the cache
				needsRestart = true
				delete(cache, pod.Name)
			}
		}
	}

	return needsRestart
}
