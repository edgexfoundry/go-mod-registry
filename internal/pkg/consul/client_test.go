//
// Copyright (c) 2019 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package consul

import (
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/assert"

	"github.com/edgexfoundry/go-mod-registry/pkg/types"
)

const (
	serviceName             = "consulUnitTest"
	serviceHost             = "localhost"
	defaultServicePort      = 8000
	consulBasePath          = "edgex/core/1.0/" + serviceName + "/"
	expectedHealthCheckPath = "api/v1/ping"
)

// change values to localhost and 8500 if you need to run tests against real Consul service running locally
var (
	testHost = ""
	port     = 0
)

type LoggingInfo struct {
	EnableRemote bool
	File         string
}

type MyConfig struct {
	Logging  LoggingInfo
	Service  types.ServiceEndpoint
	Port     int
	Host     string
	LogLevel string
}

var mockConsul *MockConsul

func TestMain(m *testing.M) {

	var testMockServer *httptest.Server
	if testHost == "" || port != 8500 {
		mockConsul = NewMockConsul()
		testMockServer = mockConsul.Start()

		URL, _ := url.Parse(testMockServer.URL)
		testHost = URL.Hostname()
		port, _ = strconv.Atoi(URL.Port())
	}

	exitCode := m.Run()
	if testMockServer != nil {
		defer testMockServer.Close()
	}
	os.Exit(exitCode)
}

func TestIsAlive(t *testing.T) {
	client := makeConsulClient(t, defaultServicePort, true)
	if !client.IsAlive() {
		t.Fatal("Consul not running")
	}
}

func TestHasConfigurationFalse(t *testing.T) {
	client := makeConsulClient(t, defaultServicePort, true)

	// Make sure the configuration doesn't already exists
	reset(t, client)

	// Don't push anything in yet so configuration will not exists

	actual, err := client.HasConfiguration()
	if !assert.NoError(t, err) {
		t.Fatal()
	}

	assert.False(t, actual)
}

func TestHasConfigurationTrue(t *testing.T) {
	client := makeConsulClient(t, defaultServicePort, true)

	// Make sure the configuration doesn't already exists
	reset(t, client)

	// Now push a value so the configuration will exist
	_ = client.PutConfigurationValue("Dummy", []byte("Value"))

	actual, err := client.HasConfiguration()
	if !assert.NoError(t, err) {
		t.Fatal()
	}

	assert.True(t, actual)
}

func TestHasConfigurationPartialServiceKey(t *testing.T) {
	client := makeConsulClient(t, defaultServicePort, true)

	// Make sure the configuration doesn't already exists
	reset(t, client)

	base := client.configBasePath
	if strings.LastIndex(base, "/") == len(base)-1 {
		base = base[:len(base)-1]
	}
	// Add a key with similar base path
	keyPair := api.KVPair{
		Key:   base + "-test/some-key",
		Value: []byte("Nothing"),
	}
	_, err := client.consulClient.KV().Put(&keyPair, nil)
	if !assert.NoError(t, err) {
		t.Fatal()
	}

	actual, err := client.HasConfiguration()
	if !assert.NoError(t, err) {
		t.Fatal()
	}

	assert.False(t, actual)
}

func TestHasConfigurationError(t *testing.T) {
	goodPort := port
	port = 1234 // change the Consul port to bad port
	defer func() {
		port = goodPort
	}()

	client := makeConsulClient(t, defaultServicePort, true)

	_, err := client.HasConfiguration()
	assert.Error(t, err, "expected error checking configuration existence")

	assert.Contains(t, err.Error(), "checking configuration existence")
}

func TestRegisterNoServiceInfoError(t *testing.T) {
	// Don't set the service info so check for info results in error
	client := makeConsulClient(t, defaultServicePort, false)

	err := client.Register()
	if !assert.Error(t, err, "Expected error due to no service info") {
		t.Fatal()
	}
}

