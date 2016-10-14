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

package kubernetes

import (
	"testing"
)

const (
	ErrUnexpected = "Unexpected error: %v."
)

/*
Test for github.com/30x/k8s-router/kubernetes/client#GetClient
*/
func TestGetClient(t *testing.T) {
	// Test will need proper kube config, dosen't need to be reachable
	client, err := GetClient()

	if err != nil {
		t.Fatalf(ErrUnexpected, err)
	} else if client == nil {
		t.Fatal("Client should not be nil")
	}
}
