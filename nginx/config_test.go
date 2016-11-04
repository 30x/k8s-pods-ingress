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
	"log"
	"strings"
	"testing"
	"text/template"

	"github.com/30x/k8s-router/router"

	"k8s.io/kubernetes/pkg/api"
)

var config *router.Config

func init() {
	envConfig, err := router.ConfigFromEnv()

	if err != nil {
		log.Fatalf("Unable to get configuration from environment: %v", err)
	}

	config = envConfig
}

func getDefaultServerConf(config *router.Config) string {
	var doc bytes.Buffer

	// Parse the default nginx server block template
	t, err := template.New("nginx-default-server").Parse(defaultNginxServerConfTmpl)

	if err != nil {
		log.Fatalf("Failed to render nginx.conf default server block template: %v.", err)
	}

	if err := t.Execute(&doc, config); err != nil {
		log.Fatalf("Failed to write template %v", err)

		return ""
	}

	return doc.String()
}

func getConfPreamble(config *router.Config) string {
	var doc bytes.Buffer

	// Parse the default nginx server block template
	t, err := template.New("nginx-http-preamble").Parse(httpConfPreambleTmpl)

	if err != nil {
		log.Fatalf("Failed to render nginx.conf http preamble template: %v.", err)
	}

	data := struct {
		Config *router.Config
	}{
		config,
	}

	if err := t.Execute(&doc, data); err != nil {
		log.Fatalf("Failed to write template %v", err)

		return ""
	}

	return doc.String()
}

func resetConf() {
	// Reset the cached default server (At runtime, we cache the results because they will never change)
	defaultNginxConf = ""
	// Change the config port
	config.Port = 80
	// Reset the cached API Key header (At runtime, we cache the results because they will never change)
	nginxAPIKeyHeader = ""
  // Reset enable nginx health check
  config.EnableNginxUpstreamCheckModule = false
}

func validateConf(t *testing.T, desc, expected string, pods []*api.Pod, secrets []*api.Secret) {
	cache := &router.Cache{
		Pods:    make(map[string]*router.PodWithRoutes),
		Secrets: make(map[string][]byte),
	}

	for _, pod := range pods {
		cache.Pods[pod.Name] = router.ConvertPodToModel(config, pod)
	}

	for _, secret := range secrets {
		cache.Secrets[secret.Namespace] = router.ConvertSecretToModel(config, secret)
	}

	actual := GetConf(config, cache)

	if expected != actual {
		expectedArr := strings.Split(expected, "\n")
		actualArr := strings.Split(actual, "\n")
		for i, line := range expectedArr {
			if line != actualArr[i] {
				t.Fatalf("Failed at line (%d): Expected=%s\n Actual=%s\n", i, line, actualArr[i])
			}
		}
		t.Fatalf("Unexpected nginx.conf was generated (%s)\nExpected: %s\n\nActual: %s\n", desc, expected, actual)
	}
}

/*
Test for github.com/30x/k8s-router/nginx/config#GetConf with an empty cache
*/
func TestGetConfNoRoutablePods(t *testing.T) {
	conf := GetConf(config, &router.Cache{})

	if conf != `
# A very simple nginx configuration file that forces nginx to start as a daemon.
events {}
http {
  # Default server that will just close the connection as if there was no server available
  server {
    listen 80 default_server;
    return 444;
  }
}
daemon on;
` {
		t.Fatal("The default nginx.conf should be returned for an empty cache")
	}
}

/*
Test for github.com/30x/k8s-router/nginx/config#GetConf with an empty cache and a custom port
*/
func TestGetConfNoRoutablePodsCustomPort(t *testing.T) {
	resetConf()

	// Change the config port
	config.Port = 90

	conf := GetConf(config, &router.Cache{})

	if conf != `
# A very simple nginx configuration file that forces nginx to start as a daemon.
events {}
http {
  # Default server that will just close the connection as if there was no server available
  server {
    listen 90 default_server;
    return 444;
  }
}
daemon on;
` {
		t.Fatal("The default nginx.conf should be returned for an empty cache and a custom port")
	}

	resetConf()
}