func TestRegisterWithPingCallback(t *testing.T) {
	doneChan := make(chan bool)
	receivedPing := false

	// Setup a server to simulate the service for the health check callback
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.Contains(request.URL.Path, expectedHealthCheckPath) {

			switch request.Method {
			case "GET":
				receivedPing = true

				writer.Header().Set("Content-Type", "text/plain")
				_, _ = writer.Write([]byte("pong"))

				doneChan <- true
			}
		}
	}))
	defer server.Close()

	// Figure out which port the simulated service is running on.
	serverUrl, _ := url.Parse(server.URL)
	serverPort, _ := strconv.Atoi(serverUrl.Port())

	client := makeConsulClient(t, serverPort, true)
	// Make sure service is not already registered.
	_ = client.consulClient.Agent().ServiceDeregister(client.serviceKey)
	_ = client.consulClient.Agent().CheckDeregister(client.serviceKey)

	// Try to clean-up after test
	defer func(client *consulClient) {
		_ = client.consulClient.Agent().ServiceDeregister(client.serviceKey)
		_ = client.consulClient.Agent().CheckDeregister(client.serviceKey)
	}(client)

	// Register the service endpoint and health check callback
	err := client.Register()
	if !assert.NoError(t, err) {
		t.Fatal()
	}

	go func() {
		time.Sleep(10 * time.Second)
		doneChan <- false
	}()

	<-doneChan
	assert.True(t, receivedPing, "Never received health check ping")
}

func TestGetServiceEndpoint(t *testing.T) {
	expectedNotFoundEndpoint := types.ServiceEndpoint{}
	expectedFoundEndpoint := types.ServiceEndpoint{
		ServiceId: serviceName,
		Host:      serviceHost,
		Port:      defaultServicePort,
	}

	client := makeConsulClient(t, defaultServicePort, true)
	// Make sure service is not already registered.
	_ = client.consulClient.Agent().ServiceDeregister(client.serviceKey)
	_ = client.consulClient.Agent().CheckDeregister(client.serviceKey)

	// Try to clean-up after test
	defer func(client *consulClient) {
		_ = client.consulClient.Agent().ServiceDeregister(client.serviceKey)
		_ = client.consulClient.Agent().CheckDeregister(client.serviceKey)
	}(client)

	// Test for endpoint not found
	actualEndpoint, err := client.GetServiceEndpoint(client.serviceKey)
	if !assert.NoError(t, err) {
		t.Fatal()
	}
	if !assert.Equal(t, expectedNotFoundEndpoint, actualEndpoint, "Test for endpoint not found result not as expected") {
		t.Fatal()
	}

	// Register the service endpoint
	err = client.Register()
	if !assert.NoError(t, err) {
		t.Fatal()
	}

	// Test endpoint found
	actualEndpoint, err = client.GetServiceEndpoint(client.serviceKey)
	if !assert.NoError(t, err) {
		t.Fatal()
	}
	if !assert.Equal(t, expectedFoundEndpoint, actualEndpoint, "Test for endpoint found result not as expected") {
		t.Fatal()
	}
}

func TestIsServiceAvailableNotRegistered(t *testing.T) {

	client := makeConsulClient(t, defaultServicePort, true)

	// Make sure service is not already registered.
	_ = client.consulClient.Agent().ServiceDeregister(client.serviceKey)
	_ = client.consulClient.Agent().CheckDeregister(client.serviceKey)

	actual := client.IsServiceAvailable(client.serviceKey)
	if !assert.Error(t, actual, "expected error") {
		t.Fatal()
	}

	assert.Contains(t, actual.Error(), "service is not registered", "Wrong error")
}

func TestIsServiceAvailableNotHealthy(t *testing.T) {

	client := makeConsulClient(t, defaultServicePort, true)

	// Make sure service is not already registered.
	_ = client.consulClient.Agent().ServiceDeregister(client.serviceKey)
	_ = client.consulClient.Agent().CheckDeregister(client.serviceKey)

	// Try to clean-up after test
	defer func(client *consulClient) {
		_ = client.consulClient.Agent().ServiceDeregister(client.serviceKey)
		_ = client.consulClient.Agent().CheckDeregister(client.serviceKey)
	}(client)

	// Register the service endpoint, without test service to respond to health check
	err := client.Register()
	if !assert.NoError(t, err) {
		t.Fatal()
	}

	// Give time for health check to run
	time.Sleep(2 * time.Second)

	actual := client.IsServiceAvailable(client.serviceKey)
	if !assert.Error(t, actual, "expected error") {
		t.Fatal()
	}

	assert.Contains(t, actual.Error(), "service not healthy", "Wrong error")
}

