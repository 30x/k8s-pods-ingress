package ingress

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"k8s.io/kubernetes/pkg/labels"
)

func getConfig(t *testing.T) *Config {
	config, err := ConfigFromEnv()

	if err != nil {
		t.Fatalf("Problem retrieving configuration")
	}

	return config
}

func getLabelSelector(t *testing.T, labelSelector string) labels.Selector {
	selector, err := labels.Parse(labelSelector)

	if err != nil {
		t.Fatalf("Unable to parse the label selector (%s): %v\n", labelSelector, err)
	}

	return selector
}

func resetEnv(t *testing.T) {
	unsetEnv := func(name string) {
		err := os.Unsetenv(name)

		if err != nil {
			t.Fatalf("Unable to unset environment variable (%s): %v\n", name, err)
		}
	}

	unsetEnv(EnvVarAPIKeySecretLocation)
	unsetEnv(EnvVarHostsAnnotation)
	unsetEnv(EnvVarPathsAnnotation)
	unsetEnv(EnvVarPort)
	unsetEnv(EnvVarRoutableLabelSelector)
}

func setEnv(t *testing.T, key, value string) {
	err := os.Setenv(key, value)

	if err != nil {
		t.Fatalf("Unable to set environment variable (%s = %s): %v\n", key, value, err)
	}
}

func validateConfig(t *testing.T, desc string, expected *Config, actual *Config) {
	makeError := func(field, eValue, aValue string) string {
		return fmt.Sprintf("Expected %s (%s) does not match actual %s (%s): %s\n", field, eValue, field, aValue, desc)
	}

	if expected.APIKeySecret != actual.APIKeySecret {
		t.Fatalf(makeError("APIKeySecret", expected.APIKeySecret, actual.APIKeySecret))
	} else if expected.APIKeySecretDataField != actual.APIKeySecretDataField {
		t.Fatalf(makeError("APIKeySecretDataField", expected.APIKeySecretDataField, actual.APIKeySecretDataField))
	} else if expected.HostsAnnotation != actual.HostsAnnotation {
		t.Fatalf(makeError("HostsAnnotation", expected.HostsAnnotation, actual.HostsAnnotation))
	} else if expected.PathsAnnotation != actual.PathsAnnotation {
		t.Fatalf(makeError("PathsAnnotation", expected.PathsAnnotation, actual.PathsAnnotation))
	} else if expected.Port != actual.Port {
		t.Fatalf(makeError("Port", strconv.Itoa(expected.Port), strconv.Itoa(actual.Port)))
	} else if expected.RoutableLabelSelector.String() != actual.RoutableLabelSelector.String() {
		t.Fatalf(makeError("RoutableLabelSelector", expected.RoutableLabelSelector.String(), actual.RoutableLabelSelector.String()))
	}
}

/*
Test for github.com/30x/k8s-router/ingress/config#ConfigFromEnv using the default environment
*/
func TestConfigFromEnvDefaultConfig(t *testing.T) {
	validateConfig(t, "default configuration", getConfig(t), &Config{
		APIKeySecret:          DefaultAPIKeySecret,
		APIKeySecretDataField: DefaultAPIKeySecretDataField,
		HostsAnnotation:       DefaultHostsAnnotation,
		PathsAnnotation:       DefaultPathsAnnotation,
		Port:                  DefaultPort,
		RoutableLabelSelector: getLabelSelector(t, DefaultRoutableLabelSelector),
	})
}

/*
Test for github.com/30x/k8s-router/ingress/config#ConfigFromEnv using invalid configurations
*/
func TestConfigFromEnvInvalidEnv(t *testing.T) {
	validateInvalidConfig := func(errMsg string) {
		config, err := ConfigFromEnv()

		if config != nil {
			t.Fatal("Config should be nil")
		} else if errMsg != err.Error() {
			t.Fatalf("Expected error message (%s) but found: %s\n", errMsg, err.Error())
		}

		resetEnv(t)
	}

	// Reset the environment variables just in case
	resetEnv(t)

	// Invalid API Key Secret location
	setEnv(t, EnvVarAPIKeySecretLocation, "routing")

	validateInvalidConfig(fmt.Sprintf(ErrMsgTmplInvalidAPIKeySecretLocation, EnvVarAPIKeySecretLocation))

	// Invalid hosts annotation
	invalidName := "*&^^%&%$$^&%&"

	setEnv(t, EnvVarHostsAnnotation, invalidName)

	validateInvalidConfig(fmt.Sprintf(ErrMsgTmplInvalidAnnotationName, EnvVarHostsAnnotation, invalidName))

	// Invalid paths annotation
	setEnv(t, EnvVarPathsAnnotation, invalidName)

	validateInvalidConfig(fmt.Sprintf(ErrMsgTmplInvalidAnnotationName, EnvVarPathsAnnotation, invalidName))

	// Invalid port (not a number)
	setEnv(t, EnvVarPort, invalidName)

	validateInvalidConfig(fmt.Sprintf(ErrMsgTmplInvalidPort, EnvVarPort, invalidName))

	// Invalid port (not a valid port)
	invalidPort := "-1"

	setEnv(t, EnvVarPort, invalidPort)

	validateInvalidConfig(fmt.Sprintf(ErrMsgTmplInvalidPort, EnvVarPort, invalidPort))

	// Invalid routable label selector
	setEnv(t, EnvVarRoutableLabelSelector, invalidName)

	validateInvalidConfig(fmt.Sprintf(ErrMsgTmplInvalidLabelSelector, EnvVarRoutableLabelSelector, invalidName))
}

/*
Test for github.com/30x/k8s-router/ingress/config#ConfigFromEnv using a valid environment
*/
func TestConfigFromEnvValidConfig(t *testing.T) {
	resetEnv(t)

	hostsAnnotation := "trafficHosts"
	pathsAnnotation := "publicPaths"
	port := "81"
	routableLabelSelector := "route-me=true"
	secretName := "custom"
	secretDataField := "another-custom"

	setEnv(t, EnvVarAPIKeySecretLocation, secretName+":"+secretDataField)
	setEnv(t, EnvVarHostsAnnotation, hostsAnnotation)
	setEnv(t, EnvVarPathsAnnotation, pathsAnnotation)
	setEnv(t, EnvVarPort, port)
	setEnv(t, EnvVarRoutableLabelSelector, routableLabelSelector)

	validateConfig(t, "default configuration", getConfig(t), &Config{
		APIKeySecret:          secretName,
		APIKeySecretDataField: secretDataField,
		HostsAnnotation:       hostsAnnotation,
		PathsAnnotation:       pathsAnnotation,
		Port:                  81,
		RoutableLabelSelector: getLabelSelector(t, routableLabelSelector),
	})
}
