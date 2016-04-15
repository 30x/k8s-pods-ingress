package main

import (
	"log"
	"time"

	"github.com/30x/k8s-pods-ingress/ingress"
	"github.com/30x/k8s-pods-ingress/kubernetes"
	"github.com/30x/k8s-pods-ingress/nginx"

	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/watch"
)

func initController(kubeClient *client.Client) (map[string]*ingress.PodWithRoutes, watch.Interface) {
	log.Println("Searching for microservices pods")

	// Query the initial list of Pods
	pods, err := ingress.GetMicroservicePodList(kubeClient)

	if err != nil {
		log.Fatalf("Failed to query the initial list of pods: %v.", err)
	}

	log.Printf("  Pods found: %d", len(pods.Items))

	// Create a cache which is a key/value pair where the key is the Pod name and the value is the Pod
	cache := make(map[string]*ingress.PodWithRoutes)

	for _, pod := range pods.Items {
		cache[pod.Name] = &ingress.PodWithRoutes{
			Pod:    &pod,
			Routes: ingress.GetRoutes(&pod),
		}
	}

	// Generate the nginx configuration and restart nginx
	nginx.StartServer(nginx.GetConfForPods(cache))

	// Get the list options so we can create the watch
	watchOptions := api.ListOptions{
		FieldSelector:   fields.Everything(),
		LabelSelector:   ingress.MicroserviceLabelSelector,
		ResourceVersion: pods.ListMeta.ResourceVersion,
	}

	// Create a watcher to be notified of Pod events
	watcher, err := kubeClient.Pods(api.NamespaceAll).Watch(watchOptions)

	log.Println("Watching pods for changes")

	if err != nil {
		log.Fatalf("Failed to create pod watcher: %v.", err)
	}

	return cache, watcher
}

/*
Simple Go application that will behave as a Kubernetes Ingress controller.  It does this by running nginx and updating
the nginx configuration based on pertinent Pod events.  To be considered for this controller the pod needs to have the
`microservice` label set to true.  Then the the pod needs to have annotations whose keys follow the following naming
convention:

  * trafficHosts: This is a space delimited list of public hosts that route to the pod(s)
  * publicPaths: This is the space delimited list of `{CONTAINER_PORT}:{PATH}` combinations that define the path routing

This application is written to run inside the Kubernetes cluster but for outside of Kubernetes you can set the
`KUBE_HOST` environment variable to run in a mock mode.
*/
func main() {
	log.Println("Starting the Kubernetes Pods-based Ingress")

	// Create the Kubernetes Client
	kubeClient, err := kubernetes.GetClient()

	if err != nil {
		log.Fatalf("Failed to create client: %v.", err)
	}

	// Start nginx with the default configuration to start nginx as a daemon
	nginx.StartServer("")

	// Create the initial cache and watcher
	cache, watcher := initController(kubeClient)

	// Loop forever
	for {
		var events []watch.Event

		// Get a 2 seconds window worth of events
		for {
			doStop := false

			select {
			case event, ok := <-watcher.ResultChan():
				if !ok {
					log.Println("Kubernetes closed the watcher, restarting")

					watcher.Stop()

					cache, watcher = initController(kubeClient)
				} else {
					events = append(events, event)
				}

			// TODO: Rewrite to start the two seconds after the first post-restart event is seen
			case <-time.After(2 * time.Second):
				doStop = true
			}

			if doStop {
				break
			}
		}

		if len(events) > 0 {
			log.Printf("%d events found", len(events))

			// Update the cache based on the events and check if the server needs to be restarted
			needsRestart := ingress.UpdatePodCacheForEvents(cache, events)

			if needsRestart {
				log.Println("  Requires nginx restart: yes")

				// Restart nginx
				nginx.StartServer(nginx.GetConfForPods(cache))
			} else {
				log.Println("  Requires nginx restart: no")
			}
		}
	}
}