/*
Test for github.com/30x/k8s-router/nginx/config#GetConf with single pod and multiple paths
*/
func TestGetConfMultiplePaths(t *testing.T) {
	expectedConf := `
events {
  worker_connections 1024;
}
http {` + getConfPreamble(config) + `
  # Upstream for /prod traffic on test.github.com
  upstream upstream2256221875 {
    # Pod testing (namespace: testing)
    server 10.244.1.16;

  }

  # Upstream for /test traffic on test.github.com
  upstream upstream999700326 {
    # Pod testing (namespace: testing)
    server 10.244.1.16:3000;

  }

  server {
    listen 80;
    server_name test.github.com;

    # Here to avoid returning the nginx welcome page for servers that do not have a "/" location.  (Issue #35)
    location / {
      return 404;
    }

    location /prod {
      # Upstream upstream2256221875
      proxy_pass http://upstream2256221875;
    }

    location /test {
      # Upstream upstream999700326
      proxy_pass http://upstream999700326;
    }
  }
` + getDefaultServerConf(config) + `}
`

	pod := api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				"routingHosts": "test.github.com",
				"routingPaths": "80:/prod 3000:/test",
			},
			Name:      "testing",
			Namespace: "testing",
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				api.Container{
					Ports: []api.ContainerPort{
						api.ContainerPort{
							ContainerPort: int32(80),
						},
						api.ContainerPort{
							ContainerPort: int32(3000),
						},
					},
				},
			},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
			PodIP: "10.244.1.16",
		},
	}

	validateConf(t, "single pod multiple paths", expectedConf, []*api.Pod{&pod}, []*api.Secret{})
}

/*
Test for github.com/30x/k8s-router/nginx/config#GetConf with single pod, multiple paths and a custom port
*/
func TestGetConfMultiplePathsCustomPort(t *testing.T) {
	resetConf()

	// Change the config port
	config.Port = 90

	expectedConf := `
events {
  worker_connections 1024;
}
http {` + getConfPreamble(config) + `
  # Upstream for /prod traffic on test.github.com
  upstream upstream2256221875 {
    # Pod testing (namespace: testing)
    server 10.244.1.16;

  }

  # Upstream for /test traffic on test.github.com
  upstream upstream999700326 {
    # Pod testing (namespace: testing)
    server 10.244.1.16:3000;

  }

  server {
    listen 90;
    server_name test.github.com;

    # Here to avoid returning the nginx welcome page for servers that do not have a "/" location.  (Issue #35)
    location / {
      return 404;
    }

    location /prod {
      # Upstream upstream2256221875
      proxy_pass http://upstream2256221875;
    }

    location /test {
      # Upstream upstream999700326
      proxy_pass http://upstream999700326;
    }
  }
` + getDefaultServerConf(config) + `}
`
	pod := api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				"routingHosts": "test.github.com",
				"routingPaths": "80:/prod 3000:/test",
			},
			Name:      "testing",
			Namespace: "testing",
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				api.Container{
					Ports: []api.ContainerPort{
						api.ContainerPort{
							ContainerPort: int32(80),
						},
						api.ContainerPort{
							ContainerPort: int32(3000),
						},
					},
				},
			},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
			PodIP: "10.244.1.16",
		},
	}

	validateConf(t, "single pod multiple paths", expectedConf, []*api.Pod{&pod}, []*api.Secret{})

	resetConf()
}

