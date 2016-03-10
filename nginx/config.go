package nginx

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"log"
	"strings"
	"text/template"

	"github.com/30x/k8s-pods-ingress/ingress"

	"k8s.io/kubernetes/pkg/api"
)

const (
	confTmpl = `
events {
  worker_connections 1024;
}
http {
  # http://nginx.org/en/docs/http/ngx_http_core_module.html
  types_hash_max_size 2048;
  server_names_hash_max_size 512;
  server_names_hash_bucket_size 64;
{{range $key, $upstream := .Upstreams}}
  # Upstream for {{$upstream.Path}} traffic on {{$upstream.Host}}
  upstream {{$upstream.Name}} {
{{range $server := $upstream.Servers}}    # Pod {{$server.PodName}}
    server {{$server.Target}}
{{end}}  }
{{end}}{{range $host, $server := .Hosts}}
  server {
    listen 80;
    server_name {{$host}};
{{range $path, $location := $server.Locations}}
    location {{$path}} {
      proxy_set_header Host $host;
      {{if $location.Server.IsUpstream}}# Upstream {{$location.Server.Target}}{{else}}# Pod {{$location.Server.PodName}}{{end}}
      proxy_pass http://{{$location.Server.Target}};
    }
{{end}}  }
{{end}}}
`
	// DefaultNginxConf is the default nginx.conf content
	DefaultNginxConf = `
# A very simple nginx configuration file that forces nginx to start as a daemon.
events {}
http {}
daemon on;
`
	// NginxConfPath is The nginx configuration file path
	NginxConfPath = "/etc/nginx/nginx.conf"
)

// Cannot declare as a constant
var tmpl *template.Template

type hostT struct {
	Locations map[string]*locationT
}

type locationT struct {
	Path   string
	Server *serverT
}

type serverT struct {
	IsUpstream bool
	PodName    string
	Target     string
}

type templateDataT struct {
	Hosts     map[string]*hostT
	Upstreams map[string]*upstreamT
}

type upstreamT struct {
	Host    string
	Name    string
	Path    string
	Servers []*serverT
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func init() {
	// Parse the nginx.conf template
	t, err := template.New("nginx").Parse(confTmpl)

	if err != nil {
		log.Fatalf("Failed to render nginx.conf template: %v.", err)
	}

	tmpl = t
}

/*
GetConfForPods takes the pod cache and returns a generated nginx configuration
*/
func GetConfForPods(cache map[string]api.Pod) string {
	// Quick out if there are no pods in the cache
	if len(cache) == 0 {
		return DefaultNginxConf
	}

	tmplData := templateDataT{
		Hosts:     make(map[string]*hostT),
		Upstreams: make(map[string]*upstreamT),
	}

	// Process the pods to populate the nginx configuration data structure
	for _, pod := range cache {
		// We do not need to validate the pod's publicPort, trafficHosts or state since that's already handled for us
		annotation, _ := pod.ObjectMeta.Annotations[ingress.KeyTrafficHostsA]

		hosts := strings.Split(annotation, " ")

		annotation, ok := pod.ObjectMeta.Annotations[ingress.KeyPublicPathsA]

		var paths []string

		// Use "/" as the default path when there are no paths defined
		if !ok {
			paths = []string{"/"}
		} else {
			paths = strings.Split(annotation, " ")
		}

		annotation, ok = pod.ObjectMeta.Annotations[ingress.KeyPathPortA]

		if !ok {
			annotation = "80"
		}

		// Process each host
		for _, hostName := range hosts {
			host, ok := tmplData.Hosts[hostName]

			if !ok {
				tmplData.Hosts[hostName] = &hostT{
					Locations: make(map[string]*locationT),
				}
				host = tmplData.Hosts[hostName]
			}

			// Process each path
			for _, path := range paths {
				location, ok := host.Locations[path]
				target := pod.Status.PodIP + ":" + annotation
				upstreamKey := hostName + path
				upstreamHash := fmt.Sprint(hash(upstreamKey))
				upstreamName := "microservice" + upstreamHash

				if ok {
					// If the current target is different than the new one, create/update the upstream accordingly
					if location.Server.Target != target {
						if upstream, ok := tmplData.Upstreams[upstreamKey]; ok {
							ok = true

							// Check to see if there is a server with the corresponding target
							for _, server := range upstream.Servers {
								if server.Target == target {
									ok = false
									break
								}
							}

							// If there is no server for this target, create one
							if ok {
								upstream.Servers = append(upstream.Servers, &serverT{
									PodName: pod.Name,
									Target:  target,
								})
							}
						} else {
							// Create the new upstream
							tmplData.Upstreams[upstreamKey] = &upstreamT{
								Name: upstreamName,
								Host: hostName,
								Path: path,
								Servers: []*serverT{
									location.Server,
									&serverT{
										PodName: pod.Name,
										Target:  target,
									},
								},
							}
						}

						// Update the location server
						location.Server = &serverT{
							IsUpstream: true,
							Target:     upstreamName,
						}
					}
				} else {
					host.Locations[path] = &locationT{
						Path: path,
						Server: &serverT{
							PodName: pod.Name,
							Target:  target,
						},
					}
				}
			}
		}
	}

	var doc bytes.Buffer

	// Useful for debugging
	if err := tmpl.Execute(&doc, tmplData); err != nil {
		log.Fatalf("Failed to write template %v", err)
	}

	return doc.String()
}
