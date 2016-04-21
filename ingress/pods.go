package ingress

import (
	"log"
	"strconv"

	"regexp"
	"strings"

	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"
)

const (
	// KeyMicroserviceL is the label used to identify microservices
	KeyMicroserviceL = "microservice"
	// KeyPublicPathsA is the annotation used to identify the list of traffic paths associated with the microservice
	KeyPublicPathsA = "publicPaths"
	// KeyTrafficHostsA is the annotation used to identify the list of traffic hosts associated with the microservice
	KeyTrafficHostsA    = "trafficHosts"
	hostnameRegexStr    = "^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\\-]*[a-zA-Z0-9])\\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\\-]*[A-Za-z0-9])$"
	ipRegexStr          = "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$"
	pathSegmentRegexStr = "^[A-Za-z0-9\\-._~!$&'()*+,;=:@]|%[0-9A-Fa-f]{2}$"
)

type pathPair struct {
	Path string
	Port string
}

/*
String implements the Stringer interface
*/
func (r *Route) String() string {
	return r.Incoming.Host + r.Incoming.Path + " -> " + r.Outgoing.IP + ":" + r.Outgoing.Port
}

/*
MicroserviceLabelSelector is the label selector to identify microservice pods.
*/
var MicroserviceLabelSelector labels.Selector
var hostnameRegex *regexp.Regexp
var ipRegex *regexp.Regexp
var pathSegmentRegex *regexp.Regexp

func compileRegex(regexStr string) *regexp.Regexp {
	compiled, err := regexp.Compile(regexStr)

	if err != nil {
		log.Fatalf("Failed to compile regular expression (%s): %v\n", regexStr, err)
	}

	return compiled
}

func init() {
	// Create the label selector for microservices
	selector, err := labels.Parse("microservice = true")

	if err != nil {
		log.Fatalf("Failed to create label selector: %v.", err)
	}

	MicroserviceLabelSelector = selector

	// Compile all regular expressions
	hostnameRegex = compileRegex(hostnameRegexStr)
	ipRegex = compileRegex(ipRegexStr)
	pathSegmentRegex = compileRegex(pathSegmentRegexStr)
}

/*
GetMicroservicePodList returns the microservices pods list.
*/
func GetMicroservicePodList(kubeClient *client.Client) (*api.PodList, error) {
	// Query the initial list of Pods
	podList, err := kubeClient.Pods(api.NamespaceAll).List(api.ListOptions{
		FieldSelector: fields.Everything(),
		LabelSelector: MicroserviceLabelSelector,
	})

	if err != nil {
		return nil, err
	}

	return podList, nil
}