/*
Test for github.com/30x/k8s-router/nginx/config#GetConf with multiple, single pod services
*/
func TestGetConfMultipleRoutableServices(t *testing.T) {
	expectedConf := `
events {
  worker_connections 1024;
}
http {` + getConfPreamble(config) + `
  # Upstream for / traffic on prod.github.com
  upstream upstream2407493903 {
    # Pod testing2 (namespace: testing)
    server 10.244.1.17;

  }

  # Upstream for /nodejs traffic on test.github.com
  upstream upstream2702957321 {
    # Pod testing (namespace: testing)
    server 10.244.1.16:3000;

  }

  server {
    listen 80;
    server_name prod.github.com;

    location / {
      # Upstream upstream2407493903
      proxy_pass http://upstream2407493903;
    }
  }

  server {
    listen 80;
    server_name test.github.com;
` + defaultNginxLocationTmpl + `
    location /nodejs {
      # Upstream upstream2702957321
      proxy_pass http://upstream2702957321;
    }
  }
` + getDefaultServerConf(config) + `}
`

	pods := []*api.Pod{
		&api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"routingHosts": "test.github.com",
					"routingPaths": "3000:/nodejs",
				},
				Name:      "testing",
				Namespace: "testing",
			},
			Spec: api.PodSpec{
				Containers: []api.Container{
					api.Container{
						Ports: []api.ContainerPort{
							api.ContainerPort{
								ContainerPort: int32(3000),
							},
						},
					},
				},
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.244.1.16",
			},
		},
		&api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"routingHosts": "prod.github.com",
					"routingPaths": "80:/",
				},
				Name:      "testing2",
				Namespace: "testing",
			},
			Spec: api.PodSpec{
				Containers: []api.Container{
					api.Container{
						Ports: []api.ContainerPort{
							api.ContainerPort{
								ContainerPort: int32(80),
							},
						},
					},
				},
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.244.1.17",
			},
		},
	}

	validateConf(t, "multiple pods, different services", expectedConf, pods, []*api.Secret{})
}

/*
Test for github.com/30x/k8s-router/nginx/config#GetConf with single, multiple pod services
*/
func TestGetConfMultiplePodRoutableServices(t *testing.T) {
	expectedConf := `
events {
  worker_connections 1024;
}
http {` + getConfPreamble(config) + `
  # Upstream for / traffic on test.github.com
  upstream upstream619897598 {
    # Pod testing (namespace: testing)
    server 10.244.1.16;
    # Pod testing2 (namespace: testing)
    server 10.244.1.17;
    # Pod testing3 (namespace: testing)
    server 10.244.1.18:3000;

  }

  server {
    listen 80;
    server_name test.github.com;

    location / {
      # Upstream upstream619897598
      proxy_pass http://upstream619897598;
    }
  }
` + getDefaultServerConf(config) + `}
`

	pods := []*api.Pod{
		&api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"routingHosts": "test.github.com",
					"routingPaths": "80:/",
				},
				Name:      "testing",
				Namespace: "testing",
			},
			Spec: api.PodSpec{
				Containers: []api.Container{
					api.Container{
						Ports: []api.ContainerPort{
							api.ContainerPort{
								ContainerPort: int32(80),
							},
						},
					},
				},
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.244.1.16",
			},
		},
		&api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"routingHosts": "test.github.com",
					"routingPaths": "80:/",
				},
				Name:      "testing2",
				Namespace: "testing",
			},
			Spec: api.PodSpec{
				Containers: []api.Container{
					api.Container{
						Ports: []api.ContainerPort{
							api.ContainerPort{
								ContainerPort: int32(80),
							},
						},
					},
				},
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.244.1.17",
			},
		},
		&api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"routingHosts": "test.github.com",
					"routingPaths": "3000:/",
				},
				Name:      "testing3",
				Namespace: "testing",
			},
			Spec: api.PodSpec{
				Containers: []api.Container{
					api.Container{
						Ports: []api.ContainerPort{
							api.ContainerPort{
								ContainerPort: int32(3000),
							},
						},
					},
				},
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.244.1.18",
			},
		},
	}

	validateConf(t, "multiple pods, same service", expectedConf, pods, []*api.Secret{})
}

