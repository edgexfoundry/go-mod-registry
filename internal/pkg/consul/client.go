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
	"time"

	consulapi "github.com/hashicorp/consul/api"

	"github.com/edgexfoundry/go-mod-registry/pkg/types"
)

const consulStatusPath = "/v1/status/leader"
const serviceStatusPass = "passing"

type consulClient struct {
	consulUrl           string
	consulClient        *consulapi.Client
	consulConfig        *consulapi.Config
	serviceKey          string
	serviceAddress      string
	servicePort         int
	healthCheckUrl      string
	healthCheckInterval string
}

// Create new Consul Client. Service details are optional, not needed just for configuration, but required if registering
func NewConsulClient(registryConfig types.Config) (*consulClient, error) {

	client := consulClient{
		serviceKey: registryConfig.ServiceKey,
		consulUrl:  registryConfig.GetRegistryUrl(),
	}

	// ServiceHost will be empty when client isn't registering the service
	if registryConfig.ServiceHost != "" {
		client.servicePort = registryConfig.ServicePort
		client.serviceAddress = registryConfig.ServiceHost
		client.healthCheckUrl = registryConfig.GetHealthCheckUrl()
		client.healthCheckInterval = registryConfig.CheckInterval
	}

	var err error

	client.consulConfig = consulapi.DefaultConfig()
	client.consulConfig.Address = client.consulUrl
	client.consulClient, err = consulapi.NewClient(client.consulConfig)
	if err != nil {
		return nil, fmt.Errorf("unable for create new Consul Client for %s: %v", client.consulUrl, err)
	}

	return &client, nil
}

// Simply checks if Consul is up and running at the configured URL
func (client *consulClient) IsAlive() bool {
	netClient := http.Client{Timeout: time.Second * 10}

	resp, err := netClient.Get(client.consulUrl + consulStatusPath)
	if err != nil {
		return false
	}

	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return true
	}

	return false
}

// Registers the current service with Consul for discover and health check
func (client *consulClient) Register() error {
	if client.serviceKey == "" || client.serviceAddress == "" || client.servicePort == 0 ||
		client.healthCheckUrl == "" || client.healthCheckInterval == "" {
		return fmt.Errorf("unable to register service with consul: Service information not set")
	}

	// Register for service discovery
	err := client.consulClient.Agent().ServiceRegister(&consulapi.AgentServiceRegistration{
		Name:    client.serviceKey,
		Address: client.serviceAddress,
		Port:    client.servicePort,
	})

	if err != nil {
		return err
	}

	// Register for Health Check
	err = client.consulClient.Agent().CheckRegister(&consulapi.AgentCheckRegistration{
		ID:        client.serviceKey,
		Name:      "Health Check: " + client.serviceKey,
		Notes:     "Check the health of the API",
		ServiceID: client.serviceKey,
		AgentServiceCheck: consulapi.AgentServiceCheck{
			HTTP:     client.healthCheckUrl,
			Interval: client.healthCheckInterval,
		},
	})

	if err != nil {
		return err
	}

	return nil
}

func (client *consulClient) Unregister() error {
	if err := client.consulClient.Agent().ServiceDeregister(client.serviceKey); err != nil {
		return fmt.Errorf("unable to de-register service with consul: %v", err)

	}
	if err := client.consulClient.Agent().CheckDeregister(client.serviceKey); err != nil {
		return fmt.Errorf("unable to de-register service health check with consul: %v", err)
	}

	return nil
}

// GetServiceEndpoint retrieves the port, service ID and host of a known endpoint from Consul.
// If this operation is successful and a known endpoint is found, it is returned. Otherwise, an error is returned.
func (client *consulClient) GetServiceEndpoint(serviceID string) (types.ServiceEndpoint, error) {
	services, err := client.consulClient.Agent().Services()
	if err != nil {
		return types.ServiceEndpoint{}, err
	}

	endpoint := types.ServiceEndpoint{}
	if service, ok := services[serviceID]; ok {
		endpoint.Port = service.Port
		endpoint.ServiceId = serviceID
		endpoint.Host = service.Address
	} else {
		return types.ServiceEndpoint{}, fmt.Errorf("no matching service endpoint found")
	}

	return endpoint, nil
}

// Checks with Consul if the target service is registered and healthy
func (client *consulClient) IsServiceAvailable(serviceKey string) (bool, error) {
	services, err := client.consulClient.Agent().Services()
	if err != nil {
		return false, fmt.Errorf("unable to check if service %s is available: %v", serviceKey, err)
	}

	if _, ok := services[serviceKey]; !ok {
		return false, fmt.Errorf("%s service is not registered. Might not have started... ", serviceKey)
	}

	healthCheck, _, err := client.consulClient.Health().Checks(serviceKey, nil)
	if err != nil {
		return false, fmt.Errorf("unable to check health of service %s: %v", serviceKey, err)
	}

	if len(healthCheck) == 0 {
		return false, fmt.Errorf("no health checks for service %s: %v", serviceKey, err)
	}

	status := healthCheck.AggregatedStatus()
	if status != serviceStatusPass {
		return false, fmt.Errorf(" %s service not healthy...", serviceKey)
	}

	return true, nil
}
