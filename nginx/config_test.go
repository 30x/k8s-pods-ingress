package nginx

import (
	"testing"

	"k8s.io/kubernetes/pkg/api"
)

/*
Test for github.com/30x/k8s-pods-ingress/nginx/config#GetConfForPods with an empty cache
*/
func TestGetConfForPodsNoMicroservices(t *testing.T) {
	conf := GetConfForPods(map[string]api.Pod{})

	if conf != DefaultNginxConf {
		t.Fatal("The default nginx.conf should be returned for an empty cache")
	}
}

/*
Test for github.com/30x/k8s-pods-ingress/nginx/config#GetConfForPods with single pod and default path/port
*/
func TestGetConfForPodsDefaultPathAndPort(t *testing.T) {
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
      # Pod testing
      proxy_pass http://10.244.1.16:80;
    }
  }
}
`

	if expectedConf != GetConfForPods(map[string]api.Pod{
		"testing": api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"trafficHosts": "test.github.com",
				},
				Name: "testing",
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.244.1.16",
			},
		},
	}) {
		t.Fatal("Unexpected nginx.conf was generated for single pod with default path and port")
	}
}

/*
Test for github.com/30x/k8s-pods-ingress/nginx/config#GetConfForPods with single pod and provided path/port
*/
func TestGetConfForPodsProvidedPathAndPort(t *testing.T) {
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

    location /testing {
      proxy_set_header Host $host;
      # Pod testing
      proxy_pass http://10.244.1.16:8080;
    }
  }
}
`

	if expectedConf != GetConfForPods(map[string]api.Pod{
		"testing": api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"trafficHosts": "test.github.com",
					"publicPaths":  "/testing",
					"pathPort":     "8080",
				},
				Name: "testing",
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.244.1.16",
			},
		},
	}) {
		t.Fatal("Unexpected nginx.conf was generated for single pod with provided path and port")
	}
}

/*
Test for github.com/30x/k8s-pods-ingress/nginx/config#GetConfForPods with single pod and multiple paths
*/
func TestGetConfForPodsMultiplePaths(t *testing.T) {
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
      proxy_pass http://10.244.1.16:80;
    }

    location /test {
      proxy_set_header Host $host;
      # Pod testing
      proxy_pass http://10.244.1.16:80;
    }
  }
}
`

	if expectedConf != GetConfForPods(map[string]api.Pod{
		"testing": api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"trafficHosts": "test.github.com",
					"publicPaths":  "/prod /test",
				},
				Name: "testing",
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.244.1.16",
			},
		},
	}) {
		t.Fatal("Unexpected nginx.conf was generated for single pod with default path and port")
	}
}

/*
Test for github.com/30x/k8s-pods-ingress/nginx/config#GetConfForPods with multiple, single pod services
*/
func TestGetConfForPodsMultipleMicroservices(t *testing.T) {
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
      proxy_pass http://10.244.1.17:80;
    }
  }

  server {
    listen 80;
    server_name test.github.com;

    location / {
      proxy_set_header Host $host;
      # Pod testing
      proxy_pass http://10.244.1.16:80;
    }
  }
}
`

	if expectedConf != GetConfForPods(map[string]api.Pod{
		"testing": api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"trafficHosts": "test.github.com",
				},
				Name: "testing",
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.244.1.16",
			},
		},
		"testing2": api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"trafficHosts": "prod.github.com",
				},
				Name: "testing2",
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.244.1.17",
			},
		},
	}) {
		t.Fatal("Unexpected nginx.conf was generated for multiple pods, different services")
	}
}

/*
Test for github.com/30x/k8s-pods-ingress/nginx/config#GetConfForPods with single, multiple pod services
*/
func TestGetConfForPodsMultiplePodMicroservice(t *testing.T) {
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
    server 10.244.1.16:80
    # Pod testing2
    server 10.244.1.17:80
    # Pod testing3
    server 10.244.1.18:80
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
}
`

	if expectedConf != GetConfForPods(map[string]api.Pod{
		"testing": api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"trafficHosts": "test.github.com",
				},
				Name: "testing",
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.244.1.16",
			},
		},
		"testing2": api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"trafficHosts": "test.github.com",
				},
				Name: "testing2",
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.244.1.17",
			},
		},
		"testing3": api.Pod{
			ObjectMeta: api.ObjectMeta{
				Annotations: map[string]string{
					"trafficHosts": "test.github.com",
				},
				Name: "testing3",
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.244.1.18",
			},
		},
	}) {
		t.Fatalf("Unexpected nginx.conf was generated for multiple pods, same service")
	}
}
