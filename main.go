package main

import (
	"log"
	"time"

	"github.com/30x/k8s-pods-ingress/ingress"
	"github.com/30x/k8s-pods-ingress/kubernetes"
	"github.com/30x/k8s-pods-ingress/nginx"

	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/watch"
)

func initController(kubeClient *client.Client) (*ingress.Cache, watch.Interface, watch.Interface) {
	log.Println("Searching for microservices pods")

	// Query the initial list of Pods
	pods, err := ingress.GetMicroservicePodList(kubeClient)

	if err != nil {
		log.Fatalf("Failed to query the initial list of pods: %v.", err)
	}

	log.Printf("  Pods found: %d", len(pods.Items))

	// Create a cache to keep track of the ingress "API Keys" and Pods (with routes)
	cache := &ingress.Cache{
		Pods:    make(map[string]*ingress.PodWithRoutes),
		Secrets: make(map[string]*api.Secret),
	}

	// Turn the pods into a map based on the pod's name
	for _, pod := range pods.Items {
		cache.Pods[pod.Name] = &ingress.PodWithRoutes{
			Pod:    &pod,
			Routes: ingress.GetRoutes(&pod),
		}
	}

	// Query the initial list of Secrets
	secrets, err := ingress.GetIngressSecretList(kubeClient)

	// Turn the secrets into a map based on the secret's namespace
	for _, secret := range secrets.Items {
		cache.Secrets[secret.Namespace] = &secret
	}

	if err != nil {
		log.Fatalf("Failed to query the initial list of secrets: %v", err)
	}

	log.Printf("  Secrets found: %d", len(secrets.Items))

	// Generate the nginx configuration and restart nginx
	nginx.StartServer(nginx.GetConf(cache))

	// Get the list options so we can create the watch
	podWatchOptions := api.ListOptions{
		LabelSelector:   ingress.MicroserviceLabelSelector,
		ResourceVersion: pods.ListMeta.ResourceVersion,
	}

	// Create a watcher to be notified of Pod events
	podWatcher, err := kubeClient.Pods(api.NamespaceAll).Watch(podWatchOptions)

	if err != nil {
		log.Fatalf("Failed to create pod watcher: %v.", err)
	}

	// Get the list options so we can create the watch
	secretWatchOptions := api.ListOptions{
		ResourceVersion: pods.ListMeta.ResourceVersion,
	}

	// Create a watcher to be notified of Pod events
	secretWatcher, err := kubeClient.Secrets(api.NamespaceAll).Watch(secretWatchOptions)

	if err != nil {
		log.Fatalf("Failed to create secret watcher: %v.", err)
	}

	return cache, podWatcher, secretWatcher
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
	cache, podWatcher, secretWatcher := initController(kubeClient)

	// Loop forever
	for {
		var podEvents []watch.Event
		var secretEvents []watch.Event

		// Get a 2 seconds window worth of events
		for {
			doRestart := false
			doStop := false

			select {
			case event, ok := <-podWatcher.ResultChan():
				if !ok {
					log.Println("Kubernetes closed the pod watcher, restarting")

					doRestart = true
				} else {
					podEvents = append(podEvents, event)
				}

			case event, ok := <-secretWatcher.ResultChan():
				if !ok {
					log.Println("Kubernetes closed the secret watcher, restarting")

					doRestart = true
				} else {
					secret := event.Object.(*api.Secret)

					// Only record secret events for secrets with the name we are interested in
					if secret.Name == ingress.KeyIngressSecretName {
						secretEvents = append(secretEvents, event)
					}
				}

			// TODO: Rewrite to start the two seconds after the first post-restart event is seen
			case <-time.After(2 * time.Second):
				doStop = true
			}

			if doStop {
				break
			} else if doRestart {
				podWatcher.Stop()
				secretWatcher.Stop()

				cache, podWatcher, secretWatcher = initController(kubeClient)
			}
		}

		needsRestart := false

		if len(podEvents) > 0 {
			log.Printf("%d pod events found", len(podEvents))

			// Update the cache based on the events and check if the server needs to be restarted
			needsRestart = ingress.UpdatePodCacheForEvents(cache.Pods, podEvents)
		}

		if !needsRestart && len(secretEvents) > 0 {
			log.Printf("%d secret events found", len(secretEvents))

			// Update the cache based on the events and check if the server needs to be restarted
			needsRestart = ingress.UpdateSecretCacheForEvents(cache.Secrets, secretEvents)
		}

		// Wrapped in an if/else to limit logging
		if len(podEvents) > 0 || len(secretEvents) > 0 {
			if needsRestart {
				log.Println("  Requires nginx restart: yes")

				// Restart nginx
				nginx.StartServer(nginx.GetConf(cache))
			} else {
				log.Println("  Requires nginx restart: no")
			}
		}
	}
}
