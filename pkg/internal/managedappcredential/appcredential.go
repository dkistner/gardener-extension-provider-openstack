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
	"fmt"

	controllerconfig "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ManagedApplicationCredential represent the lifecycle of the usage for a
// managed application credential.
type ManagedApplicationCredential struct {
	config        *controllerconfig.ApplicationCrendentialConfig
	k8sClient     client.Client
	logger        logr.Logger
	parent        *parent
	technicalName string
}

// NewManagedApplicationCredential returns a new ManagedApplicationCredential
// to managed the lifecycle of a managed application credential.
func NewManagedApplicationCredential(
	client client.Client,
	config *controllerconfig.ApplicationCrendentialConfig,
	credentials *openstack.Credentials,
	logger logr.Logger,
	technicalName string,
) (*ManagedApplicationCredential, error) {
	if !config.Enabled {
		return &ManagedApplicationCredential{config: config}, nil
	}

	parent, err := newParent(credentials)
	if err != nil {
		return nil, err
	}

	return &ManagedApplicationCredential{
		config:        config,
		k8sClient:     client,
		logger:        logger,
		parent:        parent,
		technicalName: technicalName,
	}, nil
}

// IsEnabled will determine whether managed application credentials should be used.
func (m *ManagedApplicationCredential) IsEnabled() bool {
	return m.config.Enabled
}

// Ensure will ensure that a managed application credential exists.
// Beside ensuring the existence it will also check if a renewal is required.
// This could be neccesarry in case the application credential is expired
// or the parent user has been changed.
// Additionally it will also check if there are orphan managed application credentials
// on the infrastructure exists and clean them up.
func (m *ManagedApplicationCredential) Ensure(ctx context.Context) (*openstack.Credentials, error) {
	appCredentialSecret, err := getApplicationCredentialSecret(ctx, m.k8sClient, m.technicalName)
	if err != nil {
		return nil, err
	}
	if appCredentialSecret == nil {
		return m.createApplicationCredential(ctx)
	}

	if err := m.parent.cleanupOrphanApplicationCredentials(ctx, appCredentialSecret, m.technicalName); err != nil {
		return nil, err
	}

	// Application credential need to be renewed in case the parent user is exchanged.
	if m.hasParentChanged(appCredentialSecret, m.parent.id) {
		return m.handleParentHasChanged(ctx, appCredentialSecret)
	}

	// Application credential need to be renewed in case of expiration.
	if m.isApplicationCredentialExpired(appCredentialSecret) {
		return m.createApplicationCredential(ctx)
	}

	return extractCredentials(appCredentialSecret), nil
}

// DeleteManagedApplicationCredential will delete the managed application credential.
func (m *ManagedApplicationCredential) Delete(ctx context.Context) error {
	appCredentialSecret, err := getApplicationCredentialSecret(ctx, m.k8sClient, m.technicalName)
	if err != nil {
		return err
	}

	if appCredentialSecret == nil {
		m.logger.Info("cannot trigger managed application credential deletion as no application credential are available", applicationCredentialSecretName)
		return nil
	}

	appCredentialID, ok := appCredentialSecret.Data[openstack.ApplicationCredentialID]
	if !ok {
		m.logger.Info("cannot trigger managed application credential deletion as application credential id is unknown")
		return nil
	}

	if err := m.parent.identityClient.DeleteApplicationCredential(ctx, m.parent.id, string(appCredentialID)); err != nil {
		return err
	}

	return deleteApplicationCredentialSecret(ctx, m.k8sClient, m.technicalName)
}

func (m *ManagedApplicationCredential) createApplicationCredential(ctx context.Context) (*openstack.Credentials, error) {
	appCredential, err := m.parent.identityClient.CreateApplicationCredential(ctx, m.parent.id, m.technicalName, calculateExirationTime(m.config))
	if err != nil {
		return nil, err
	}

	secretData := makeSecretData(m.parent, appCredential.ID, appCredential.Name, appCredential.Secret)
	appCredentialSecret, err := ensureApplicationCredentialSecret(ctx, m.k8sClient, secretData, m.technicalName)
	if err != nil {
		return nil, err
	}

	return extractCredentials(appCredentialSecret), nil
}

func (m *ManagedApplicationCredential) handleParentHasChanged(ctx context.Context, appCredentialSecret *corev1.Secret) (*openstack.Credentials, error) {
	// Try to setup the old parent including an Openstack identity client for the old parent user.
	// This might not work as the old parent user could be already deleted, is not assigned to
	// the Openstack project anymore or the known information about the old user are meanwhile outdated.
	// In this case the application credentials owned by the old parent user cannot be removed.
	oldParent, oldParentErr := newParentFromSecret(appCredentialSecret, m.parent.credentials)
	if oldParentErr == nil {
		if err := oldParent.cleanupOrphanApplicationCredentials(ctx, appCredentialSecret, m.technicalName); err != nil {
			return nil, err
		}
	}

	// Create application credential owned by the new parent user.
	newAppCredential, err := m.parent.identityClient.CreateApplicationCredential(ctx, m.parent.id, m.technicalName, calculateExirationTime(m.config))
	if err != nil {
		return nil, err
	}

	// Delete the in use application credential owned by the old parent user.
	if oldParentErr == nil {
		inUseAppCredentialID, ok := appCredentialSecret.Data[openstack.ApplicationCredentialID]
		if !ok {
			return nil, fmt.Errorf("could not determine in use application credential id")
		}

		if err := oldParent.identityClient.DeleteApplicationCredential(ctx, oldParent.id, string(inUseAppCredentialID)); err != nil {
			return nil, err
		}
	}

	// Update the application credential secret with application credential from new parent user.
	secretData := makeSecretData(m.parent, newAppCredential.ID, newAppCredential.Name, newAppCredential.Secret)
	newAppCredentialSecret, err := ensureApplicationCredentialSecret(ctx, m.k8sClient, secretData, m.technicalName)
	if err != nil {
		return nil, err
	}

	return extractCredentials(newAppCredentialSecret), nil
}

// GetSecretReference return a reference to the secret which contain information
// to the in use managed application credential.
func (m *ManagedApplicationCredential) GetSecretReference() *corev1.SecretReference {
	return getSecretReference(m.technicalName)
}
