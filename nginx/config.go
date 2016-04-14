package nginx

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"log"
	"text/template"

	"github.com/30x/k8s-pods-ingress/ingress"
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
    server {{$server.Target}};
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
{{end}}` + DefaultNginxServerConf + `}
`
	// DefaultNginxConf is the default nginx.conf content
	DefaultNginxConf = `
# A very simple nginx configuration file that forces nginx to start as a daemon.
events {}
http {` + DefaultNginxServerConf + `}
daemon on;
`
	// DefaultNginxServerConf is the default nginx server configuration
	DefaultNginxServerConf = `
  # Default server that will just close the connection as if there was no server available
  server {
    listen 80 default_server;
    return 444;
  }
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
func GetConfForPods(cache map[string]*ingress.PodWithRoutes) string {
	// Quick out if there are no pods in the cache
	if len(cache) == 0 {
		return DefaultNginxConf
	}

	tmplData := templateDataT{
		Hosts:     make(map[string]*hostT),
		Upstreams: make(map[string]*upstreamT),
	}

	// Process the pods to populate the nginx configuration data structure
	for _, cacheEntry := range cache {
		// Process each pod route
		for _, route := range cacheEntry.Routes {
			host, ok := tmplData.Hosts[route.Incoming.Host]

			if !ok {
				tmplData.Hosts[route.Incoming.Host] = &hostT{
					Locations: make(map[string]*locationT),
				}
				host = tmplData.Hosts[route.Incoming.Host]
			}

			location, ok := host.Locations[route.Incoming.Path]
			upstreamKey := route.Incoming.Host + route.Incoming.Path
			upstreamHash := fmt.Sprint(hash(upstreamKey))
			upstreamName := "microservice" + upstreamHash
			target := route.Outgoing.IP

			if route.Outgoing.Port != "80" && route.Outgoing.Port != "443" {
				target += ":" + route.Outgoing.Port
			}

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
								PodName: cacheEntry.Pod.Name,
								Target:  target,
							})
						}
					} else {
						// Create the new upstream
						tmplData.Upstreams[upstreamKey] = &upstreamT{
							Name: upstreamName,
							Host: route.Incoming.Host,
							Path: route.Incoming.Path,
							Servers: []*serverT{
								location.Server,
								&serverT{
									PodName: cacheEntry.Pod.Name,
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
				host.Locations[route.Incoming.Path] = &locationT{
					Path: route.Incoming.Path,
					Server: &serverT{
						PodName: cacheEntry.Pod.Name,
						Target:  target,
					},
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