func TestGetConfMultiplePodsWithHealthCheckDisabled(t *testing.T) {

	// Enable Nginx Upstream Health Check
	config.EnableNginxUpstreamCheckModule = false

	expectedConf := `
events {
  worker_connections 1024;
}
http {` + getConfPreamble(config) + `
  # Upstream for / traffic on test.github.com
  upstream upstream619897598 {
    # Pod testing (namespace: testing)
    server 10.244.1.16;
    # Pod testing2 (namespace: testing)
    server 10.244.1.17;
    # Pod testing3 (namespace: testing)
    server 10.244.1.18:3000;

  }

  server {
    listen 80;
    server_name test.github.com;

    location / {
      # Upstream upstream619897598
      proxy_pass http://upstream619897598;
    }
  }
` + getDefaultServerConf(config) + `}
`

	probe := api.Probe{
		FailureThreshold: 3,
		SuccessThreshold: 1,
		PeriodSeconds: 10,
		TimeoutSeconds: 5,
		Handler: api.Handler{
			HTTPGet: &api.HTTPGetAction{
				Path: "/status",
			},
		},
	}

	pods := []*api.Pod{
		&api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"routingHosts": "test.github.com",
					"routingPaths": "80:/",
				},
				Name:      "testing",
				Namespace: "testing",
			},
			Spec: api.PodSpec{
				Containers: []api.Container{
					api.Container{
						Ports: []api.ContainerPort{
							api.ContainerPort{
								ContainerPort: int32(80),
							},
						},
						ReadinessProbe: &probe,
					},
				},
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.244.1.16",
			},
		},
		&api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"routingHosts": "test.github.com",
					"routingPaths": "80:/",
				},
				Name:      "testing2",
				Namespace: "testing",
			},
			Spec: api.PodSpec{
				Containers: []api.Container{
					api.Container{
						Ports: []api.ContainerPort{
							api.ContainerPort{
								ContainerPort: int32(80),
							},
						},
						ReadinessProbe: &probe,
					},
				},
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.244.1.17",
			},
		},
		&api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"routingHosts": "test.github.com",
					"routingPaths": "3000:/",
				},
				Name:      "testing3",
				Namespace: "testing",
			},
			Spec: api.PodSpec{
				Containers: []api.Container{
					api.Container{
						Ports: []api.ContainerPort{
							api.ContainerPort{
								ContainerPort: int32(3000),
							},
						},
						ReadinessProbe: &probe,
					},
				},
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.244.1.18",
			},
		},
	}

	validateConf(t, "multiple pods, same service with ReadinessProbes but health check disabled", expectedConf, pods, []*api.Secret{})
}

/*
Test for github.com/30x/k8s-router/nginx/config#GetConf with single, multiple pod services with health checks
*/
func TestGetConfMultiplePodsWithHealthCheckEnabled(t *testing.T) {

	// Enable Nginx Upstream Health Check
	config.EnableNginxUpstreamCheckModule = true

	expectedConf := `
events {
  worker_connections 1024;
}
http {` + getConfPreamble(config) + `
  # Upstream for / traffic on test.github.com
  upstream upstream619897598 {
    # Pod testing (namespace: testing)
    server 10.244.1.16;
    # Pod testing2 (namespace: testing)
    server 10.244.1.17;
    # Pod testing3 (namespace: testing)
    server 10.244.1.18:3000;

    # Upstream Health Check for nginx_upstream_check_module - https://github.com/yaoweibin/nginx_upstream_check_module 
    check interval=10000 rise=1 fall=3 timeout=5000 port=0 type=http;
    check_http_send "GET /status HTTP/1.0\r\n\r\n";
    check_http_expect_alive http_2xx; 

  }

  server {
    listen 80;
    server_name test.github.com;

    location / {
      # Upstream upstream619897598
      proxy_pass http://upstream619897598;
    }
  }
` + getDefaultServerConf(config) + `}
`

	probe := api.Probe{
		FailureThreshold: 3,
		SuccessThreshold: 1,
		PeriodSeconds: 10,
		TimeoutSeconds: 5,
		Handler: api.Handler{
			HTTPGet: &api.HTTPGetAction{
				Path: "/status",
			},
		},
	}

	pods := []*api.Pod{
		&api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"routingHosts": "test.github.com",
					"routingPaths": "80:/",
				},
				Name:      "testing",
				Namespace: "testing",
			},
			Spec: api.PodSpec{
				Containers: []api.Container{
					api.Container{
						Ports: []api.ContainerPort{
							api.ContainerPort{
								ContainerPort: int32(80),
							},
						},
						ReadinessProbe: &probe,
					},
				},
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.244.1.16",
			},
		},
		&api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"routingHosts": "test.github.com",
					"routingPaths": "80:/",
				},
				Name:      "testing2",
				Namespace: "testing",
			},
			Spec: api.PodSpec{
				Containers: []api.Container{
					api.Container{
						Ports: []api.ContainerPort{
							api.ContainerPort{
								ContainerPort: int32(80),
							},
						},
						ReadinessProbe: &probe,
					},
				},
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.244.1.17",
			},
		},
		&api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"routingHosts": "test.github.com",
					"routingPaths": "3000:/",
				},
				Name:      "testing3",
				Namespace: "testing",
			},
			Spec: api.PodSpec{
				Containers: []api.Container{
					api.Container{
						Ports: []api.ContainerPort{
							api.ContainerPort{
								ContainerPort: int32(3000),
							},
						},
						ReadinessProbe: &probe,
					},
				},
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.244.1.18",
			},
		},
	}

	validateConf(t, "multiple pods, same service with nginx health checks", expectedConf, pods, []*api.Secret{})
}

