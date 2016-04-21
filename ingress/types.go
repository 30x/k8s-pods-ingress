package ingress

import (
	"k8s.io/kubernetes/pkg/api"
)

/*
Cache is the structure containing the ingress API Keys and the microservices pods cache
*/
type Cache struct {
	Pods    map[string]*PodWithRoutes
	Secrets map[string]*api.Secret
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
