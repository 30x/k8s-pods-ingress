package main

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"text/template"
	"time"

	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/watch"
)

const (
	keyMicroservices = "microservices"
	keyPathPort      = "pathPort"
	keyPublicPaths   = "publicPaths"
	keyTrafficHosts  = "trafficHosts"
	nginxConf        = `
events {
  worker_connections 1024;
}
http {
  # http://nginx.org/en/docs/http/ngx_http_core_module.html
  types_hash_max_size 2048;
  server_names_hash_max_size 512;
  server_names_hash_bucket_size 64;
{{range $name, $upstream := .Upstreams}} 
  # Upstream for {{$name}}
  upstream microservice{{$upstream.Hash}} {
{{range $server := $upstream.Servers}}    server {{$server}}
{{end}}  }
{{end}}
{{range $host, $server := .Servers}}  server {
    listen 80;
    server_name {{$host}};
{{range $path, $target := $server.Locations}}
    location {{$path}} {
      proxy_set_header Host $host;
      proxy_pass http://{{$target}};
    }
{{end}}
  }

{{end}}}
`
)

type NginxTemplateData struct {
	Servers   map[string]Server
	Upstreams map[string]Upstream
}

type Server struct {
	Locations map[string]string
}

type Upstream struct {
	Hash    string
	Servers []string
}

