package nginx

import (
	"encoding/base64"
	"testing"

	"github.com/30x/k8s-pods-ingress/ingress"

	"k8s.io/kubernetes/pkg/api"
)

func validateConf(t *testing.T, desc, expected string, pods []*api.Pod, secrets []*api.Secret) {
	cache := &ingress.Cache{
		Pods:    make(map[string]*ingress.PodWithRoutes),
		Secrets: make(map[string]*api.Secret),
	}

	for _, pod := range pods {
		cache.Pods[pod.Name] = &ingress.PodWithRoutes{
			Pod:    pod,
			Routes: ingress.GetRoutes(pod),
		}
	}

	for _, secret := range secrets {
		cache.Secrets[secret.Namespace] = secret
	}

	actual := GetConf(cache)

	if expected != actual {
		t.Fatalf("Unexpected nginx.conf was generated (%s)\nExpected: %s\n\nActual: %s\n", desc, expected, actual)
	}
}

/*
Test for github.com/30x/k8s-pods-ingress/nginx/config#GetConf with an empty cache
*/
func TestGetConfNoMicroservices(t *testing.T) {
	conf := GetConf(&ingress.Cache{})

	if conf != DefaultNginxConf {
		t.Fatal("The default nginx.conf should be returned for an empty cache")
	}
}

/*
Test for github.com/30x/k8s-pods-ingress/nginx/config#GetConf with single pod and multiple paths
*/
func TestGetConfMultiplePaths(t *testing.T) {
	expectedConf := `
events {
  worker_connections 1024;
}
http {
  # http://nginx.org/en/docs/http/ngx_http_core_module.html
  types_hash_max_size 2048;
  server_names_hash_max_size 512;
  server_names_hash_bucket_size 64;

  server {
    listen 80;
    server_name test.github.com;

    location /prod {
      proxy_set_header Host $host;
      # Pod testing
      proxy_pass http://10.244.1.16;
    }

    location /test {
      proxy_set_header Host $host;
      # Pod testing
      proxy_pass http://10.244.1.16:3000;
    }
  }
` + DefaultNginxServerConf + `}
`

	pod := api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				"trafficHosts": "test.github.com",
				"publicPaths":  "80:/prod 3000:/test",
			},
			Name: "testing",
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
			PodIP: "10.244.1.16",
		},
	}

	validateConf(t, "single pod multiple paths", expectedConf, []*api.Pod{&pod}, []*api.Secret{})
}

/*
Test for github.com/30x/k8s-pods-ingress/nginx/config#GetConf with multiple, single pod services
*/
func TestGetConfMultipleMicroservices(t *testing.T) {
	expectedConf := `
events {
  worker_connections 1024;
}
http {
  # http://nginx.org/en/docs/http/ngx_http_core_module.html
  types_hash_max_size 2048;
  server_names_hash_max_size 512;
  server_names_hash_bucket_size 64;

  server {
    listen 80;
    server_name prod.github.com;

    location / {
      proxy_set_header Host $host;
      # Pod testing2
      proxy_pass http://10.244.1.17;
    }
  }

  server {
    listen 80;
    server_name test.github.com;

    location /nodejs {
      proxy_set_header Host $host;
      # Pod testing
      proxy_pass http://10.244.1.16:3000;
    }
  }
` + DefaultNginxServerConf + `}
`

	pods := []*api.Pod{
		&api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"trafficHosts": "test.github.com",
					"publicPaths":  "3000:/nodejs",
				},
				Name: "testing",
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.244.1.16",
			},
		},
		&api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"trafficHosts": "prod.github.com",
					"publicPaths":  "80:/",
				},
				Name: "testing2",
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
Test for github.com/30x/k8s-pods-ingress/nginx/config#GetConf with single, multiple pod services
*/
func TestGetConfMultiplePodMicroservice(t *testing.T) {
	expectedConf := `
events {
  worker_connections 1024;
}
http {
  # http://nginx.org/en/docs/http/ngx_http_core_module.html
  types_hash_max_size 2048;
  server_names_hash_max_size 512;
  server_names_hash_bucket_size 64;

  # Upstream for / traffic on test.github.com
  upstream microservice619897598 {
    # Pod testing
    server 10.244.1.16;
    # Pod testing2
    server 10.244.1.17;
    # Pod testing3
    server 10.244.1.18:3000;
  }

  server {
    listen 80;
    server_name test.github.com;

    location / {
      proxy_set_header Host $host;
      # Upstream microservice619897598
      proxy_pass http://microservice619897598;
    }
  }
` + DefaultNginxServerConf + `}
`

	pods := []*api.Pod{
		&api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"trafficHosts": "test.github.com",
					"publicPaths":  "80:/",
				},
				Name: "testing",
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.244.1.16",
			},
		},
		&api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"trafficHosts": "test.github.com",
					"publicPaths":  "80:/",
				},
				Name: "testing2",
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.244.1.17",
			},
		},
		&api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"trafficHosts": "test.github.com",
					"publicPaths":  "3000:/",
				},
				Name: "testing3",
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.244.1.18",
			},
		},
	}

	validateConf(t, "multiple pods, same service", expectedConf, pods, []*api.Secret{})
}

/*
Test for github.com/30x/k8s-pods-ingress/nginx/config#GetConf with single pod and multiple paths
*/
func TestGetConfWithAPIKey(t *testing.T) {
	apiKey := []byte("Updated-API-Key")
	expectedConf := `
events {
  worker_connections 1024;
}
http {
  # http://nginx.org/en/docs/http/ngx_http_core_module.html
  types_hash_max_size 2048;
  server_names_hash_max_size 512;
  server_names_hash_bucket_size 64;

  server {
    listen 80;
    server_name test.github.com;

    location / {
      proxy_set_header Host $host;
      # Check the Ingress API Key (namespace: testing)
      if ($http_x_ingress_api_key != '` + base64.StdEncoding.EncodeToString(apiKey) + `') {
        return 403;
      }
      # Pod testing
      proxy_pass http://10.244.1.16;
    }
  }
` + DefaultNginxServerConf + `}
`

	pod := api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				"trafficHosts": "test.github.com",
				"publicPaths":  "80:/",
			},
			Name:      "testing",
			Namespace: "testing",
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
			PodIP: "10.244.1.16",
		},
	}
	secret := api.Secret{
		ObjectMeta: api.ObjectMeta{
			Name:      ingress.KeyIngressSecretName,
			Namespace: "testing",
		},
		Data: map[string][]byte{
			"api-key": apiKey,
		},
	}

	validateConf(t, "pod with API Key", expectedConf, []*api.Pod{&pod}, []*api.Secret{&secret})
}
