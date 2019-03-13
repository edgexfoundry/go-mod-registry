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
	"github.com/edgexfoundry/go-mod-registry"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/mitchellh/consulstructure"
	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const consulStatusPath = "/v1/agent/self"
const serviceStatusPass = "passing"


type consulClient struct {
	consulUrl           string
	consulClient        *consulapi.Client
	consulConfig        *consulapi.Config
	configBasePath      string
	serviceKey           string
	serviceAddress      string
	servicePort         int
	healthCheckUrl      string
	healthCheckInterval string
}

// Create new Consul Client. Service details are optional, not needed just for configuration, but required if registering
func NewConsulClient(registryConfig registry.Config,) (*consulClient, error) {

	client := consulClient{
		serviceKey:      registryConfig.ServiceKey,
		consulUrl:      registryConfig.GetRegistryUrl(),
		configBasePath: registryConfig.Stem + registryConfig.ServiceKey,
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
		return nil, fmt.Errorf("unable for create new Consul Client: %v", err)
	}

	return &client, nil
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

// Checks to see if Consul contains the service's configuration.
func (client *consulClient) HasConfiguration() (bool, error) {
	if stemKeys, _, err := client.consulClient.KV().Keys(client.configBasePath, "", nil); err != nil {
		return false, fmt.Errorf("checking configuration existence from Consul failed: %v", err)
	} else if len(stemKeys) == 0 {
		return false, nil
	} else {
		return true, nil
	}
}

// Puts a full toml configuration into Consul
func (client *consulClient) PutConfigurationToml(configuration *toml.Tree, overwrite bool) error {

	configurationMap := configuration.ToMap()
	keyValues := convertInterfaceToConsulPairs("", configurationMap)

	// Put config properties into Consul.
	for _, keyValue := range keyValues {
		exists, _ := client.ConfigurationValueExists(keyValue.Key)
		if !exists || overwrite {
			if err := client.PutConfigurationValue(keyValue.Key, []byte(keyValue.Value)); err != nil {
				return err
			}
		}
	}

	return nil
}

// Puts a full configuration struct into the Registry
func (client *consulClient) PutConfiguration(configuration interface{}, overwrite bool) error {
	bytes, err := toml.Marshal(configuration)
	if err != nil {
		return err
	}

	tree, err := toml.LoadBytes(bytes)
	if err != nil {
		return err
	}

	err = client.PutConfigurationToml(tree, overwrite)
	if err != nil {
		return err
	}

	return nil

}

// Gets the full configuration from Consul into the target configuration struct.
// Passed in struct is only a reference for decoder, empty struct is ok
// Returns the configuration in the target struct as interface{}, which caller must cast
func (client *consulClient) GetConfiguration(configStruct interface{}) (interface{}, error) {
	var err error
	var configuration interface{}

	exists, err := client.HasConfiguration()
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, fmt.Errorf("the Registry (Consul) doesn't contain configuration for %s. Expected data at %s", client.serviceKey, client.configBasePath)
	}

	// Update configuration data from Consul using decoder
	updateChannel := make(chan interface{})
	errorChannel := make(chan error)

	decoder := client.newConsulDecoder()
	decoder.Consul = client.consulConfig
	decoder.Target = configStruct
	decoder.Prefix = client.configBasePath
	decoder.ErrCh = errorChannel
	decoder.UpdateCh = updateChannel

	defer func() {
		decoder.Close()
		close(updateChannel)
		// This is hack to avoid panic when consul is stopped since decoder is incorrectly still running
		// and will send an error on the channel
		// TODO: uncomment close and remove comments when decoder is fix to stop properly
		//close(errorChannel)
	}()

	go decoder.Run()

	select {
	case <-time.After(2 * time.Second):
		err = errors.New("timeout loading config from client")
	case ex := <-errorChannel:
		err = errors.New(ex.Error())
	case raw := <-updateChannel:
		configuration = raw
	}

	return configuration, err
}

