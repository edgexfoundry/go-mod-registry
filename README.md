# go-mod-registry
Registry client library for use by Go implementation of EdgeX micro services.  This project contains the abstract Registry interface and an implementation for Consul.
These interface functions initialize a connection to the Registry service, registering the service for discovery and  health checks, push and pull configuration values to/from the Registry service and pull dependent service endpoint information and status.

### What is this repository for? ###
* Initialize connection to a Registry service
* Register the service with the Registry service for discovery and the health check callback
* Push a service's configuration in to the Registry
* Pull service's configuration from the Registry into its configuration struct
* Pull service endpoint information from the Registry for dependent services.
* Check the health status of dependent services via the Registry service.

### Installation ###
* Make sure you have modules enabled, i.e. have an initialized  go.mod file 
* If your code is in your GOPATH then make sure ```GO111MODULE=on``` is set
* Run ```go get github.com/edgexfoundry/go-mod-registry```
    * This will add the go-mod-registry to the go.mod file and download it into the module cache
    
### How to Use ###
This library is used by Go programs for interacting with the Registry service (i.e. Consul) and requires that a Registry service be running somewhere that the Registry Client can connect.  The Registry service connection information as well as which registry implementation to use is stored in the service's toml configuration as:

        [Registry]
        Host = 'localhost'
        Port = 8500
        Type = 'consul'

The following code snippets demonstrate how a service uses this Registry module to register, load configuration, listen to for configuration updates and to get dependent service endpoint information.

This code snippet shows how to connect to the Registry service and register the current service for discovery and health checks and to get the service's configuration from the Registry service. Note that the expected health check callback URL path is "/api/v1/ping" which your service must implement. 
```
func initializeConfiguration(useRegistry bool, useProfile string) (*ConfigurationStruct, error) {
	configuration := &ConfigurationStruct{}
	err := config.LoadFromFile(useProfile, configuration)
	if err != nil {
		return nil, err
	}

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
            Stem:            internal.ConfigRegistryStem,
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

		// Get the service's configuration from the Registry service
        rawConfig, err := registry.Client.GetConfiguration(configuration)
        if err != nil {
            return fmt.Errorf("could not get configuration from Registry: %v", err.Error())
        }

        actual, ok := rawConfig.(*ConfigurationStruct)
        if !ok {
            return fmt.Errorf("configuration from Registry failed type check")
        }

        *configuration = actual
        
        // Run as go func so doesn't block
        go listenForConfigChanges()
    }
```

This code snippet shows how to listen for configuration changes from the Registry after connecting and registering above.

```
func listenForConfigChanges() {
	if registryClient == nil {
		LoggingClient.Error("listenForConfigChanges() registry client not set")
		return
	}

	registryClient.WatchForChanges(updateChannel, errChannel, &WritableInfo{}, internal.WritableKey)

	signalChan := make(chan os.Signal)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <- signalChan:
			// Quietly and gracefully stop when SIGINT/SIGTERM received
			return

		case ex := <-errChannel:
			LoggingClient.Error(ex.Error())

		case raw, ok := <-updateChannel:
			if !ok {
				return
			}

			actual, ok := raw.(*WritableInfo)
			if !ok {
				LoggingClient.Error("listenForConfigChanges() type check failed")
				return
			}

			Configuration.Writable = *actual

			LoggingClient.Info("Writeable configuration has been updated. Setting log level to " + Configuration.Writable.LogLevel)
			LoggingClient.SetLogLevel(Configuration.Writable.LogLevel)
		}
	}
}
```
This code snippet shows how to get dependent service endpoint information and check status of the dependent service.
```
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