/*
Builds the nginx configuration and restarts nginx.
*/
func buildNginx(cache map[string]api.Pod, nginxConfPath string, tmpl template.Template) {
	tmplData := NginxTemplateData{
		Servers:   make(map[string]Server),
		Upstreams: make(map[string]Upstream),
	}

	// Process the pods to populate the nginx configuration data structure
	for _, pod := range cache {
		annotation, ok := pod.ObjectMeta.Annotations[keyTrafficHosts]

		// This pod does not have the trafficHosts annotation set
		if !ok {
			log.Printf("  Pod (%s) skipped: Missing 'trafficHosts' annotation\n", pod.Name)
			continue
		}

		hosts := strings.Split(annotation, " ")

		annotation, ok = pod.ObjectMeta.Annotations[keyPublicPaths]

		var paths []string

		// Use "/" as the default path when there are no paths defined
		if !ok {
			paths = []string{"/"}
		} else {
			paths = strings.Split(annotation, " ")
		}

		annotation, ok = pod.ObjectMeta.Annotations[keyPathPort]

		if !ok {
			annotation = "80"
		} else {
			_, err := strconv.Atoi(annotation)

			if err != nil {
				log.Printf("  Pod (%s) skipped: Invalid 'publicPort' value (%s): %v.\n",
					pod.Name, annotation, err)
				continue

			}
		}

		// Process each host
		for _, host := range hosts {
			server, ok := tmplData.Servers[host]

			if !ok {
				tmplData.Servers[host] = Server{
					Locations: make(map[string]string),
				}
				server = tmplData.Servers[host]
			}

			// Process each path
			for _, path := range paths {
				// This can happen when you have pods that are undeployed but not deleted
				if pod.Status.PodIP == "" {
					continue
				}

				eTarget, ok := server.Locations[path]
				nTarget := pod.Status.PodIP + ":" + annotation
				upstreamKey := host + path
				upstreamHash := fmt.Sprint(hash(upstreamKey))

				if ok {
					// If the current target is different than the new one, create/update the
					// upstream accordingly.
					if eTarget != nTarget {
						if upstream, ok := tmplData.Upstreams[upstreamKey]; ok {
							// Add the new target to the upstream servers
							if !contains(upstream.Servers, nTarget) {
								upstream.Servers = append(upstream.Servers, nTarget)

								tmplData.Upstreams[upstreamKey] = upstream
							}
						} else {
							// Create the new upstream
							tmplData.Upstreams[upstreamKey] = Upstream{
								Hash:    upstreamHash,
								Servers: []string{eTarget, nTarget},
							}
						}

						// Update the location to point to the upstream
						server.Locations[path] = "microservice" + upstreamHash
					}
				} else {
					server.Locations[path] = nTarget
				}
			}
		}
	}

	var doc bytes.Buffer

	// Useful for debugging
	if err := tmpl.Execute(&doc, tmplData); err != nil {
		log.Fatalf("Failed to write template %v", err)
	}

	log.Println("Generated nginx.conf")
	log.Println(doc.String())

	// Create the nginx.conf file based on the template
	if w, err := os.Create(nginxConfPath); err != nil {
		log.Fatalf("Failed to open %s: %v", nginxConfPath, err)
	} else if _, err := io.WriteString(w, doc.String()); err != nil {
		log.Fatalf("Failed to write template %v", err)
	}

	log.Printf("  Rebuilt %s\n", nginxConfPath)

	// Restart nginx
	shellOut("nginx -s reload")

	log.Println("  Restarted nginx")
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

/*
Run a command in a shell.
*/
func shellOut(cmd string) {
	// Do not start nginx if we're running outside the container
	if os.Getenv("KUBE_HOST") == "" {
		out, err := exec.Command("sh", "-c", cmd).CombinedOutput()

		if err != nil {
			log.Fatalf("Failed to execute %v: %v, err: %v", cmd, string(out), err)
		}

	}
}

/*
Simple Go application that will behave as a Kubernetes Ingress controller.  It does this by running nginx and updating
the nginx configuration based on pertinent Pod events.  To be considered for this controller the pod needs to have the
`microservice` label set to true.  Then the the pod needs to have annotations whose keys follow the following naming
convention:

  * trafficHosts: This is a space delimited list of public hosts that route to the pod(s)
  * publicPaths: This is the space delimited list of public paths that route to the pod(s)
  * pathPort: This is the pod port that the

This application is written to run inside the Kubernetes cluster but for outside of Kubernetes you can set the
`KUBE_HOST` and `NGINX_CONF` environment variables.
*/
func main() {
	var kubeConfig client.Config
	var nginxConfPath string

	// Set the Kubernetes configuration based on the environment
	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token"); err == nil {
		if config, err := client.InClusterConfig(); err != nil {
			log.Fatalf("Failed to create in-cluster config: %v.", err)
		} else {
			kubeConfig = *config
		}

		nginxConfPath = "/etc/nginx/nginx.conf"
	} else {
		kubeConfig = client.Config{
			Host: os.Getenv("KUBE_HOST"),
		}

		if kubeConfig.Host == "" {
			log.Fatal("When ran outside of Kubernetes, the KUBE_HOST environment variable is required")
		}

		nginxConfPath = os.Getenv("NGINX_CONF")

		if nginxConfPath == "" {
			log.Fatal("When ran outside of Kubernetes, the NGINX_CONF environment variable is required")
		}
	}

	// Create the Kubernetes client based on the configuration
	kubeClient, err := client.New(&kubeConfig)

	if err != nil {
		log.Fatalf("Failed to create client: %v.", err)
	} else {
		log.Println("k8s client created")
	}

	// Create the label selector for microservices
	selector, err := labels.Parse("microservice = true")

	if err != nil {
		log.Fatalf("Failed to create label selector: %v.", err)
	}

	// Create the Pod query/watch options
	options := api.ListOptions{
		FieldSelector: fields.Everything(),
		LabelSelector: selector,
	}

	// Query the initial list of Pods
	pods, err := kubeClient.Pods(api.NamespaceDefault).List(options)

	if err != nil {
		log.Fatalf("Failed to query the initial list of pods: %v.", err)
	}

	log.Printf("Microservices pods found: %d", len(pods.Items))

	// Create a cache which is a key/value pair where the key is the Pod name and the value is the Pod
	cache := make(map[string]api.Pod)

	for _, pod := range pods.Items {
		cache[pod.Name] = pod
	}

	// Start nginx with the default configuration to start nginx as a daemon
	shellOut("nginx")

	// Parse the nginx.conf template
	tmpl, _ := template.New("nginx").Parse(nginxConf)

	// Generate the nginx configuration
	buildNginx(cache, nginxConfPath, *tmpl)

	// Update the watch options with the resource version
	options.ResourceVersion = pods.ListMeta.ResourceVersion

	// Create a watcher to be notified of Pod events
	watcher, err := kubeClient.Pods(api.NamespaceAll).Watch(options)

	if err != nil {
		log.Fatalf("Failed to create pod watcher: %v.", err)
	}

	// Use a rate limiter that allows one query every 30 seconds
	// rateLimiter := util.NewTokenBucketRateLimiter(1.0/30.0, 1)
	rateLimiter := util.NewTokenBucketRateLimiter(0.1, 1)

	// Loop forever
	for {
		// Wait until the rate limiter allows
		rateLimiter.Accept()

		needsRebuild := false
		// TODO: Switch this logic to instead recreate the watch
		shouldTerminate := false

		log.Print("Checking for Pod events")

		for {
			hasMoreEvents := true

			select {
			case event, ok := <-watcher.ResultChan():
				if !ok {
					log.Printf("Failure to get pod event: %v\n", err)

					watcher.Stop()

					shouldTerminate = true
				} else {
					// Coerce the event target to a Pod
					pod := event.Object.(*api.Pod)

					switch event.Type {
					case watch.Added:
						log.Printf("  Pod added: %s", pod.Name)

						needsRebuild = true
						cache[pod.Name] = *pod

					case watch.Deleted:
						log.Printf("  Pod deleted: %s", pod.Name)

						needsRebuild = true
						delete(cache, pod.Name)

					case watch.Modified:
						log.Printf("  Pod updated: %s", pod.Name)

						// Check if the pod still has the microservice label
						if val, ok := pod.ObjectMeta.Labels["microservice"]; ok {
							if val != "true" {
								log.Print("    Pod is no longer a microservice")

								// Pod no longer the `microservices` label set to true
								// so we need to remove it from the cache
								needsRebuild = true
								delete(cache, pod.Name)
							} else {
								// If the annotations we're interested in change, rebuild
								if pod.Annotations[keyMicroservices] != cache[pod.Name].Annotations[keyMicroservices] ||
									pod.Annotations[keyTrafficHosts] != cache[pod.Name].Annotations[keyTrafficHosts] ||
									pod.Annotations[keyPublicPaths] != cache[pod.Name].Annotations[keyPublicPaths] ||
									pod.Annotations[keyPathPort] != cache[pod.Name].Annotations[keyPathPort] {
									needsRebuild = true
								}

								// Add/Update the cache entry
								cache[pod.Name] = *pod
							}
						} else {
							log.Print("    Pod is no longer a microservice")

							// Pod no longer has the `microservices` label so we need to
							// remove it from the cache
							needsRebuild = true
							delete(cache, pod.Name)
						}
					}
				}
			case <-time.After(5 * time.Second):
				// Time out after 5 seconds of inactivity
				hasMoreEvents = false
			}

			// Break this loop if we have no more events or if the watcher should stop
			if !hasMoreEvents || shouldTerminate {
				break
			}
		}

		// Break the watcher polling if there was an issue getting the pod events
		if shouldTerminate {
			break
		}

		log.Printf("  Pod count: %d\n", len(cache))

		// Go doesn't have a ternary operator so let's just do this the long way
		if needsRebuild {
			log.Println("  Needs rebuild: yes")

			// Rebuild and restart nginx
			buildNginx(cache, nginxConfPath, *tmpl)
		} else {
			log.Println("  Needs rebuild: no")
		}
	}
}
