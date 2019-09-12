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
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	consulapi "github.com/hashicorp/consul/api"
)

const verbose = false

type MockConsul struct {
	keyValueStore     map[string]*consulapi.KVPair
	serviceStore      map[string]consulapi.AgentService
	serviceCheckStore map[string]consulapi.AgentCheck
}

func NewMockConsul() *MockConsul {
	mock := MockConsul{}
	mock.keyValueStore = make(map[string]*consulapi.KVPair)
	mock.serviceStore = make(map[string]consulapi.AgentService)
	mock.serviceCheckStore = make(map[string]consulapi.AgentCheck)
	return &mock
}

var keyChannels map[string]chan bool
var PrefixChannels map[string]chan bool

func (mock *MockConsul) Reset() {
	mock.keyValueStore = make(map[string]*consulapi.KVPair)
	mock.serviceStore = make(map[string]consulapi.AgentService)
	mock.serviceCheckStore = make(map[string]consulapi.AgentCheck)
}

func (mock *MockConsul) Start() (*httptest.Server, chan bool) {
	ch := make(chan bool)
	keyChannels = make(map[string]chan bool)
	var consulIndex = 1
	PrefixChannels = make(map[string]chan bool)

	testMockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.Contains(request.URL.Path, "/v1/kv/") {
			key := strings.Replace(request.URL.Path, "/v1/kv/", "", 1)

			switch request.Method {
			case "PUT":
				body := make([]byte, request.ContentLength)
				if _, err := io.ReadFull(request.Body, body); err != nil {
					log.Printf("error reading request body: %s", err.Error())
				}

				keyValuePair, found := mock.keyValueStore[key]
				if found {
					keyValuePair.ModifyIndex++
					keyValuePair.Value = body
				} else {
					keyValuePair = &consulapi.KVPair{
						Key:         key,
						Value:       body,
						ModifyIndex: 1,
						CreateIndex: 1,
						Flags:       0,
						LockIndex:   0,
					}
				}

				mock.keyValueStore[key] = keyValuePair

				if verbose {
					log.Printf("PUTing new value for %s", key)
				}

				channel, found := keyChannels[key]
				if found {
					channel <- true
				}
				for prefix, channel := range PrefixChannels {
					if strings.HasPrefix(key, prefix) {
						consulIndex++
						if channel != nil {
							channel <- true
						}
					}
				}
			case "GET":
				// this is what the wait query parameters will look like "index=1&wait=600000ms"
				var pairs consulapi.KVPairs
				var prefixFound bool
				query := request.URL.Query()
				waitTime := query.Get("wait")
				// Recurse parameters are usually set when prefix is monitored,
				// if found we need to find all keys with prefix set in URL.
				_, recurseFound := query["recurse"]
				_, allKeysRequested := query["keys"]
				if recurseFound {
					pairs, prefixFound = mock.checkForPrefix(key)
					if !prefixFound {
						http.NotFound(writer, request)
						return
					}
					//Default wait time is 30 minutes, over riding it for unit test purpose
					waitTime = "1s"
					if waitTime != "" {
						waitForNextPutPrefix(key, waitTime)
					}
					writer.Header().Set("X-Consul-Index", strconv.Itoa(consulIndex))
				} else if allKeysRequested {
					// Just returning array of key names
					var keys []string

					pairs, prefixFound = mock.checkForPrefix(key)
					if !prefixFound {
						http.NotFound(writer, request)
						return
					}

					for _, key := range pairs {
						keys = append(keys, key.Key)
					}

					jsonData, _ := json.MarshalIndent(&keys, "", "  ")

					writer.Header().Set("Content-Type", "application/json")
					writer.WriteHeader(http.StatusOK)
					if _, err := writer.Write(jsonData); err != nil {
						log.Printf("error writing data response: %s", err.Error())
					}

				} else {
					keyValuePair, found := mock.keyValueStore[key]
					pairs = consulapi.KVPairs{keyValuePair}
					if !found {
						http.NotFound(writer, request)
						return
					}
					if waitTime != "" {
						waitForNextPut(key, waitTime)
					}
				}

				jsonData, _ := json.MarshalIndent(&pairs, "", "  ")

				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(http.StatusOK)
				if _, err := writer.Write(jsonData); err != nil {
					log.Printf("error writing data response: %s", err.Error())
				}
			}
		} else if strings.HasSuffix(request.URL.Path, "/v1/agent/service/register") {
			switch request.Method {
			case "PUT":
				body := make([]byte, request.ContentLength)
				if _, err := io.ReadFull(request.Body, body); err != nil {
					log.Printf("error reading request body: %s", err.Error())
				}
				//AgentServiceRegistration struct represents how service registration information is recieved
				var mockServiceRegister consulapi.AgentServiceRegistration

				//AgentService struct represent how service information is store internally
				var mockService consulapi.AgentService
				// unmarshal request body
				if err := json.Unmarshal(body, &mockServiceRegister); err != nil {
					log.Printf("error reading request body: %s", err.Error())
				}

				//Copying over basic fields required for current test cases.
				mockService.ID = mockServiceRegister.Name
				mockService.Service = mockServiceRegister.Name
				mockService.Address = mockServiceRegister.Address
				mockService.Port = mockServiceRegister.Port

				mock.serviceStore[mockService.ID] = mockService
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(http.StatusOK)

			}
		} else if strings.HasSuffix(request.URL.Path, "/v1/agent/services") {
			switch request.Method {
			case "GET":
				jsonData, _ := json.MarshalIndent(&mock.serviceStore, "", "  ")

				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(http.StatusOK)
				if _, err := writer.Write(jsonData); err != nil {
					log.Printf("error writing data response: %s", err.Error())
				}

			}
		} else if strings.Contains(request.URL.Path, "/v1/agent/service/deregister/") {
			key := strings.Replace(request.URL.Path, "/v1/agent/service/deregister/", "", 1)
			switch request.Method {
			case "PUT":
				_, ok := mock.serviceStore[key]
				if ok {
					delete(mock.serviceStore, key)
				}

				_, ok = mock.serviceCheckStore[key]
				if ok {
					delete(mock.serviceCheckStore, key)
				}
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(http.StatusOK)

			}
		} else if strings.Contains(request.URL.Path, "/v1/agent/self") {
			switch request.Method {
			case "GET":
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(http.StatusOK)

			}

		} else if strings.Contains(request.URL.Path, "/agent/check/register") {
			switch request.Method {
			case "PUT":
				body := make([]byte, request.ContentLength)
				if _, err := io.ReadFull(request.Body, body); err != nil {
					log.Printf("error reading request body: %s", err.Error())
				}

				var healthCheck consulapi.AgentCheckRegistration
				if err := json.Unmarshal(body, &healthCheck); err != nil {
					log.Printf("error reading request body: %s", err.Error())
				}

				//if endpoint for health check is set, then try call the endpoint once after interval.
				if healthCheck.AgentServiceCheck.HTTP != "" && healthCheck.AgentServiceCheck.Interval != "" {
					go func() {
						check := consulapi.AgentCheck{
							Node:        "Mock Consul server",
							CheckID:     "Health Check: " + healthCheck.ServiceID,
							Name:        "Health Check: " + healthCheck.ServiceID,
							Status:      "TBD",
							Output:      "TBD",
							ServiceID:   healthCheck.ServiceID,
							ServiceName: healthCheck.ServiceID,
						}

						_, err := http.Get(healthCheck.AgentServiceCheck.HTTP)
						if err != nil {
							check.Status = "critical"
							check.Output = "HTTP GET " + healthCheck.AgentServiceCheck.HTTP + ": health check endpoint unreachable"

							if verbose {
								log.Print("Not able to reach health check endpoint")
							}
						} else {
							check.Status = "passing"
							check.Output = "HTTP GET " + healthCheck.AgentServiceCheck.HTTP + ": 200 OK Output: pong"

							if verbose {
								log.Print("Health check endpoint is reachable!")
							}
						}

						mock.serviceCheckStore[healthCheck.ServiceID] = check
						ch <- true
					}()

				}

				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(http.StatusOK)
			}
		} else if strings.Contains(request.URL.Path, "/v1/health/checks/") {
			switch request.Method {
			case "GET":
				agentChecks := make([]consulapi.AgentCheck, 0)
				key := strings.Replace(request.URL.Path, "/v1/health/checks/", "", 1)
				check, ok := mock.serviceCheckStore[key]
				if ok {
					agentChecks = append(agentChecks, check)
				}

				jsonData, _ := json.MarshalIndent(&agentChecks, "", "  ")

				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(http.StatusOK)
				if _, err := writer.Write(jsonData); err != nil {
					log.Printf("error writing data response: %s", err.Error())
				}
			}
		}
	}))

	return testMockServer, ch
}

