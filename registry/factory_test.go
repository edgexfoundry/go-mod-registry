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

package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/edgexfoundry/go-mod-registry/v4/pkg/types"
)

var registryConfig = types.Config{
	Host:        "localhost",
	Port:        59890,
	ServiceKey:  "edgex-registry-tests",
	ServiceHost: "localhost",
	ServicePort: 8080,
}

func TestNewRegistryClientKeeper(t *testing.T) {

	registryConfig.Type = "keeper"

	_, err := NewRegistryClient(registryConfig)
	if assert.Nil(t, err, "New Registry client failed: ", err) == false {
		t.Fatal()
	}
}

func TestNewRegistryBogusType(t *testing.T) {

	registryConfig.Type = "bogus"

	_, err := NewRegistryClient(registryConfig)
	if assert.NotNil(t, err, "Expected registry type error") == false {
		t.Fatal()
	}
}
