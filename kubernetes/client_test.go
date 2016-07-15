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
	"os"
	"testing"
)

const (
	ErrUnexpected = "Unexpected error: %v."
)

/*
Test for github.com/30x/k8s-router/kubernetes/client#GetClient
*/
func TestGetClient(t *testing.T) {
	os.Unsetenv("KUBE_HOST")

	client, err := GetClient()

	if client != nil {
		t.Fatal("Client should be nil when KUBE_HOST is not set")
	} else if err.Error() != ErrNeedsKubeHostSet {
		t.Fatalf(ErrUnexpected, err)
	}

	// The value does not matter because creating a client does not validate the k8s server
	os.Setenv("KUBE_HOST", "http://192.168.64.2:8080")

	client, err = GetClient()

	if err != nil {
		t.Fatalf(ErrUnexpected, err)
	} else if client == nil {
		t.Fatal("Client should not be nil")
	}
}
