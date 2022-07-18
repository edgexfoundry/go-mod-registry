//
// Copyright (C) 2022 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package keeper

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"

	"github.com/edgexfoundry/go-mod-registry/v2/pkg/dtos"
)

type MockKeeper struct {
	serviceStore map[string]dtos.RegistrationDTO
	serviceLock  sync.Mutex
}

func NewMockKeeper() *MockKeeper {
	mock := MockKeeper{
		serviceStore: make(map[string]dtos.RegistrationDTO),
	}

	return &mock
}

func (mock *MockKeeper) Start() *httptest.Server {
	testMockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasSuffix(request.URL.Path, ApiRegisterRoute) {
			switch request.Method {
			case http.MethodPost:
				mock.serviceLock.Lock()
				defer mock.serviceLock.Unlock()

				bodyBytes, err := ioutil.ReadAll(request.Body)
				if err != nil {
					log.Printf("error reading request body: %s", err.Error())
				}

				var req dtos.AddRegistrationRequest
				err = json.Unmarshal(bodyBytes, &req)
				if err != nil {
					log.Printf("error decoding request body: %s", err.Error())
				}

				resp, err := http.Get(req.Registration.HealthCheck.Type + "://" + req.Registration.Host + ":" + strconv.Itoa(req.Registration.Port) + req.Registration.HealthCheck.Path)
				if err != nil {
					log.Printf("error health checking: %s", err.Error())
				} else {
					if resp.StatusCode == http.StatusOK {
						req.Registration.Status = "UP"
					} else {
						req.Registration.Status = "DOWN"
					}
				}
				mock.serviceStore[req.Registration.ServiceId] = req.Registration

				writer.Header().Set(ContentTypeJSON, ContentTypeJSON)
				writer.WriteHeader(http.StatusCreated)

			}
		} else if strings.HasSuffix(request.URL.Path, ApiAllRegistrationRoute) {
			switch request.Method {
			case http.MethodGet:
				mock.serviceLock.Lock()
				defer mock.serviceLock.Unlock()

				var registrations []dtos.RegistrationDTO
				for _, r := range mock.serviceStore {
					registrations = append(registrations, r)
				}
				res := dtos.MultiRegistrationsResponse{
					BaseWithTotalCountResponse: dtos.BaseWithTotalCountResponse{
						BaseResponse: dtos.BaseResponse{
							Versionable: dtos.Versionable{ApiVersion: ApiVersion},
							RequestId:   "",
							Message:     "",
							StatusCode:  200,
						},
						TotalCount: uint32(len(mock.serviceStore)),
					},
					Registrations: registrations,
				}
				jsonData, _ := json.Marshal(res)
				writer.Header().Set(ContentType, ContentTypeJSON)
				writer.WriteHeader(http.StatusOK)
				_, err := writer.Write(jsonData)
				if err != nil {
					log.Printf("error writing data response: %s", err.Error())
				}
			}
		} else if strings.Contains(request.URL.Path, ApiRegistrationByServiceIdRoute) {
			key := strings.Replace(request.URL.Path, ApiRegistrationByServiceIdRoute, "", 1)
			switch request.Method {
			case http.MethodGet:
				var res interface{}
				var statusCode int
				r, ok := mock.serviceStore[key]
				if !ok {
					res = dtos.BaseResponse{
						Versionable: dtos.Versionable{ApiVersion: ApiVersion},
						RequestId:   "",
						Message:     "not found",
						StatusCode:  404,
					}
					statusCode = 404
				} else {
					res = dtos.RegistrationResponse{
						BaseResponse: dtos.BaseResponse{
							Versionable: dtos.Versionable{ApiVersion: ApiVersion},
							RequestId:   "",
							Message:     "",
							StatusCode:  200,
						},
						Registration: r,
					}
					statusCode = 200
				}

				jsonData, _ := json.Marshal(res)
				writer.Header().Set(ContentType, ContentTypeJSON)
				writer.WriteHeader(statusCode)
				_, err := writer.Write(jsonData)
				if err != nil {
					log.Printf("error writing data response: %s", err.Error())
				}
			case http.MethodDelete:
				mock.serviceLock.Lock()
				defer mock.serviceLock.Unlock()

				_, ok := mock.serviceStore[key]
				if ok {
					delete(mock.serviceStore, key)
				}

				writer.WriteHeader(http.StatusOK)
			}
		} else if strings.Contains(request.URL.Path, ApiPingRoute) {
			switch request.Method {
			case http.MethodGet:
				writer.Header().Set(ContentType, ContentTypeText)
				_, _ = writer.Write([]byte("pong"))
			}
		}
	}))

	return testMockServer
}
