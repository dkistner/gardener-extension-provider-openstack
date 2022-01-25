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
	"fmt"
	"time"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"

	corev1 "k8s.io/api/core/v1"
)

func (m *ManagedApplicationCredential) isApplicationCredentialExpired(secret *corev1.Secret) bool {
	creationTimeRaw, ok := secret.Data[applicationCredentialSecretCreationTime]
	if !ok {
		m.logger.Info("could not determine if the creation time of the in use managed application credential, managed application credential will be renewed")
		return true
	}

	creationTime, err := time.Parse(time.RFC1123, string(creationTimeRaw))
	if err != nil {
		m.logger.Info("could not determine if the in use managed application credential is expired, managed application credential will be renewed")
		return true
	}

	if time.Now().UTC().After(creationTime.Add(m.config.Lifetime.Duration)) {
		return true
	}

	return false
}

func (m *ManagedApplicationCredential) hasParentChanged(secret *corev1.Secret, parentID string) bool {
	configuredParentID, ok := secret.Data[applicationCredentialSecretParentID]
	if !ok {
		m.logger.Info("could not determine the parent user id of the in use managed application credential, managed application credential will be renewed")
		return true
	}

	fmt.Println(string(configuredParentID), parentID)
	if string(configuredParentID) != parentID {
		return true
	}

	return false
}

func (m *ManagedApplicationCredential) extractCredentials(appCredentialSecret *corev1.Secret) *openstack.Credentials {
	return &openstack.Credentials{
		DomainName:                  m.parent.credentials.DomainName,
		TenantName:                  m.parent.credentials.TenantName,
		AuthURL:                     m.parent.credentials.AuthURL,
		ApplicationCredentialID:     string(appCredentialSecret.Data[openstack.ApplicationCredentialID]),
		ApplicationCredentialName:   string(appCredentialSecret.Data[openstack.ApplicationCredentialName]),
		ApplicationCredentialSecret: string(appCredentialSecret.Data[openstack.ApplicationCredentialSecret]),
	}
}
