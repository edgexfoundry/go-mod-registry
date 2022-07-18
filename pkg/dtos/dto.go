//
// Copyright (C) 2022 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package dtos

type AddRegistrationRequest struct {
	BaseRequest  `json:",inline"`
	Registration RegistrationDTO `json:"registration"`
}

type RegistrationResponse struct {
	BaseResponse `json:",inline"`
	Registration RegistrationDTO `json:"registration"`
}

type MultiRegistrationsResponse struct {
	BaseWithTotalCountResponse `json:",inline"`
	Registrations              []RegistrationDTO `json:"registrations"`
}

type RegistrationDTO struct {
	DBTimestamp   `json:",inline"`
	ServiceId     string      `json:"serviceId"`
	Status        string      `json:"status"`
	Host          string      `json:"host"`
	Port          int         `json:"port"`
	HealthCheck   HealthCheck `json:"healthCheck"`
	LastConnected int64       `json:"lastConnected"`
}

type HealthCheck struct {
	Interval string `json:"interval"`
	Path     string `json:"path"`
	Type     string `json:"type"`
}

type DBTimestamp struct {
	Created  int64 `json:"created,omitempty"`
	Modified int64 `json:"modified,omitempty"`
}

type BaseWithTotalCountResponse struct {
	BaseResponse `json:",inline"`
	TotalCount   uint32 `json:"totalCount"`
}

type BaseRequest struct {
	Versionable `json:",inline"`
	RequestId   string `json:"requestId"`
}

type BaseResponse struct {
	Versionable `json:",inline"`
	RequestId   string `json:"requestId"`
	Message     string `json:"message"`
	StatusCode  int    `json:"statusCode"`
}

type Versionable struct {
	ApiVersion string `json:"apiVersion"`
}