func waitForNextPut(key string, waitTime string) {
	timeout, err := time.ParseDuration(waitTime)
	if err != nil {
		log.Printf("Error parsing waitTime %s into a duration: %s", waitTime, err.Error())
	}
	channel := make(chan bool)
	keyChannels[key] = channel
	timedOut := false
	go func() {
		time.Sleep(timeout)
		timedOut = true
		if keyChannels[key] != nil {
			keyChannels[key] <- true
			if verbose {
				log.Printf("Timed out watching for change on %s", key)
			}
		}
	}()

	if verbose {
		log.Printf("Watching for change on %s", key)
	}
	<-channel
	close(channel)
	keyChannels[key] = nil
	if !timedOut {
		log.Printf("%s changed", key)
	}
}
func waitForNextPutPrefix(key string, waitTime string) {
	timeout, err := time.ParseDuration(waitTime)
	if err != nil {
		log.Printf("Error parsing waitTime %s into a duration: %s", waitTime, err.Error())
	}
	channel := make(chan bool)
	PrefixChannels[key] = channel
	timedOut := false
	go func() {
		time.Sleep(timeout)
		timedOut = true
		if PrefixChannels[key] != nil {
			PrefixChannels[key] <- true
			if verbose {
				log.Printf("Timed out watching for change on %s", key)
			}
		}
	}()

	if verbose {
		log.Printf("Watching for change on %s", key)
	}

	<-channel
	close(channel)
	PrefixChannels[key] = nil
	if !timedOut {
		log.Printf("%s changed", key)
	}
}

func (mock *MockConsul) checkForPrefix(prefix string) (consulapi.KVPairs, bool) {
	var pairs consulapi.KVPairs
	for k, v := range mock.keyValueStore {
		if strings.HasPrefix(k, prefix) {
			pairs = append(pairs, v)
		}
	}
	if len(pairs) == 0 {
		return nil, false
	}
	return pairs, true

}
