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
	"reflect"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/labels"
)

/*
Cache is the structure containing the router API Keys and the routable pods cache
*/
type Cache struct {
	Pods    map[string]*PodWithRoutes
	Secrets map[string][]byte
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
	// Enable Nginx Upstream Health Check Module
	EnableNginxUpstreamCheckModule bool
	// The name of the annotation used to find hosts to route
	HostsAnnotation string
	// The name of the annotation used to find paths to route
	PathsAnnotation string
	// The port that nginx will listen on
	Port int
	// The label selector used to identify routable objects
	RoutableLabelSelector labels.Selector
	// Max client request body size. nginx config: client_max_body_size. eg 10m
	ClientMaxBodySize string
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
	HealthCheck *HealthCheck
}

/*
Health check of an Outgoing upstream server allows nginx to monitor pod health.
*/
type HealthCheck struct {
	HttpCheck bool
	Path string
	Method string
	TimeoutMs int32
	IntervalMs int32
	UnhealthyThreshold int32
	HealthyThreshold int32
	Port int32
}

func (a HealthCheck) Equal(b *HealthCheck) bool {
	return reflect.DeepEqual(a, *b);
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
