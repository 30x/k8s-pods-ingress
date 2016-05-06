package ingress

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/labels"
)

/*
Cache is the structure containing the ingress API Keys and the routable pods cache
*/
type Cache struct {
	Pods    map[string]*PodWithRoutes
	Secrets map[string]*api.Secret
}

/*
Config is the structure containing the configuration
*/
type Config struct {
	// The secret name used to store the API Key for the namespace
	APIKeySecret string
	// The secret data field name to store the API Key for the namespace
	APIKeySecretDataField string
	// The name of the annotation used to find hosts to route
	HostsAnnotation string
	// The name of the annotation used to find paths to route
	PathsAnnotation string
	// The label selector used to identify routable objects
	RoutableLabelSelector labels.Selector
}

/*
Incoming describes the information required to route an incoming request
*/
type Incoming struct {
	Host string
	Path string
}

/*
Outgoing describes the information required to proxy to a backend
*/
type Outgoing struct {
	IP   string
	Port string
}

/*
PodWithRoutes contains a pod and its routes
*/
type PodWithRoutes struct {
	Pod    *api.Pod
	Routes []*Route
}

/*
Route describes the incoming route matching details and the outgoing proxy backend details
*/
type Route struct {
	Incoming *Incoming
	Outgoing *Outgoing
}