/*
Test for github.com/30x/k8s-router/nginx/config#GetConf with single, multiple pod services with tcp health checks
*/
func TestGetConfMultiplePodsWithTCPHealthCheckEnabled(t *testing.T) {

	// Enable Nginx Upstream Health Check
	config.EnableNginxUpstreamCheckModule = true

	expectedConf := `
events {
  worker_connections 1024;
}
http {` + getConfPreamble(config) + `
  # Upstream for / traffic on test.github.com
  upstream upstream619897598 {
    # Pod testing (namespace: testing)
    server 10.244.1.16;
    # Pod testing2 (namespace: testing)
    server 10.244.1.17;
    # Pod testing3 (namespace: testing)
    server 10.244.1.18:3000;

    # Upstream Health Check for nginx_upstream_check_module - https://github.com/yaoweibin/nginx_upstream_check_module 
    check interval=10000 rise=1 fall=3 timeout=5000 port=0 type=tcp;

  }

  server {
    listen 80;
    server_name test.github.com;

    location / {
      # Upstream upstream619897598
      proxy_pass http://upstream619897598;
    }
  }
` + getDefaultServerConf(config) + `}
`

	probe := api.Probe{
		FailureThreshold: 3,
		SuccessThreshold: 1,
		PeriodSeconds: 10,
		TimeoutSeconds: 5,
		Handler: api.Handler{
			TCPSocket: &api.TCPSocketAction{
			},
		},
	}

	pods := []*api.Pod{
		&api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"routingHosts": "test.github.com",
					"routingPaths": "80:/",
				},
				Name:      "testing",
				Namespace: "testing",
			},
			Spec: api.PodSpec{
				Containers: []api.Container{
					api.Container{
						Ports: []api.ContainerPort{
							api.ContainerPort{
								ContainerPort: int32(80),
							},
						},
						ReadinessProbe: &probe,
					},
				},
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.244.1.16",
			},
		},
		&api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"routingHosts": "test.github.com",
					"routingPaths": "80:/",
				},
				Name:      "testing2",
				Namespace: "testing",
			},
			Spec: api.PodSpec{
				Containers: []api.Container{
					api.Container{
						Ports: []api.ContainerPort{
							api.ContainerPort{
								ContainerPort: int32(80),
							},
						},
						ReadinessProbe: &probe,
					},
				},
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.244.1.17",
			},
		},
		&api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"routingHosts": "test.github.com",
					"routingPaths": "3000:/",
				},
				Name:      "testing3",
				Namespace: "testing",
			},
			Spec: api.PodSpec{
				Containers: []api.Container{
					api.Container{
						Ports: []api.ContainerPort{
							api.ContainerPort{
								ContainerPort: int32(3000),
							},
						},
						ReadinessProbe: &probe,
					},
				},
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.244.1.18",
			},
		},
	}

	validateConf(t, "multiple pods, same service with nginx tcp health checks", expectedConf, pods, []*api.Secret{})
}

