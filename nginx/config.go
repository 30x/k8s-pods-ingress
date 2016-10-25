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

package nginx

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"log"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/30x/k8s-router/router"
)

const (
	defaultNginxConfTmpl = `
# A very simple nginx configuration file that forces nginx to start as a daemon.
events {}
http {` + defaultNginxServerConfTmpl + `}
daemon on;
`
	defaultNginxServerConfTmpl = `
  # Default server that will just close the connection as if there was no server available
  server {
    listen {{.Port}} default_server;
    return 444;
  }
`
	defaultNginxLocationTmpl = `
    # Here to avoid returning the nginx welcome page for servers that do not have a "/" location.  (Issue #35)
    location / {
      return 404;
    }
`
	httpConfPreambleTmpl = `
  # http://nginx.org/en/docs/http/ngx_http_core_module.html
  types_hash_max_size 2048;
  server_names_hash_max_size 512;
  server_names_hash_bucket_size 64;

  # Force HTTP 1.1 for upstream requests
  proxy_http_version 1.1;

  # When nginx proxies to an upstream, the default value used for 'Connection' is 'close'.  We use this variable to do
  # the same thing so that whenever a 'Connection' header is in the request, the variable reflects the provided value
  # otherwise, it defaults to 'close'.  This is opposed to just using "proxy_set_header Connection $http_connection"
  # which would remove the 'Connection' header from the upstream request whenever the request does not contain a
  # 'Connection' header, which is a deviation from the nginx norm.
  map $http_connection $p_connection {
    default $http_connection;
    ''      close;
  }

  # Pass through the appropriate headers
  proxy_set_header Connection $p_connection;
  proxy_set_header Host $http_host;
  proxy_set_header Upgrade $http_upgrade;
`
	nginxConfTmpl = `
events {
  worker_connections 1024;
}
http {` + httpConfPreambleTmpl + `{{range $key, $upstream := .Upstreams}}
  # Upstream for {{$upstream.Path}} traffic on {{$upstream.Host}}
  upstream {{$upstream.Name}} {
{{range $server := $upstream.Servers}}    # Pod {{$server.Pod.Name}} (namespace: {{$server.Pod.Namespace}})
    server {{$server.Target}};
{{end}}  }
{{end}}{{range $host, $server := .Hosts}}
  server {
    listen {{$.Port}};
    server_name {{$host}};
{{if $server.NeedsDefaultLocation}}` + defaultNginxLocationTmpl + `{{end}}{{range $path, $location := $server.Locations}}
    location {{$path}} {
      {{if ne $location.Secret ""}}# Check the Routing API Key (namespace: {{$location.Namespace}})
      if ($http_{{$.APIKeyHeader}} != "{{$location.Secret}}") {
        return 403;
      }

      {{end}}{{if $location.Server.IsUpstream}}# Upstream {{$location.Server.Target}}{{else}}# Pod {{$location.Server.Pod.Name}} (namespace: {{$location.Server.Pod.Namespace}}){{end}}
      proxy_pass http://{{$location.Server.Target}};
    }
{{end}}  }
{{end}}` + defaultNginxServerConfTmpl + `}
`
	// NginxConfPath is The nginx configuration file path
	NginxConfPath = "/etc/nginx/nginx.conf"
)

// Cannot declare as a constant
var defaultNginxConf string
var defaultNginxConfTemplate *template.Template
var nginxAPIKeyHeader string
var nginxConfTemplate *template.Template

type hostT struct {
	Locations            map[string]*locationT
	NeedsDefaultLocation bool
}

type locationT struct {
	Namespace string
	Path      string
	Secret    string
	Server    *serverT
}

type serverT struct {
	IsUpstream bool
	Pod        *router.PodWithRoutes
	Target     string
}

type serversT []*serverT

type templateDataT struct {
	APIKeyHeader string
	Hosts        map[string]*hostT
	Port         int
	Upstreams    map[string]*upstreamT
}

type upstreamT struct {
	Host    string
	Name    string
	Path    string
	Servers serversT
}

func (slice serversT) Len() int {
	return len(slice)
}