// Sets up a Consul watch for the target key and send back updates on the update channel.
// Passed in struct is only a reference for decoder, empty struct is ok
// Sends the configuration in the target struct as interface{} on updateChannel, which caller must cast
func (client *consulClient) WatchForChanges(updateChannel chan<- interface{}, errorChannel chan<- error, configuration interface{}, watchKey string) {
	// some watch keys may already have the "/", add it for those that don't
	if !strings.Contains(watchKey, "/") {
		watchKey = "/" + watchKey
	}

	decoder := client.newConsulDecoder()
	decoder.Consul = client.consulConfig
	decoder.Target = configuration
	decoder.Prefix = client.configBasePath + watchKey
	decoder.ErrCh = errorChannel
	decoder.UpdateCh = updateChannel

	go decoder.Run()
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

// Checks if a configuration value exists in Consul
func (client *consulClient) ConfigurationValueExists(name string) (bool, error) {
	keyPair, _, err := client.consulClient.KV().Get(client.fullPath(name), nil)
	if err != nil {
		return false, fmt.Errorf("unable to check existence of %s in Consul: %v", client.fullPath(name), err)
	}
	return keyPair != nil, nil
}

// Gets a specific configuration value from Consul
func (client *consulClient) GetConfigurationValue(name string) ([]byte, error) {
	keyPair, _, err := client.consulClient.KV().Get(client.fullPath(name), nil)
	if err != nil {
		return nil, fmt.Errorf("unable to get value for %s from Consul: %v", client.fullPath(name), err)
	}

	if keyPair == nil {
		return nil, nil
	}

	return keyPair.Value, nil
}

// Puts a specific configuration value into Consul
func (client *consulClient) PutConfigurationValue(name string, value []byte) error {
	keyPair := &consulapi.KVPair{
		Key:   client.fullPath(name),
		Value: value,
	}

	_, err := client.consulClient.KV().Put(keyPair, nil)
	if err != nil {
		return fmt.Errorf("unable to put value for %s into Consul: %v", client.fullPath(name), err)
	}
	return nil
}

// Gets the service endpoint information for the target ID from Consul
func (client *consulClient) GetServiceEndpoint(serviceID string) (registry.ServiceEndpoint, error) {
	services, err := client.consulClient.Agent().Services()
	if err != nil {
		return registry.ServiceEndpoint{}, err
	}

	endpoint := registry.ServiceEndpoint{}
	if service, ok := services[serviceID]; ok {
		endpoint.Port = service.Port
		endpoint.ServiceId = serviceID
		endpoint.Host = service.Address
	}

	return endpoint, nil
}

// Checks with Consul if the target service is registered and healthy
func (client *consulClient) IsServiceAvailable(serviceKey string) error {
	services, err := client.consulClient.Agent().Services()
	if err != nil {
		return fmt.Errorf("unable to check if service %s is available: %v", serviceKey, err)
	}

	if _, ok := services[serviceKey]; !ok {
		return fmt.Errorf("%s service is not registered. Might not have started... ", serviceKey)
	}

	healthCheck, _, err := client.consulClient.Health().Checks(serviceKey, nil)
	if err != nil {
		return fmt.Errorf("unable to check health of service %s: %v", serviceKey, err)
	}

	if len(healthCheck) == 0 {
		return fmt.Errorf("no health checks for service %s: %v", serviceKey, err)
	}

	status := healthCheck.AggregatedStatus()
	if status != serviceStatusPass {
		return fmt.Errorf(" %s service not healthy...", serviceKey)
	}

	return nil
}

func (client *consulClient) fullPath(name string) string {
	return client.configBasePath + "/" + name
}

type pair struct {
	Key   string
	Value string
}

func convertInterfaceToConsulPairs(path string, interfaceMap interface{}) []*pair {
	pairs := make([]*pair, 0)

	pathPre := ""
	if path != "" {
		pathPre = path + "/"
	}

	switch interfaceMap.(type) {
	case []interface{}:
		for index, item := range interfaceMap.([]interface{}) {
			nextPairs := convertInterfaceToConsulPairs(pathPre+strconv.Itoa(index), item)
			pairs = append(pairs, nextPairs...)
		}

	case map[string]interface{}:
		for index, item := range interfaceMap.(map[string]interface{}) {
			nextPairs := convertInterfaceToConsulPairs(pathPre+index, item)
			pairs = append(pairs, nextPairs...)
		}

	case int:
		pairs = append(pairs, &pair{Key: path, Value: strconv.Itoa(interfaceMap.(int))})

	case int64:
		var value = int(interfaceMap.(int64))
		pairs = append(pairs, &pair{Key: path, Value: strconv.Itoa(value)})

	case float64:
		pairs = append(pairs, &pair{Key: path, Value: strconv.FormatFloat(interfaceMap.(float64), 'f', -1, 64)})

	case bool:
		pairs = append(pairs, &pair{Key: path, Value: strconv.FormatBool(interfaceMap.(bool))})

	case nil:
		pairs = append(pairs, &pair{Key: path, Value: ""})

	default:
		pairs = append(pairs, &pair{Key: path, Value: interfaceMap.(string)})
	}

	return pairs
}

func (client *consulClient) newConsulDecoder() *consulstructure.Decoder {
	return &consulstructure.Decoder{
		Consul: &consulapi.Config{
			Address: client.consulUrl,
		},
	}
}