/*
Test for github.com/30x/k8s-router/nginx/config#GetConf with API Key
*/
func TestGetConfWithAPIKey(t *testing.T) {
	apiKey := []byte("Updated-API-Key")
	expectedConf := `
events {
  worker_connections 1024;
}
http {` + getConfPreamble(config) + `
  # Upstream for / traffic on test.github.com
  upstream upstream619897598 {
    # Pod testing (namespace: testing)
    server 10.244.1.16;

  }

  server {
    listen 80;
    server_name test.github.com;

    location / {
      # Check the Routing API Key (namespace: testing)
      if ($http_x_routing_api_key != "` + base64.StdEncoding.EncodeToString(apiKey) + `") {
        return 403;
      }

      # Upstream upstream619897598
      proxy_pass http://upstream619897598;
    }
  }
` + getDefaultServerConf(config) + `}
`

	pod := api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				"routingHosts": "test.github.com",
				"routingPaths": "80:/",
			},
			Name:      "testing",
			Namespace: "testing",
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				api.Container{
					Ports: []api.ContainerPort{
						api.ContainerPort{
							ContainerPort: int32(80),
						},
					},
				},
			},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
			PodIP: "10.244.1.16",
		},
	}
	secret := api.Secret{
		ObjectMeta: api.ObjectMeta{
			Name:      config.APIKeySecret,
			Namespace: "testing",
		},
		Data: map[string][]byte{
			"api-key": apiKey,
		},
	}

	validateConf(t, "pod with API Key", expectedConf, []*api.Pod{&pod}, []*api.Secret{&secret})
}

/*
Test for github.com/30x/k8s-router/nginx/config#GetConf with custom API Key header
*/
func TestGetConfWithCustomAPIKeyHeader(t *testing.T) {
	resetConf()

	// Change the API Key Header
	config.APIKeyHeader = "X-SOMETHING-CUSTOM_API*KEY"

	apiKey := []byte("Updated-API-Key")
	expectedConf := `
events {
  worker_connections 1024;
}
http {` + getConfPreamble(config) + `
  # Upstream for / traffic on test.github.com
  upstream upstream619897598 {
    # Pod testing (namespace: testing)
    server 10.244.1.16;

  }

  server {
    listen 80;
    server_name test.github.com;

    location / {
      # Check the Routing API Key (namespace: testing)
      if ($http_x_something_custom_api_key != "` + base64.StdEncoding.EncodeToString(apiKey) + `") {
        return 403;
      }

      # Upstream upstream619897598
      proxy_pass http://upstream619897598;
    }
  }
` + getDefaultServerConf(config) + `}
`

	pod := api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				"routingHosts": "test.github.com",
				"routingPaths": "80:/",
			},
			Name:      "testing",
			Namespace: "testing",
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				api.Container{
					Ports: []api.ContainerPort{
						api.ContainerPort{
							ContainerPort: int32(80),
						},
					},
				},
			},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
			PodIP: "10.244.1.16",
		},
	}
	secret := api.Secret{
		ObjectMeta: api.ObjectMeta{
			Name:      config.APIKeySecret,
			Namespace: "testing",
		},
		Data: map[string][]byte{
			"api-key": apiKey,
		},
	}

	validateConf(t, "pod with API Key", expectedConf, []*api.Pod{&pod}, []*api.Secret{&secret})

	resetConf()
}

/*
Test for ClientMaxBodySize config variable in Nginx Template
*/
func TestClientMaxBodySize(t *testing.T) {
	config.ClientMaxBodySize = "1234m"
	doc := getConfPreamble(config)
	idx := strings.Index(doc, "client_max_body_size 1234m;")
	if (idx < 0) {
		log.Fatalf("Failed to include client_max_body_size from config.")
	}
}