func TestIsServiceAvailableHealthy(t *testing.T) {
	doneChan := make(chan bool)

	// Setup a server to simulate the service for the health check callback
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.Contains(request.URL.Path, expectedHealthCheckPath) {

			switch request.Method {
			case "GET":
				writer.Header().Set("Content-Type", "text/plain")
				_, _ = writer.Write([]byte("pong"))

				doneChan <- true
			}
		}
	}))
	defer server.Close()

	// Figure out which port the simulated service is running on.
	serverUrl, _ := url.Parse(server.URL)
	serverPort, _ := strconv.Atoi(serverUrl.Port())

	client := makeConsulClient(t, serverPort, true)
	// Make sure service is not already registered.
	_ = client.consulClient.Agent().ServiceDeregister(client.serviceKey)
	_ = client.consulClient.Agent().CheckDeregister(client.serviceKey)

	// Try to clean-up after test
	defer func(client *consulClient) {
		_ = client.consulClient.Agent().ServiceDeregister(client.serviceKey)
		_ = client.consulClient.Agent().CheckDeregister(client.serviceKey)
	}(client)

	// Register the service endpoint
	err := client.Register()
	if !assert.NoError(t, err) {
		t.Fatal()
	}
	// Give time for health check to run
	go func() {
		time.Sleep(10 * time.Second)
		doneChan <- false
	}()

	receivedPing := <-doneChan
	if !assert.True(t, receivedPing, "Never received health check ping") {
		t.Fatal()
	}

	actual := client.IsServiceAvailable(client.serviceKey)
	if !assert.NoError(t, actual, "IsServiceAvailable result not as expected") {
		t.Fatal()
	}
}

func TestConfigurationValueExists(t *testing.T) {
	key := "Foo"
	value := []byte("bar")
	fullKey := consulBasePath + key

	client := makeConsulClient(t, defaultServicePort, true)
	expected := false

	// Make sure the configuration doesn't already exists
	reset(t, client)

	actual, err := client.ConfigurationValueExists(key)
	if !assert.NoError(t, err) {
		t.Fatal()
	}
	if !assert.False(t, actual) {
		t.Fatal()
	}

	keyPair := api.KVPair{
		Key:   fullKey,
		Value: value,
	}

	_, err = client.consulClient.KV().Put(&keyPair, nil)
	if !assert.NoError(t, err) {
		t.Fatal()
	}

	expected = true
	actual, err = client.ConfigurationValueExists(key)
	if !assert.NoError(t, err) {
		t.Fatal()
	}
	if !assert.Equal(t, expected, actual) {
		t.Fatal()
	}
}

func TestGetConfigurationValue(t *testing.T) {
	key := "Foo"
	expected := []byte("bar")
	fullKey := consulBasePath + key
	client := makeConsulClient(t, defaultServicePort, true)

	// Make sure the target key/value exists
	keyPair := api.KVPair{
		Key:   fullKey,
		Value: expected,
	}

	_, err := client.consulClient.KV().Put(&keyPair, nil)
	if !assert.NoError(t, err) {
		t.Fatal()
	}

	actual, err := client.GetConfigurationValue(key)
	if !assert.NoError(t, err) {
		t.Fatal()
	}
	if !assert.Equal(t, expected, actual) {
		t.Fatal()
	}
}

func TestPutConfigurationValue(t *testing.T) {
	key := "Foo"
	expected := []byte("bar")
	expectedFullKey := consulBasePath + key
	client := makeConsulClient(t, defaultServicePort, true)

	// Make sure the configuration doesn't already exists
	reset(t, client)

	_, _ = client.consulClient.KV().Delete(expectedFullKey, nil)

	err := client.PutConfigurationValue(key, expected)
	assert.NoError(t, err)

	keyValue, _, err := client.consulClient.KV().Get(expectedFullKey, nil)
	if !assert.NoError(t, err) {
		t.Fatal()
	}
	if !assert.NotNil(t, keyValue, "%s value not found", expectedFullKey) {
		t.Fatal()
	}

	actual := keyValue.Value

	assert.Equal(t, expected, actual)

}

