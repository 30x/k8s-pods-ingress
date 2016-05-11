package utils

import (
	"testing"
)

/*
Test for github.com/30x/k8s-pods-ingress/utils/validation#IsValidPort with invalid values
*/
func TestIsValidPortNotNumberInvalidValues(t *testing.T) {
	makeError := func() {
		t.Fatal("Should had returned false")
	}

	if IsValidPort(0) {
		makeError()
	} else if IsValidPort(70000) {
		makeError()
	}
}

/*
Test for github.com/30x/k8s-pods-ingress/utils/validation#IsValidPort with valid values
*/
func TestIsValidPortNotNumberValidValues(t *testing.T) {
	makeError := func() {
		t.Fatal("Should had returned true")
	}

	if !IsValidPort(1) {
		makeError()
	} else if !IsValidPort(65000) {
		makeError()
	}
}
