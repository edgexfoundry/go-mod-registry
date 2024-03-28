//
// Copyright (C) 2024 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package keeper

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/edgexfoundry/go-mod-core-contracts/v3/common"

	"github.com/edgexfoundry/go-mod-registry/v3/pkg/types"
)

const (
	serviceName        = "keeperUnitTest"
	defaultServiceHost = "localhost"
	defaultServicePort = 8000
)

// change values to localhost and 59883 if you need to run tests against real core-keeper service running locally
var (
	testRegistryHost = ""
	testRegistryPort = 0
)

var mockKeeper *MockKeeper

func TestMain(m *testing.M) {
	var testMockServer *httptest.Server
	if testRegistryHost == "" || testRegistryPort != 59883 {
		mockKeeper = NewMockKeeper()
		testMockServer = mockKeeper.Start()

		URL, _ := url.Parse(testMockServer.URL)
		testRegistryHost = URL.Hostname()
		testRegistryPort, _ = strconv.Atoi(URL.Port())
	}

	exitCode := m.Run()
	if testMockServer != nil {
		defer testMockServer.Close()
	}
	os.Exit(exitCode)
}

func TestIsAlive(t *testing.T) {
	client := makeKeeperClient(t, getUniqueServiceName(), defaultServiceHost, defaultServicePort, true)
	if !client.IsAlive() {
		t.Fatal("Keeper not running")
	}
}

func TestRegisterNoServiceInfoError(t *testing.T) {
	// Don't set the service info so check for info results in error
	client := makeKeeperClient(t, getUniqueServiceName(), defaultServiceHost, defaultServicePort, false)

	err := client.Register()
	require.Error(t, err, "Expected error due to no service info")
}

func TestRegisterWithPingCallback(t *testing.T) {
	doneChan := make(chan bool, 1)
	receivedPing := false

	// Setup a server to simulate the service for the health check callback
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.Contains(request.URL.Path, common.ApiPingRoute) {
			switch request.Method {
			case http.MethodGet:
				receivedPing = true

				writer.Header().Set(common.ContentType, common.ContentTypeText)
				_, _ = writer.Write([]byte("pong"))

				doneChan <- true
			}
		}
	}))
	defer server.Close()

	// Figure out which port the simulated service is running on.
	serverUrl, _ := url.Parse(server.URL)
	serverHost := serverUrl.Hostname()
	serverPort, _ := strconv.Atoi(serverUrl.Port())

	client := makeKeeperClient(t, getUniqueServiceName(), serverHost, serverPort, true)

	// Try to clean-up after test
	defer func() {
		_ = client.Unregister()
	}()

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

func TestDuplicateRegister(t *testing.T) {
	client := makeKeeperClient(t, getUniqueServiceName(), defaultServiceHost, defaultServicePort, true)

	// Try to clean-up after test
	defer func() {
		_ = client.Unregister()
	}()

	// Register the service for the first time
	err := client.Register()
	require.NoError(t, err)

	// Make sure the service already got registered
	_, err = client.GetServiceEndpoint(client.serviceKey)
	require.NoError(t, err, "Error getting service endpoint")

	// Re-register the service and ensure no error occurred
	err = client.Register()
	require.NoError(t, err)
}

func TestUnregister(t *testing.T) {
	client := makeKeeperClient(t, getUniqueServiceName(), defaultServiceHost, defaultServicePort, true)

	// Make sure service is not already registered.
	_ = client.Unregister()

	err := client.Register()
	require.NoError(t, err, "Error registering service")

	err = client.Unregister()
	require.NoError(t, err, "Error un-registering service")

	_, err = client.GetServiceEndpoint(client.serviceKey)
	require.NoError(t, err, "Expected no error since service registry still exists after un-registering")
}

func TestGetServiceEndpoint(t *testing.T) {
	uniqueServiceName := getUniqueServiceName()
	expectedFoundEndpoint := types.ServiceEndpoint{
		ServiceId: uniqueServiceName,
		Host:      defaultServiceHost,
		Port:      defaultServicePort,
	}

	client := makeKeeperClient(t, uniqueServiceName, defaultServiceHost, defaultServicePort, true)
	// Make sure service is not already registered.
	_ = client.Unregister()

	// Try to clean-up after test
	defer func() {
		_ = client.Unregister()
	}()

	// Test for endpoint not found
	actualEndpoint, err := client.GetServiceEndpoint(client.serviceKey)
	require.NoError(t, err)

	require.Equal(t, expectedFoundEndpoint, actualEndpoint, "Test for unregistered endpoint found result not as expected")

	// Register the service endpoint
	err = client.Register()
	require.NoError(t, err)

	// Test endpoint found
	actualEndpoint, err = client.GetServiceEndpoint(client.serviceKey)
	require.NoError(t, err)

	require.Equal(t, expectedFoundEndpoint, actualEndpoint, "Test for endpoint found result not as expected")
}

func TestIsServiceAvailableNotRegistered(t *testing.T) {

	client := makeKeeperClient(t, getUniqueServiceName(), defaultServiceHost, defaultServicePort, true)

	// Make sure service is not already registered.
	_ = client.Unregister()

	actual, err := client.IsServiceAvailable(client.serviceKey)

	require.False(t, actual)
	require.Error(t, err, "expected error")
	require.Contains(t, err.Error(), "service has been unregistered", "Wrong error")
}

func TestIsServiceAvailableNotHealthy(t *testing.T) {

	client := makeKeeperClient(t, getUniqueServiceName(), defaultServiceHost, defaultServicePort, true)

	// Make sure service is not already registered.
	_ = client.Unregister()

	// Try to clean-up after test
	defer func() {
		_ = client.Unregister()
	}()

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
	doneChan := make(chan bool, 1)

	// Setup a server to simulate the service for the health check callback
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.Contains(request.URL.Path, common.ApiPingRoute) {
			switch request.Method {
			case http.MethodGet:
				writer.Header().Set(common.ContentType, common.ContentTypeText)
				_, _ = writer.Write([]byte("pong"))

				doneChan <- true
			}
		}
	}))
	defer server.Close()

	// Figure out which port the simulated service is running on.
	serverUrl, _ := url.Parse(server.URL)
	serverHost := serverUrl.Hostname()
	serverPort, _ := strconv.Atoi(serverUrl.Port())

	client := makeKeeperClient(t, getUniqueServiceName(), serverHost, serverPort, true)

	// Try to clean-up after test
	defer func() {
		_ = client.Unregister()
	}()

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

func makeKeeperClient(t *testing.T, serviceName string, serviceHost string, servicePort int, setServiceInfo bool) *keeperClient {
	registryConfig := types.Config{
		Host:          testRegistryHost,
		Port:          testRegistryPort,
		CheckInterval: "1s",
		CheckRoute:    common.ApiPingRoute,
		ServiceKey:    serviceName,
		AuthInjector:  NewNullAuthenticationInjector(),
	}

	if setServiceInfo {
		registryConfig.ServiceHost = serviceHost
		registryConfig.ServicePort = servicePort
	}

	client, err := NewKeeperClient(registryConfig)
	require.NoError(t, err)

	return client
}

func getUniqueServiceName() string {
	return serviceName + strconv.Itoa(time.Now().Nanosecond())
}