/*
GetRoutes returns an array of routes defined within the provided pod
*/
func GetRoutes(pod *api.Pod) []*Route {
	var routes []*Route

	// Do not process pods that are not running
	if pod.Status.Phase == api.PodRunning {
		var hosts []string
		var pathPairs []*pathPair

		annotation, ok := pod.Annotations[KeyTrafficHostsA]

		// This pod does not have the trafficHosts annotation set
		if ok {
			// Process the routing hosts
			for _, host := range strings.Split(annotation, " ") {
				valid := hostnameRegex.MatchString(host)

				if !valid {
					valid = ipRegex.MatchString(host)

					if !valid {
						log.Printf("    Pod (%s) routing issue: trafficHost (%s) is not a valid hostname/ip\n", pod.Name, host)

						continue
					}
				}

				// Record the host
				hosts = append(hosts, host)
			}

			// Do not process the routing paths if there are no valid hosts
			if len(hosts) > 0 {
				annotation, ok = pod.Annotations[KeyPublicPathsA]

				if ok {
					for _, publicPath := range strings.Split(annotation, " ") {
						pathParts := strings.Split(publicPath, ":")

						if len(pathParts) == 2 {
							cPathPair := &pathPair{}

							// Validate the port
							port, err := strconv.Atoi(pathParts[0])

							if err == nil && port > 0 && port < 65536 {
								cPathPair.Port = pathParts[0]
							} else {
								log.Printf("    Pod (%s) routing issue: publicPath port (%s) is not valid\n", pod.Name, pathParts[0])
							}

							// Validate the path (when necessary)
							if port > 0 {
								pathSegments := strings.Split(pathParts[1], "/")
								valid := true

								for i, pathSegment := range pathSegments {
									// Skip the first and last entry
									if (i == 0 || i == len(pathParts)-1) && pathSegment == "" {
										continue
									} else if !pathSegmentRegex.MatchString(pathSegment) {
										log.Printf("    Pod (%s) routing issue: publicPath path (%s) is not a valid\n", pod.Name, pathParts[0])

										valid = false

										break
									}
								}

								if valid {
									cPathPair.Path = pathParts[1]
								}
							}

							if cPathPair.Path != "" && cPathPair.Port != "" {
								pathPairs = append(pathPairs, cPathPair)
							}
						} else {
							log.Printf("    Pod (%s) routing issue: publicPath (%s) is not a valid PORT:PATH combination\n", pod.Name, annotation)
						}
					}
				} else {
					log.Printf("    Pod (%s) is not routable: Missing '%s' annotation\n", pod.Name, KeyPublicPathsA)
				}
			}

			// Turn the hosts and path pairs into routes
			if hosts != nil && pathPairs != nil {
				for _, host := range hosts {
					for _, cPathPair := range pathPairs {
						routes = append(routes, &Route{
							Incoming: &Incoming{
								Host: host,
								Path: cPathPair.Path,
							},
							Outgoing: &Outgoing{
								IP:   pod.Status.PodIP,
								Port: cPathPair.Port,
							},
						})
					}
				}
			}
		} else {
			log.Printf("    Pod (%s) is not routable: Missing '%s' annotation\n", pod.Name, KeyTrafficHostsA)
		}
	} else {
		log.Printf("    Pod (%s) is not routable: Not running (%s)\n", pod.Name, pod.Status.Phase)
	}

	return routes
}

/*
UpdatePodCacheForEvents updates the cache based on the pod events and returns if the changes warrant an nginx restart.
*/
func UpdatePodCacheForEvents(cache map[string]*PodWithRoutes, events []watch.Event) bool {
	needsRestart := false

	for _, event := range events {
		// Coerce the event target to a Pod
		pod := event.Object.(*api.Pod)

		log.Printf("  Pod (%s) event: %s\n", pod.Name, event.Type)

		// Process the event
		switch event.Type {
		case watch.Added:
			// This event is likely never going to be handled in the real world because most pod add events happen prior to
			// pod being routable but it's here just in case.
			needsRestart = true
			cache[pod.Name] = &PodWithRoutes{
				Pod:    pod,
				Routes: GetRoutes(pod),
			}

		case watch.Deleted:
			needsRestart = true
			delete(cache, pod.Name)

		case watch.Modified:
			// Check if the pod still has the microservice label
			if val, ok := pod.Labels[KeyMicroserviceL]; ok {
				if val != "true" {
					log.Println("    Pod is no longer a microservice")

					// Pod no longer the `microservices` label set to true
					// so we need to remove it from the cache
					needsRestart = true
					delete(cache, pod.Name)
				} else {
					cached, ok := cache[pod.Name]

					// If the annotations we're interested in change or if there is no cache entry, rebuild
					if !ok ||
						pod.Annotations[KeyMicroserviceL] != cached.Pod.Annotations[KeyMicroserviceL] ||
						pod.Annotations[KeyTrafficHostsA] != cached.Pod.Annotations[KeyTrafficHostsA] ||
						pod.Annotations[KeyPublicPathsA] != cached.Pod.Annotations[KeyPublicPathsA] {
						needsRestart = true
					}

					// Add/Update the cache entry
					cache[pod.Name].Pod = pod
					cache[pod.Name].Routes = GetRoutes(pod)
				}
			} else {
				log.Println("    Pod is no longer a microservice")

				// Pod no longer has the `microservices` label so we need to
				// remove it from the cache
				needsRestart = true
				delete(cache, pod.Name)
			}
		}

		cacheEntry, ok := cache[pod.Name]

		if ok {
			if len(cacheEntry.Routes) > 0 {
				log.Println("    Pod is routable")
			} else {
				log.Println("    Pod is not routable")
			}
		}
	}

	return needsRestart
}