func TestGetConfiguration(t *testing.T) {
	expected := MyConfig{
		Logging: LoggingInfo{
			EnableRemote: true,
			File:         "NONE",
		},
		Service: types.ServiceEndpoint{
			ServiceId: "Dummy",
			Host:      "10.6.7.8",
			Port:      8080,
		},
		Port:     8000,
		Host:     "localhost",
		LogLevel: "debug",
	}

	client := makeConsulClient(t, defaultServicePort, true)

	_ = client.PutConfigurationValue("Logging/EnableRemote", []byte(strconv.FormatBool(expected.Logging.EnableRemote)))
	_ = client.PutConfigurationValue("Logging/File", []byte(expected.Logging.File))
	_ = client.PutConfigurationValue("Service/ServiceId", []byte(expected.Service.ServiceId))
	_ = client.PutConfigurationValue("Service/Host", []byte(expected.Service.Host))
	_ = client.PutConfigurationValue("Service/Port", []byte(strconv.Itoa(expected.Service.Port)))
	_ = client.PutConfigurationValue("Port", []byte(strconv.Itoa(expected.Port)))
	_ = client.PutConfigurationValue("Host", []byte(expected.Host))
	_ = client.PutConfigurationValue("LogLevel", []byte(expected.LogLevel))

	result, err := client.GetConfiguration(&MyConfig{})

	if !assert.NoError(t, err) {
		t.Fatal()
	}

	configuration := result.(*MyConfig)

	if !assert.NotNil(t, configuration) {
		t.Fatal()
	}

	assert.Equal(t, expected.Logging.EnableRemote, configuration.Logging.EnableRemote, "Logging.EnableRemote not as expected")
	assert.Equal(t, expected.Logging.File, configuration.Logging.File, "Logging.File not as expected")
	assert.Equal(t, expected.Service.Port, configuration.Service.Port, "Service.Port not as expected")
	assert.Equal(t, expected.Service.Host, configuration.Service.Host, "Service.Host not as expected")
	assert.Equal(t, expected.Service.ServiceId, configuration.Service.ServiceId, "Service.ServiceId not as expected")
	assert.Equal(t, expected.Port, configuration.Port, "Port not as expected")
	assert.Equal(t, expected.Host, configuration.Host, "Host not as expected")
	assert.Equal(t, expected.LogLevel, configuration.LogLevel, "LogLevel not as expected")
}

func TestPutConfiguration(t *testing.T) {
	expected := MyConfig{
		Logging: LoggingInfo{
			EnableRemote: true,
			File:         "NONE",
		},
		Service: types.ServiceEndpoint{
			ServiceId: "Dummy",
			Host:      "10.6.7.8",
			Port:      8080,
		},
		Port:     8000,
		Host:     "localhost",
		LogLevel: "debug",
	}

	client := makeConsulClient(t, defaultServicePort, true)

	// Make sure the tree of values doesn't exist.
	_, _ = client.consulClient.KV().DeleteTree(consulBasePath, nil)

	defer func() {
		// Clean up
		_, _ = client.consulClient.KV().DeleteTree(consulBasePath, nil)
	}()

	err := client.PutConfiguration(expected, true)
	if !assert.NoErrorf(t, err, "unable to put configuration: %v", err) {
		t.Fatal()
	}

	actual, err := client.HasConfiguration()
	if !assert.True(t, actual, "Failed to put configuration") {
		t.Fail()
	}

	assert.True(t, configValueSet("Logging/EnableRemote", client))
	assert.True(t, configValueSet("Logging/File", client))
	assert.True(t, configValueSet("Service/ServiceId", client))
	assert.True(t, configValueSet("Service/Host", client))
	assert.True(t, configValueSet("Service/Port", client))
	assert.True(t, configValueSet("Port", client))
	assert.True(t, configValueSet("Host", client))
	assert.True(t, configValueSet("LogLevel", client))
}

func configValueSet(key string, client *consulClient) bool {
	exists, _ := client.ConfigurationValueExists(key)
	return exists
}

func TestPutConfigurationTomlNoPreviousValues(t *testing.T) {
	client := makeConsulClient(t, defaultServicePort, true)

	// Make sure the tree of values doesn't exist.
	_, _ = client.consulClient.KV().DeleteTree(consulBasePath, nil)

	defer func() {
		// Clean up
		_, _ = client.consulClient.KV().DeleteTree(consulBasePath, nil)
	}()

	configMap := createKeyValueMap()
	configuration, err := toml.TreeFromMap(configMap)
	if err != nil {
		log.Fatalf("unable to create TOML Tree from map: %v", err)
	}
	err = client.PutConfigurationToml(configuration, false)
	if !assert.NoError(t, err) {
		t.Fatal()
	}

	keyValues := convertInterfaceToConsulPairs("", configMap)
	for _, keyValue := range keyValues {
		expected := string(keyValue.Value)
		value, err := client.GetConfigurationValue(keyValue.Key)
		if !assert.NoError(t, err) {
			t.Fatal()
		}
		actual := string(value)
		if !assert.Equal(t, expected, actual, "Values for %s are not equal", keyValue.Key) {
			t.Fatal()
		}
	}
}

