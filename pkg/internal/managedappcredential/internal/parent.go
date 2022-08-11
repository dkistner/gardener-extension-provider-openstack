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

package internal

import (
	"reflect"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

// Parent represent an Openstack user owning an application credential.
type Parent struct {
	credentials    *openstack.Credentials
	id             *string
	identityClient openstackclient.Identity
}

// NewParentFromCredentials returns a new parent user based on given Credentials.
func NewParentFromCredentials(credentials *openstack.Credentials) *Parent {
	return &Parent{credentials: credentials}
}

func newParentFromSecret(secret *corev1.Secret) *Parent {
	parent := NewParentFromCredentials(&openstack.Credentials{
		Username:   readSecretKey(secret, applicationCredentialSecretParentName),
		Password:   readSecretKey(secret, applicationCredentialSecretParentSecret),
		DomainName: readSecretKey(secret, openstack.DomainName),
		TenantName: readSecretKey(secret, openstack.TenantName),
		AuthURL:    readSecretKey(secret, openstack.AuthURL),
	})
	parent.id = pointer.StringPtr(readSecretKey(secret, applicationCredentialSecretParentID))

	return parent
}

// Init will initialize the parent user by setting up it's client.
func (p *Parent) Init(clientFactory openstackclient.FactoryFactory) error {
	factory, err := clientFactory.NewFactory(p.credentials)
	if err != nil {
		return err
	}

	identityClient, err := factory.Identity()
	if err != nil {
		return err
	}

	parentToken, err := identityClient.GetClientUser()
	if err != nil {
		return err
	}

	p.id = &parentToken.ID
	p.identityClient = identityClient
	return nil
}

func (p *Parent) GetID() string {
	if p.id == nil {
		panic("parent user is not initialized")
	}
	return *p.id
}

func (p *Parent) GetClient() openstackclient.Identity {
	if p.identityClient == nil {
		panic("parent user is not initialized")
	}
	return p.identityClient
}

// IsEqual check if a given parent user is identical.
func (p *Parent) IsEqual(p2 *Parent) bool {
	var (
		equal             bool
		tmpParentPassword = p.credentials.Password
	)

	// Make the passwords of both parent credentials temporarily equal
	// to exlude the password from equality check.
	p.credentials.Password = p2.credentials.Password

	// Compare the parent credentials to check for equality.
	if reflect.DeepEqual(p.credentials, p2.credentials) {
		equal = true
	}

	// Set the password back to the original one.
	p.credentials.Password = tmpParentPassword
	return equal
}

func (p *Parent) HaveEqualSecrets(p2 *Parent) bool {
	if p.credentials.Password == p2.credentials.Password {
		return true
	}
	return false
}

func (p *Parent) IsApplicationCredential() bool {
	if len(p.credentials.ApplicationCredentialID) > 0 || len(p.credentials.ApplicationCredentialName) > 0 {
		return true
	}
	return false
}
