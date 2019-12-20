# go-mod-registry
Registry client library for use by Go implementation of EdgeX micro services.  This project contains the abstract Registry interface and an implementation for Consul. These interface functions initialize a connection to the Registry service, register the service for discovery and  health checks and request service endpoint and status

### What is this repository for? ###
* Initialize connection to a Registry service
* Register the service with the Registry service for discovery and the health check callback
* Pull service endpoint information from the Registry for dependent services.
* Check the health status of dependent services via the Registry service.

### Installation ###
* Make sure you have modules enabled, i.e. have an initialized  go.mod file 
* If your code is in your GOPATH then make sure ```GO111MODULE=on``` is set
* Run ```go get github.com/edgexfoundry/go-mod-registry```
    * This will add the go-mod-registry to the go.mod file and download it into the module cache
    
### How to Use ###
This library is used by Go programs for interacting with the Registry service (i.e. Consul) and requires that a Registry service be running somewhere that the Registry Client can connect.  The Registry service connection information as well as which registry implementation to use is stored in the service's toml configuration as:

```go
    [Registry]
    Host = 'localhost'
    Port = 8500
    Type = 'consul'
    Enabled = true
```

The following code snippets demonstrate how a service uses this Registry module to register and to get dependent service endpoint information.

This code snippet shows how to connect to the Registry service and register the current service for discovery and health checks. 

> *Note that the expected health check callback URL path is "/api/v1/ping" which your service must implement.* 

```go
func initialize(useRegistry bool) error {
    if useRegistry {
        registryConfig := types.Config{
            Host:            conf.Registry.Host,
            Port:            conf.Registry.Port,
            Type:            conf.Registry.Type,
            ServiceKey:      internal.CoreDataServiceKey,
            ServiceHost:     conf.Service.Host,
            ServicePort:     conf.Service.Port,
            ServiceProtocol: conf.Service.Protocol,
            CheckInterval:   conf.Service.CheckInterval,
            CheckRoute:      clients.ApiPingRoute,
        }

        registryClient, err = registry.NewRegistryClient(registryConfig)
    	if err != nil {
    		return fmt.Errorf("connection to Registry could not be made: %v", err.Error())
    	}
    
    	// Register this service with Registry
    	err = registryClient.Register()
    	if err != nil {
    		return fmt.Errorf("could not register service with Registry: %v", err.Error())
    	}
    }
```

This code snippet shows how to get dependent service endpoint information and check status of the dependent service.

```go
    ...
    if e.RegistryClient != nil {
	    endpoint, err = (*e.RegistryClient).GetServiceEndpoint(params.ServiceKey)
	    ...
        url := fmt.Sprintf("http://%s:%v%s", endpoint.Address, endpoint.Port, params.Path)
        ...
        if (*e.RegistryClient).IsServiceAvailable(params.ServiceKey) {
           ...
        }
    } 
    ...
```