func TestPutConfigurationTomlWithoutOverWrite(t *testing.T) {
	client := makeConsulClient(t, defaultServicePort, true)

	// Make sure the tree of values doesn't exist.
	_, _ = client.consulClient.KV().DeleteTree(consulBasePath, nil)

	defer func() {
		// Clean up
		_, _ = client.consulClient.KV().DeleteTree(consulBasePath, nil)
	}()

	configMap := createKeyValueMap()

	configuration, _ := toml.TreeFromMap(configMap)
	err := client.PutConfigurationToml(configuration, false)
	if !assert.NoError(t, err) {
		t.Fatal()
	}

	//Update map with new value and try to overwrite it
	configMap["int"] = 2
	configMap["int64"] = 164
	configMap["float64"] = 2.4
	configMap["string"] = "bye"
	configMap["bool"] = false

	// Try to put new values with overwrite = false
	configuration, _ = toml.TreeFromMap(configMap)
	err = client.PutConfigurationToml(configuration, false)
	if !assert.NoError(t, err) {
		t.Fatal()
	}

	keyValues := convertInterfaceToConsulPairs("", configMap)
	for _, keyValue := range keyValues {
		expected := string(keyValue.Value)
		value, err := client.GetConfigurationValue(keyValue.Key)
		if !assert.NoError(t, err) {
			t.Fatal()
		}
		actual := string(value)
		if !assert.NotEqual(t, expected, actual, "Values for %s are equal, expected not equal", keyValue.Key) {
			t.Fatal()
		}
	}
}

func TestPutConfigurationTomlOverWrite(t *testing.T) {
	client := makeConsulClient(t, defaultServicePort, true)

	// Make sure the tree of values doesn't exist.
	_, _ = client.consulClient.KV().DeleteTree(consulBasePath, nil)
	// Clean up after unit test
	defer func() {
		_, _ = client.consulClient.KV().DeleteTree(consulBasePath, nil)
	}()

	configMap := createKeyValueMap()

	configuration, _ := toml.TreeFromMap(configMap)
	err := client.PutConfigurationToml(configuration, false)
	if !assert.NoError(t, err) {
		t.Fatal()
	}

	//Update map with new value and try to overwrite it
	configMap["int"] = 2
	configMap["float64"] = 2.4
	configMap["string"] = "bye"
	configMap["bool"] = false

	// Try to put new values with overwrite = True
	configuration, _ = toml.TreeFromMap(configMap)
	err = client.PutConfigurationToml(configuration, true)
	if !assert.NoError(t, err) {
		t.Fatal()
	}

	keyValues := convertInterfaceToConsulPairs("", configMap)
	for _, keyValue := range keyValues {
		expected := string(keyValue.Value)
		value, err := client.GetConfigurationValue(keyValue.Key)
		if !assert.NoError(t, err) {
			t.Fatal()
		}
		actual := string(value)
		if !assert.Equal(t, expected, actual, "Values for %s are not equal", keyValue.Key) {
			t.Fatal()
		}
	}
}

