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
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/edgexfoundry/go-mod-registry/v2/pkg/types"
)

const (
	serviceName             = "consulUnitTest"
	serviceHost             = "localhost"
	defaultServicePort      = 8000
	expectedHealthCheckPath = "api/v1/ping"
)

// change values to localhost and 8500 if you need to run tests against real Consul service running locally
var (
	testHost = ""
	port     = 0
)

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
	client := makeConsulClient(t, getUniqueServiceName(), defaultServicePort, true, "", nil)
	if !client.IsAlive() {
		t.Fatal("Consul not running")
	}
}

func TestRegisterNoServiceInfoError(t *testing.T) {
	// Don't set the service info so check for info results in error
	client := makeConsulClient(t, getUniqueServiceName(), defaultServicePort, false, "", nil)

	err := client.Register()
	require.Error(t, err, "Expected error due to no service info")
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

	client := makeConsulClient(t, getUniqueServiceName(), serverPort, true, "", nil)
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
	require.NoError(t, err)

	go func() {
		time.Sleep(10 * time.Second)
		doneChan <- false
	}()

	<-doneChan
	require.True(t, receivedPing, "Never received health check ping")
}

func TestRegisterCustomWithPingCallback(t *testing.T) {
	doneChan := make(chan bool)
	receivedPing := false

	route := "/test/route"

	// Setup a server to simulate the service for the health check callback
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.Contains(request.URL.Path, route) {

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

	client := makeConsulClient(t, getUniqueServiceName(), serverPort, true, "", nil)
	// Make sure service is not already registered.
	_ = client.consulClient.Agent().ServiceDeregister(client.serviceKey)
	_ = client.consulClient.Agent().CheckDeregister(client.serviceKey)

	// Try to clean-up after test
	defer func(client *consulClient) {
		_ = client.consulClient.Agent().ServiceDeregister(client.serviceKey)
		_ = client.consulClient.Agent().CheckDeregister(client.serviceKey)
	}(client)

	id := "check-id"
	name := "check-name"

	// Register the service endpoint and health check callback
	err := client.RegisterCheck(id, name, "", route, "5s")

	require.NoError(t, err)

	go func() {
		time.Sleep(10 * time.Second)
		doneChan <- false
	}()

	<-doneChan
	require.True(t, receivedPing, "Never received health check ping")
}

func TestUnregister(t *testing.T) {
	client := makeConsulClient(t, getUniqueServiceName(), defaultServicePort, true, "", nil)

	// Make sure service is not already registered.
	_ = client.consulClient.Agent().ServiceDeregister(client.serviceKey)
	_ = client.consulClient.Agent().CheckDeregister(client.serviceKey)

	err := client.Register()
	require.NoError(t, err, "Error registering service")

	err = client.Unregister()
	require.NoError(t, err, "Error un-registering service")

	_, err = client.GetServiceEndpoint(client.serviceKey)
	require.Error(t, err, "Expected error getting service endpoint")
}

func TestUnregisterCheck(t *testing.T) {
	client := makeConsulClient(t, getUniqueServiceName(), defaultServicePort, true, "", nil)

	id := "check-id"

	// Make sure service is not already registered.
	_ = client.consulClient.Agent().ServiceDeregister(client.serviceKey)
	_ = client.consulClient.Agent().CheckDeregister(client.serviceKey)
	_ = client.consulClient.Agent().CheckDeregister(id)

	err := client.RegisterCheck(id, "", "", "", "15s")
	require.NoError(t, err, "Error registering check")

	err = client.UnregisterCheck(id)
	require.NoError(t, err, "Error un-registering check")

	err = client.UnregisterCheck("test")
	require.Error(t, err, "Error un-registering check")

	err = client.Unregister()
	require.NoError(t, err, "Error un-registering service")

	_, err = client.GetServiceEndpoint(client.serviceKey)
	require.Error(t, err, "Expected error getting service endpoint")
}

func TestAccessToken(t *testing.T) {
	uniqueServiceName := getUniqueServiceName()
	client := makeConsulClient(t, uniqueServiceName, defaultServicePort, true, "", nil)

	// Test if have access to endpoint w/o access token set
	err := client.Register()
	require.NoError(t, err)

	expectedToken := "MyAccessToken"
	mockConsul.SetExpectedAccessToken(expectedToken)
	defer mockConsul.ClearExpectedAccessToken()

	// Now verify get error w/o providing the expected access token
	err = client.Register()
	require.Error(t, err)
	require.EqualError(t, err, "Unexpected response code: 403 ()")

	// Now verify not error when providing the expected access token
	client = makeConsulClient(t, uniqueServiceName, defaultServicePort, true, expectedToken, nil)
	err = client.Register()
	require.NoError(t, err)
}

func TestRenewAccessToken(t *testing.T) {
	goodToken := "bfb78dc5-c6a3-33d9-88b5-e3a4b63dda77" // nolint: gosec
	badToken := "badToken-c6a3-33d9-88b5-e3a4b63dda77"  // nolint: gosec
	serviceName := "RenewAccessToken-Test"

	renewCalled := false
	getAccessToken := func() (string, error) {
		fmt.Println("RenewAccessToken called")
		renewCalled = true
		return goodToken, nil
	}

	createClient := func() *consulClient {
		client := makeConsulClient(t, serviceName, defaultServicePort, true, badToken, getAccessToken)
		mockConsul.SetExpectedAccessToken(goodToken)
		renewCalled = false
		return client
	}

	t.Run("GetAllServiceEndpoints", func(t *testing.T) {
		client := createClient()

		_, err := client.GetAllServiceEndpoints()
		require.NoError(t, err)
		assert.True(t, renewCalled)
	})

	t.Run("GetServiceEndpoint", func(t *testing.T) {
		client := createClient()

		_, err := client.GetServiceEndpoint(serviceName)
		require.Error(t, err)
		assert.EqualError(t, err, "no matching service endpoint found")
		assert.True(t, renewCalled)
	})

	t.Run("IsServiceAvailable", func(t *testing.T) {
		client := createClient()

		_, err := client.IsServiceAvailable(serviceName)
		require.Error(t, err)
		assert.EqualError(t, err, "RenewAccessToken-Test service is not registered. Might not have started... ")
		assert.True(t, renewCalled)
	})

	t.Run("Register", func(t *testing.T) {
		client := createClient()

		err := client.Register()
		require.NoError(t, err)
		assert.True(t, renewCalled)
	})

	t.Run("RegisterCheck", func(t *testing.T) {
		client := createClient()

		err := client.RegisterCheck(serviceName, "check-me", "my notes", "/ping", "120s")
		require.NoError(t, err)
		assert.True(t, renewCalled)
	})

	t.Run("Unregister", func(t *testing.T) {
		client := createClient()

		err := client.Unregister()
		require.NoError(t, err)
		assert.True(t, renewCalled)
	})

	t.Run("UnregisterCheck", func(t *testing.T) {
		client := createClient()

		err := client.UnregisterCheck(serviceName)
		require.Error(t, err)
		assert.EqualError(t, err, "unable to de-register service health check with consul: Unexpected response code: 400 ()")
		assert.True(t, renewCalled)
	})
}

func TestGetServiceEndpoint(t *testing.T) {
	uniqueServiceName := getUniqueServiceName()
	expectedNotFoundEndpoint := types.ServiceEndpoint{}
	expectedFoundEndpoint := types.ServiceEndpoint{
		ServiceId: uniqueServiceName,
		Host:      serviceHost,
		Port:      defaultServicePort,
	}

	client := makeConsulClient(t, uniqueServiceName, defaultServicePort, true, "", nil)
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
	require.Error(t, err)

	require.Equal(t, expectedNotFoundEndpoint, actualEndpoint, "Test for endpoint not found result not as expected")

	// Register the service endpoint
	err = client.Register()
	require.NoError(t, err)

	// Test endpoint found
	actualEndpoint, err = client.GetServiceEndpoint(client.serviceKey)
	require.NoError(t, err)

	require.Equal(t, expectedFoundEndpoint, actualEndpoint, "Test for endpoint found result not as expected")
}

func TestIsServiceAvailableNotRegistered(t *testing.T) {

	client := makeConsulClient(t, getUniqueServiceName(), defaultServicePort, true, "", nil)

	// Make sure service is not already registered.
	_ = client.consulClient.Agent().ServiceDeregister(client.serviceKey)
	_ = client.consulClient.Agent().CheckDeregister(client.serviceKey)

	actual, err := client.IsServiceAvailable(client.serviceKey)

	require.False(t, actual)

	require.Error(t, err, "expected error")

	require.Contains(t, err.Error(), "service is not registered", "Wrong error")
}

func TestIsServiceAvailableNotHealthy(t *testing.T) {

	client := makeConsulClient(t, getUniqueServiceName(), defaultServicePort, true, "", nil)

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
	require.NoError(t, err)

	// Give time for health check to run
	time.Sleep(2 * time.Second)

	actual, err := client.IsServiceAvailable(client.serviceKey)
	require.False(t, actual)

	require.Error(t, err, "expected error")

	require.Contains(t, err.Error(), "service not healthy", "Wrong error")
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

	client := makeConsulClient(t, getUniqueServiceName(), serverPort, true, "", nil)
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
	require.NoError(t, err)

	// Give time for health check to run
	go func() {
		time.Sleep(10 * time.Second)
		doneChan <- false
	}()

	receivedPing := <-doneChan
	require.True(t, receivedPing, "Never received health check ping")

	actual, err := client.IsServiceAvailable(client.serviceKey)
	require.NoError(t, err, "IsServiceAvailable result not as expected")

	require.True(t, actual, "IsServiceAvailable result not as expected")
}

func makeConsulClient(t *testing.T, serviceName string, servicePort int, setServiceInfo bool, accessToken string, tokenCallback types.GetAccessTokenCallback) *consulClient {
	registryConfig := types.Config{
		Host:           testHost,
		Port:           port,
		CheckInterval:  "1s",
		CheckRoute:     "/api/v1/ping",
		ServiceKey:     serviceName,
		AccessToken:    accessToken,
		GetAccessToken: tokenCallback,
	}

	if setServiceInfo {
		registryConfig.ServiceHost = serviceHost
		registryConfig.ServicePort = servicePort
	}

	mockConsul.SetExpectedAccessToken(accessToken)

	client, err := NewConsulClient(registryConfig)
	require.NoError(t, err)

	return client
}

func getUniqueServiceName() string {
	return serviceName + strconv.Itoa(time.Now().Nanosecond())
}
