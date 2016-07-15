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

package utils

import (
	"testing"
)

/*
Test for github.com/30x/k8s-router/utils/validation#IsValidPort with invalid values
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
Test for github.com/30x/k8s-router/utils/validation#IsValidPort with valid values
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
