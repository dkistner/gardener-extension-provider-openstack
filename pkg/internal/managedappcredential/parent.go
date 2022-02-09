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
	"context"
	"strings"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

type parent struct {
	id     string
	name   string
	secret string

	credentials    *openstack.Credentials
	identityClient openstackclient.Identity
}

func newParentFromCredentials(credentials *openstack.Credentials) (*parent, error) {
	factory, err := openstackclient.NewOpenstackClientFromCredentials(credentials)
	if err != nil {
		return nil, err
	}

	identityClient, err := factory.Identity()
	if err != nil {
		return nil, err
	}

	parent := &parent{
		credentials:    credentials,
		identityClient: identityClient,
		name:           credentials.Username,
		secret:         credentials.Password,
	}

	parentID, err := identityClient.LookupClientUserID()
	if err != nil {
		return nil, err
	}

	parent.id = parentID
	return parent, nil
}

func newParentFromSecret(secret *corev1.Secret) (*parent, error) {
	parentCredential := &openstack.Credentials{}

	if data, ok := secret.Data[openstack.TenantName]; ok {
		parentCredential.TenantName = string(data)
	}

	if data, ok := secret.Data[openstack.DomainName]; ok {
		parentCredential.DomainName = string(data)
	}

	if data, ok := secret.Data[openstack.AuthURL]; ok {
		parentCredential.AuthURL = string(data)
	}

	if data, ok := secret.Data[applicationCredentialSecretParentName]; ok {
		parentCredential.Username = string(data)
	}

	if data, ok := secret.Data[applicationCredentialSecretParentSecret]; ok {
		parentCredential.Password = string(data)
	}

	return newParentFromCredentials(parentCredential)
}

func (p *parent) cleanupOrphanApplicationCredentials(ctx context.Context, inUseAppCredentialSecret *corev1.Secret, technicalName string) error {
	var inUseAppCredentialID *string
	if inUseAppCredentialSecret != nil {
		if id, ok := inUseAppCredentialSecret.Data[openstack.ApplicationCredentialID]; ok {
			inUseAppCredentialID = pointer.StringPtr(string(id))
		}
	}

	applicationCredentials, err := p.identityClient.ListApplicationCredentials(ctx, p.id)
	if err != nil {
		return err
	}

	for _, appCredential := range applicationCredentials {
		if !strings.HasPrefix(appCredential.Name, technicalName) {
			continue
		}

		// Skip the application credential which is currently in use.
		if inUseAppCredentialID != nil && *inUseAppCredentialID == appCredential.ID {
			continue
		}

		if err := p.identityClient.DeleteApplicationCredential(ctx, p.id, appCredential.ID); err != nil {
			return err
		}
	}

	return nil
}
