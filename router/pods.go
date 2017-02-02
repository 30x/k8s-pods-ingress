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
	"hash/fnv"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/30x/k8s-router/utils"

	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"
)

const (
	hostnameRegexStr    = "^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\\-]*[a-zA-Z0-9])\\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\\-]*[A-Za-z0-9])$"
	ipRegexStr          = "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$"
	pathSegmentRegexStr = `^([A-Za-z0-9\-._~!$&'()*+,;=:@]+|%[0-9A-Fa-f]{2})+$`
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
	// Compile all regular expressions
	hostnameRegex = compileRegex(hostnameRegexStr)
	ipRegex = compileRegex(ipRegexStr)
	pathSegmentRegex = compileRegex(pathSegmentRegexStr)
}

func isContainerPort(ports []int32, port int32) bool {
	for _, vPort := range ports {
		if vPort == port {
			return true
		}
	}
	return false
}

/*
GetRoutablePodList returns the routable pods list.
*/
func GetRoutablePodList(config *Config, kubeClient *client.Client) (*api.PodList, error) {
	// Query the initial list of Pods
	podList, err := kubeClient.Pods(api.NamespaceAll).List(api.ListOptions{
		FieldSelector: fields.Everything(),
		LabelSelector: config.RoutableLabelSelector,
	})

	if err != nil {
		return nil, err
	}

	return podList, nil
}

/*
 Calculate hash for hosts and paths annotations to compare when pod is modified
*/
func calculateAnnotationHash(config *Config, pod *api.Pod) uint64 {
	h := fnv.New64()
	h.Write([]byte(pod.Annotations[config.HostsAnnotation]))
	h.Write([]byte(pod.Annotations[config.PathsAnnotation]))
	return h.Sum64()
}

/*
 Converts a Kubernetes pod model to our model
*/
func ConvertPodToModel(config *Config, pod *api.Pod) *PodWithRoutes {
	return &PodWithRoutes{
		Name:           pod.Name,
		Namespace:      pod.Namespace,
		Status:         pod.Status.Phase,
		AnnotationHash: calculateAnnotationHash(config, pod),
		Routes:         GetRoutes(config, pod),
	}
}

/*
GetRoutes returns an array of routes defined within the provided pod
*/
func GetRoutes(config *Config, pod *api.Pod) []*Route {
	var routes []*Route

	// Do not process pods that are not running
	if pod.Status.Phase == api.PodRunning {
		// Do not process pods without an IP
		if pod.Status.PodIP != "" {
			var hosts []string
			var pathPairs []*pathPair
			var ports []int32

			annotation, ok := pod.Annotations[config.HostsAnnotation]

			// This pod does not have the hosts annotation set
			if ok {
				// Process the routing hosts
				for _, host := range strings.Split(annotation, " ") {
					valid := hostnameRegex.MatchString(host)

					if !valid {
						valid = ipRegex.MatchString(host)

						if !valid {
							log.Printf("    Pod (%s) routing issue: %s (%s) is not a valid hostname/ip\n", pod.Name, config.HostsAnnotation, host)

							continue
						}
					}

					// Record the host
					hosts = append(hosts, host)
				}

				// Do not process the routing paths if there are no valid hosts
				if len(hosts) > 0 {
					annotation, ok = pod.Annotations[config.PathsAnnotation]

					// Create a list of valid routing ports
					for _, container := range pod.Spec.Containers {
						for _, port := range container.Ports {
							ports = append(ports, port.ContainerPort)
						}
					}

					if ok {
						for _, publicPath := range strings.Split(annotation, " ") {
							pathParts := strings.Split(publicPath, ":")

							if len(pathParts) == 2 {
								cPathPair := &pathPair{}

								// Validate the port
								port, err := strconv.Atoi(pathParts[0])

								if err != nil || !utils.IsValidPort(port) {
									log.Printf("    Pod (%s) routing issue: %s port (%s) is not valid\n", pod.Name, config.PathsAnnotation, pathParts[0])
								} else if !isContainerPort(ports, int32(port)) {
									log.Printf("    Pod (%s) routing issue: %s port (%s) is not an exposed container port\n", pod.Name, config.PathsAnnotation, pathParts[0])
								} else {
									cPathPair.Port = pathParts[0]
								}

								// Validate the path (when necessary)
								if port > 0 {
									if strings.HasPrefix(pathParts[1], "/") {
										pathSegments := strings.Split(pathParts[1], "/")
										valid := true

										for i, pathSegment := range pathSegments {
											// Skip the first and last entry
											if (i == 0 || i == len(pathSegments)-1) && pathSegment == "" {
												continue
											} else if !pathSegmentRegex.MatchString(pathSegment) {
												log.Printf("    Pod (%s) routing issue: publicPath path (%s) is not valid\n", pod.Name, pathParts[1])

												valid = false

												break
											}
										}

										if valid {
											cPathPair.Path = pathParts[1]
										}
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
						log.Printf("    Pod (%s) is not routable: Missing '%s' annotation\n", pod.Name, config.PathsAnnotation)
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
				log.Printf("    Pod (%s) is not routable: Missing '%s' annotation\n", pod.Name, config.HostsAnnotation)
			}
		} else {
			log.Printf("    Pod (%s) is not routable: Pod does not have an IP\n", pod.Name)
		}
	} else {
		log.Printf("    Pod (%s) is not routable: Not running (%s)\n", pod.Name, pod.Status.Phase)
	}

	return routes
}

/*
UpdatePodCacheForEvents updates the cache based on the pod events and returns if the changes warrant an nginx restart.
*/
func UpdatePodCacheForEvents(config *Config, cache map[string]*PodWithRoutes, events []watch.Event) bool {
	needsRestart := false

	for _, event := range events {
		pod := event.Object.(*api.Pod)

		log.Printf("  Pod (%s) event: %s\n", pod.Name, event.Type)

		// Process the event
		switch event.Type {
		case watch.Added:
			// This event is likely never going to be handled in the real world because most pod add events happen prior to
			// pod being routable but it's here just in case.
			cache[pod.Name] = ConvertPodToModel(config, pod)

			needsRestart = len(cache[pod.Name].Routes) > 0

		case watch.Deleted:
			needsRestart = true
			delete(cache, pod.Name)

		case watch.Modified:
			podLabels := labels.Set(pod.Labels)

			// Check if the pod still has the routable label
			if config.RoutableLabelSelector.Matches(podLabels) {
				cached, ok := cache[pod.Name]

				// If anything routing related changes, trigger a server restart
				if !ok || calculateAnnotationHash(config, pod) != cached.AnnotationHash || pod.Status.Phase != cached.Status {
					needsRestart = true
				}

				// Add/Update the cache entry
				cache[pod.Name] = ConvertPodToModel(config, pod)
			} else {
				log.Println("    Pod is no longer routable")

				// Pod no longer matches the routable label selector so we need to remove it from the cache
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