func TestWatchForChanges(t *testing.T) {
	expectedConfig := MyConfig{
		Logging: LoggingInfo{
			EnableRemote: true,
			File:         "NONE",
		},
		Service: types.ServiceEndpoint{
			ServiceId: "Dummy",
			Host:      "10.6.7.8",
			Port:      8080,
		},
		Port:     8000,
		Host:     "localhost",
		LogLevel: "debug",
	}

	expectedChange := "random"

	client := makeConsulClient(t, defaultServicePort, false)

	// Make sure the tree of values doesn't exist.
	_, _ = client.consulClient.KV().DeleteTree(consulBasePath, nil)
	// Clean up after unit test
	defer func() {
		_, _ = client.consulClient.KV().DeleteTree(consulBasePath, nil)
	}()

	_ = client.PutConfigurationValue("Logging/EnableRemote", []byte(strconv.FormatBool(expectedConfig.Logging.EnableRemote)))
	_ = client.PutConfigurationValue("Logging/File", []byte(expectedConfig.Logging.File))
	_ = client.PutConfigurationValue("Service/ServiceId", []byte(expectedConfig.Service.ServiceId))
	_ = client.PutConfigurationValue("Service/Host", []byte(expectedConfig.Service.Host))
	_ = client.PutConfigurationValue("Service/Port", []byte(strconv.Itoa(expectedConfig.Service.Port)))
	_ = client.PutConfigurationValue("Port", []byte(strconv.Itoa(expectedConfig.Port)))
	_ = client.PutConfigurationValue("Host", []byte(expectedConfig.Host))
	_ = client.PutConfigurationValue("LogLevel", []byte(expectedConfig.LogLevel))

	loggingUpdateChannel := make(chan interface{})
	serviceUpdateChannel := make(chan interface{})
	errorChannel := make(chan error)

	client.WatchForChanges(loggingUpdateChannel, errorChannel, &LoggingInfo{}, "Logging")
	client.WatchForChanges(serviceUpdateChannel, errorChannel, &types.ServiceEndpoint{}, "/Service")

	loggingPass := 1
	servicePass := 1
	updates := 0

	for {
		select {
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting on Logging configuration loggingChanges")

		case loggingChanges := <-loggingUpdateChannel:
			assert.NotNil(t, loggingChanges)
			logInfo := loggingChanges.(*LoggingInfo)

			// first pass is for Consul Decoder always sending data once watch has been setup. It hasn't actually changed
			if loggingPass == 1 {
				if !assert.Equal(t, logInfo.File, expectedConfig.Logging.File) {
					t.Fatal()
				}

				// Make a change to logging
				_ = client.PutConfigurationValue("Logging/File", []byte(expectedChange))

				loggingPass--
				continue
			}

			// Now the data should have changed
			assert.Equal(t, logInfo.File, expectedChange)
			updates++
			if updates == 2 {
				return
			}

		case serviceChanges := <-serviceUpdateChannel:
			assert.NotNil(t, serviceChanges)
			service := serviceChanges.(*types.ServiceEndpoint)

			// first pass is for Consul Decoder always sending data once watch has been setup. It hasn't actually changed
			if servicePass == 1 {
				if !assert.Equal(t, service.Port, expectedConfig.Service.Port) {
					t.Fatal()
				}

				// Make a change to logging
				_ = client.PutConfigurationValue("Service/Host", []byte(expectedChange))

				servicePass--
				continue
			}

			// Now the data should have changed
			assert.Equal(t, service.Host, expectedChange)
			updates++
			if updates == 2 {
				return
			}

		case waitError := <-errorChannel:
			t.Fatalf("received WatchForChanges error for Logging: %v", waitError)
		}
	}
}

func makeConsulClient(t *testing.T, servicePort int, setServiceInfo bool) *consulClient {
	registryConfig := types.Config{
		Host:          testHost,
		Port:          port,
		Stem:          "edgex/core/1.0/",
		CheckInterval: "1s",
		CheckRoute:    "/api/v1/ping",
		ServiceKey:    serviceName,
	}

	if setServiceInfo {
		registryConfig.ServiceHost = serviceHost
		registryConfig.ServicePort = servicePort
	}

	client, err := NewConsulClient(registryConfig)
	if assert.NoError(t, err) == false {
		t.Fatal()
	}

	return client
}

func createKeyValueMap() map[string]interface{} {
	configMap := make(map[string]interface{})

	configMap["int"] = int(1)
	configMap["int64"] = int64(64)
	configMap["float64"] = float64(1.4)
	configMap["string"] = "hello"
	configMap["bool"] = true

	return configMap
}

func reset(t *testing.T, client *consulClient) {
	// Make sure the configuration doesn't already exists
	if mockConsul != nil {
		mockConsul.Reset()
	} else {
		key := client.configBasePath
		if strings.LastIndex(key, "/") == len(key)-1 {
			key = key[:len(key)-1]
		}

		_, err := client.consulClient.KV().Delete(key, nil)
		if !assert.NoError(t, err) {
			t.Fatal()
		}
	}
}
