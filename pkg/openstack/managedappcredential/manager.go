// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package managedappcredential

import (
	controllerconfig "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Config is the config for the managed application credentials.
var Config = controllerconfig.ApplicationCredentialConfig{}

// Manager is responsible to manage the lifecycle of the managed appplication
// credentials of an Openstack Shoot cluster.
type Manager struct {
	client                 client.Client
	namespace              string
	openstackClientFactory openstackclient.FactoryFactory
	shootName              string
}

// NewManager returns a new manager to manage the lifecycle of
// the managed appplication credentials of an Openstack Shoot cluster.
func NewManager(openstackClientFactory openstackclient.FactoryFactory, client client.Client, namespace, shootName string) *Manager {
	return &Manager{
		client:                 client,
		namespace:              namespace,
		openstackClientFactory: openstackClientFactory,
		shootName:              shootName,
	}
}
