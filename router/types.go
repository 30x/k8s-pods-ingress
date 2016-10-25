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

package router

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/labels"
)

/*
Cache is the structure containing the router API Keys and the routable pods cache
*/
type Cache struct {
	Pods    map[string]*PodWithRoutes
	Secrets map[string]*api.Secret
}

/*
Config is the structure containing the configuration
*/
type Config struct {
	// The header name used to identify the API Key
	APIKeyHeader string
	// The secret name used to store the API Key for the namespace
	APIKeySecret string
	// The secret data field name to store the API Key for the namespace
	APIKeySecretDataField string
	// The name of the annotation used to find hosts to route
	HostsAnnotation string
	// The name of the annotation used to find paths to route
	PathsAnnotation string
	// The port that nginx will listen on
	Port int
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
	Name string
	Namespace string
	Status api.PodPhase
	AnnotationHash uint64
	Routes []*Route
}

/*
Route describes the incoming route matching details and the outgoing proxy backend details
*/
type Route struct {
	Incoming *Incoming
	Outgoing *Outgoing
}