func (slice serversT) Less(i, j int) bool {
	return slice[i].Pod.Name < slice[j].Pod.Name
}

func (slice serversT) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func convertAPIKeyHeaderForNginx(config *router.Config) {
	if nginxAPIKeyHeader == "" {
		// Convert the API Key header to nginx
		nginxAPIKeyHeader = strings.ToLower(regexp.MustCompile("[^A-Za-z0-9]").ReplaceAllString(config.APIKeyHeader, "_"))
	}
}

func init() {
	// Parse the default nginx.conf template
	t, err := template.New("nginx-default").Parse(defaultNginxConfTmpl)

	if err != nil {
		log.Fatalf("Failed to render default nginx.conf template: %v.", err)
	}

	defaultNginxConfTemplate = t

	// Parse the nginx.conf template
	t2, err := template.New("nginx").Parse(nginxConfTmpl)

	if err != nil {
		log.Fatalf("Failed to render nginx.conf template: %v.", err)
	}

	nginxConfTemplate = t2
}

/*
GetConf takes the router cache and returns a generated nginx configuration
*/
func GetConf(config *router.Config, cache *router.Cache) string {
	// Quick out if there are no pods in the cache
	if len(cache.Pods) == 0 {
		return GetDefaultConf(config)
	}

	// Make sure we've converted the API Key to nginx format
	convertAPIKeyHeaderForNginx(config)

	tmplData := templateDataT{
		APIKeyHeader: nginxAPIKeyHeader,
		Hosts:        make(map[string]*hostT),
		Port:         config.Port,
		Upstreams:    make(map[string]*upstreamT),
	}

	// Process the pods to populate the nginx configuration data structure
	for _, cacheEntry := range cache.Pods {
		// Process each pod route
		for _, route := range cacheEntry.Routes {
			host, ok := tmplData.Hosts[route.Incoming.Host]

			if !ok {
				tmplData.Hosts[route.Incoming.Host] = &hostT{
					Locations:            make(map[string]*locationT),
					NeedsDefaultLocation: true,
				}
				host = tmplData.Hosts[route.Incoming.Host]
			}

			var locationSecret string
			namespace := cacheEntry.Namespace
			secret, ok := cache.Secrets[namespace]

			if ok {
				// There is guaranteed to be an API Key so no need to double check
				locationSecret = base64.StdEncoding.EncodeToString(secret.Data[config.APIKeySecretDataField])
			}

			location, ok := host.Locations[route.Incoming.Path]
			upstreamKey := route.Incoming.Host + route.Incoming.Path
			upstreamHash := fmt.Sprint(hash(upstreamKey))
			upstreamName := "upstream" + upstreamHash
			target := route.Outgoing.IP

			if route.Outgoing.Port != "80" && route.Outgoing.Port != "443" {
				target += ":" + route.Outgoing.Port
			}

			// Unset the need for a default location if necessary
			if host.NeedsDefaultLocation && route.Incoming.Path == "/" {
				host.NeedsDefaultLocation = false
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
								Pod:    cacheEntry,
								Target: target,
							})

							// Sort to make finding your pods in an upstream easier
							sort.Sort(upstream.Servers)
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
									Pod:    cacheEntry,
									Target: target,
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
					Namespace: namespace,
					Path:      route.Incoming.Path,
					Secret:    locationSecret,
					Server: &serverT{
						Pod:    cacheEntry,
						Target: target,
					},
				}
			}
		}
	}

	var doc bytes.Buffer

	// Useful for debugging
	if err := nginxConfTemplate.Execute(&doc, tmplData); err != nil {
		log.Fatalf("Failed to write template %v", err)
	}

	return doc.String()
}

/*
GetDefaultConf returns the default nginx.conf
*/
func GetDefaultConf(config *router.Config) string {
	// Make sure we've converted the API Key to nginx format
	convertAPIKeyHeaderForNginx(config)

	if defaultNginxConf == "" {
		var doc bytes.Buffer

		if err := defaultNginxConfTemplate.Execute(&doc, config); err != nil {
			log.Fatalf("Failed to write template %v", err)
		} else {
			defaultNginxConf = doc.String()
		}
	}

	return defaultNginxConf
}
