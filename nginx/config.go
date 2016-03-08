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

type nginxTemplateData struct {
	Servers   map[string]nginxServer
	Upstreams map[string]nginxUpstream
}

type nginxServer struct {
	Locations map[string]string
}

type nginxUpstream struct {
	Hash    string
	Servers []string
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

func init() {
	// Parse the nginx.conf template
	t, err := template.New("nginx").Parse(confTmpl)

	if err != nil {
		log.Fatalf("Failed to render nginx.conf template: %v.", err)
	}

	tmpl = t
}

/*
Takes the host to pod cache and returns a generated nginx configuration as string.
*/
func GetConfForPods(cache map[string]api.Pod) string {
	// Quick out if there are no pods in the cache
	if len(cache) == 0 {
		return DefaultNginxConf
	}

	tmplData := nginxTemplateData{
		Servers:   make(map[string]nginxServer),
		Upstreams: make(map[string]nginxUpstream),
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
		for _, host := range hosts {
			server, ok := tmplData.Servers[host]

			if !ok {
				tmplData.Servers[host] = nginxServer{
					Locations: make(map[string]string),
				}
				server = tmplData.Servers[host]
			}

			// Process each path
			for _, path := range paths {
				eTarget, ok := server.Locations[path]
				nTarget := pod.Status.PodIP + ":" + annotation
				upstreamKey := host + path
				upstreamHash := fmt.Sprint(hash(upstreamKey))

				if ok {
					// If the current target is different than the new one, create/update the upstream accordingly
					if eTarget != nTarget {
						if upstream, ok := tmplData.Upstreams[upstreamKey]; ok {
							// Add the new target to the upstream servers
							if !contains(upstream.Servers, nTarget) {
								upstream.Servers = append(upstream.Servers, nTarget)

								tmplData.Upstreams[upstreamKey] = upstream
							}
						} else {
							// Create the new upstream
							tmplData.Upstreams[upstreamKey] = nginxUpstream{
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

	return doc.String()
}
