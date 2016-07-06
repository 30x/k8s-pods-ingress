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
