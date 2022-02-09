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
	technicalName string

	secret *corev1.Secret
	parent *parent
}

// NewManagedApplicationCredential returns a new ManagedApplicationCredential
// to managed the lifecycle of a managed application credential.
func NewManagedApplicationCredential(ctx context.Context, k8sClient client.Client, technicalName string, logger logr.Logger) (*ManagedApplicationCredential, error) {
	var new = ManagedApplicationCredential{
		k8sClient:     k8sClient,
		logger:        logger,
		technicalName: technicalName,
	}

	secret, err := getApplicationCredentialSecret(ctx, k8sClient, technicalName)
	if err != nil {
		return nil, err
	}

	if secret != nil {
		new.secret = secret
	}

	return &new, nil
}

// InjectParentUserCredentials injects the credentials of the parent user
// for managed application credentials. In case the parent user itself is
// an application credential then its credential will not injected.
func (m *ManagedApplicationCredential) InjectParentUserCredentials(credentials *openstack.Credentials) error {
	if credentials.ApplicationCredentialID != "" || credentials.ApplicationCredentialName != "" {
		return nil
	}

	parent, err := newParentFromCredentials(credentials)
	if err != nil {
		return err
	}

	m.parent = parent
	return nil
}

// InjectConfig injects the managed application credentials configuration.
func (m *ManagedApplicationCredential) InjectConfig(config *controllerconfig.ApplicationCrendentialConfig) {
	m.config = config
}

// IsEnabled check whether managed appplication credentials can be used.
func (m *ManagedApplicationCredential) IsEnabled() bool {
	if m.config == nil {
		return false
	}

	if !m.config.Enabled {
		return false
	}

	if m.parent == nil {
		return false
	}

	return true
}

// IsAvailable checks if infomation about an managed application credential
// are known and determine this way if the application credential can be used.
// Internally this check is based on the existence of a secret which hold
// information about the in use managed application crededential.
func (m *ManagedApplicationCredential) IsAvailable() bool {
	if m.secret != nil {
		return true
	}
	return false
}

// Ensure will ensure that a managed application credential exists.
// Beside ensuring the existence it will also check if a renewal is required.
// This could be necessary in case the application credential is expired
// or the parent user has been changed.
func (m *ManagedApplicationCredential) Ensure(ctx context.Context) error {
	if !m.IsEnabled() {
		return fmt.Errorf("cannot ensure managed application credential as it not enabled")
	}

	if !m.IsAvailable() {
		return m.createNewApplicationCredential(ctx)
	}

	if m.hasParentChanged(m.secret, m.parent.id) {
		return m.handleParentHasChanged(ctx, m.secret)
	}

	if m.isApplicationCredentialExpired(m.secret) {
		return m.createNewApplicationCredential(ctx)
	}

	return nil
}

func (m *ManagedApplicationCredential) createNewApplicationCredential(ctx context.Context) error {
	appCredential, err := m.parent.identityClient.CreateApplicationCredential(ctx, m.parent.id, m.technicalName, calculateExirationTime(m.config))
	if err != nil {
		return err
	}

	secretData := makeSecretData(m.parent, appCredential.ID, appCredential.Name, appCredential.Secret)
	appCredentialSecret, err := ensureApplicationCredentialSecret(ctx, m.k8sClient, secretData, m.technicalName)
	if err != nil {
		return err
	}

	m.secret = appCredentialSecret
	return nil
}

func (m *ManagedApplicationCredential) handleParentHasChanged(ctx context.Context, appCredentialSecret *corev1.Secret) error {
	// Try to setup the old parent including a correspondong Openstack identity client.
	// This might not work as the old parent user could be already deleted, it is
	// not assigned to the Openstack project anymore or the stored information
	// about the old parent user are meanwhile outdated.
	// In this case the application credentials owned by the old parent user
	// cannot be removed by the openstack extension.

	oldParent, oldParentErr := newParentFromSecret(appCredentialSecret)
	if oldParentErr == nil {
		if err := oldParent.cleanupOrphanApplicationCredentials(ctx, appCredentialSecret, m.technicalName); err != nil {
			return err
		}
	}

	// Create application credential owned by the new parent user.
	newAppCredential, err := m.parent.identityClient.CreateApplicationCredential(ctx, m.parent.id, m.technicalName, calculateExirationTime(m.config))
	if err != nil {
		return err
	}

	// Delete the in use application credential owned by the old parent user.
	if oldParentErr == nil {
		appCredentialID, ok := appCredentialSecret.Data[openstack.ApplicationCredentialID]
		if !ok {
			return fmt.Errorf("could not determine the ID of the in use managed application credential")
		}

		if err := oldParent.identityClient.DeleteApplicationCredential(ctx, oldParent.id, string(appCredentialID)); err != nil {
			return err
		}
	}

	// Update the application credential secret with application credential from new parent user.
	secretData := makeSecretData(m.parent, newAppCredential.ID, newAppCredential.Name, newAppCredential.Secret)
	newAppCredentialSecret, err := ensureApplicationCredentialSecret(ctx, m.k8sClient, secretData, m.technicalName)
	if err != nil {
		return err
	}

	m.secret = newAppCredentialSecret
	return nil
}

// DeleteIfExists delete the in use managed application credential
// and its secret in the controlplane namespace if it exists.

// DeleteIfExists delete the in use managed application credential if one exists.
// In addition it delete the corresponding secret resource in the control plane
// namespace which hold information about the application credential.
func (m *ManagedApplicationCredential) DeleteIfExists(ctx context.Context) error {
	if !m.IsAvailable() {
		return nil
	}

	if m.parent == nil {
		parent, err := newParentFromSecret(m.secret)
		if err != nil {
			m.logger.Info("cannot delete managed application credential as information about parent user are unknown")
			return deleteApplicationCredentialSecret(ctx, m.k8sClient, m.technicalName)
		}
		m.parent = parent
	}

	appCredentialID, ok := m.secret.Data[openstack.ApplicationCredentialID]
	if !ok {
		m.logger.Info("cannot delete the managed application credential as application credential id is unknown")
		return deleteApplicationCredentialSecret(ctx, m.k8sClient, m.technicalName)
	}

	if err := m.parent.identityClient.DeleteApplicationCredential(ctx, m.parent.id, string(appCredentialID)); err != nil {
		return err
	}

	return deleteApplicationCredentialSecret(ctx, m.k8sClient, m.technicalName)
}

// CleanupOrphans will remove orphan managed application credentials
// on the Openstack infrastructure layer.
func (m *ManagedApplicationCredential) CleanupOrphans(ctx context.Context) error {
	if !m.IsAvailable() {
		return fmt.Errorf("cannot cleanup orphan managed application credential as no information about the in use application credential are available")
	}

	if m.parent == nil {
		parent, err := newParentFromSecret(m.secret)
		if err != nil {
			return err
		}
		m.parent = parent
	}

	return m.parent.cleanupOrphanApplicationCredentials(ctx, m.secret, m.technicalName)
}

// GetCredentials return the credentials for the in use managed application credential.
func (m *ManagedApplicationCredential) GetCredentials() (*openstack.Credentials, error) {
	if !m.IsAvailable() {
		return nil, fmt.Errorf("cannot generate credentials for in use managed application credential")
	}

	return &openstack.Credentials{
		DomainName:                  string(m.secret.Data[openstack.DomainName]),
		TenantName:                  string(m.secret.Data[openstack.TenantName]),
		AuthURL:                     string(m.secret.Data[openstack.AuthURL]),
		ApplicationCredentialID:     string(m.secret.Data[openstack.ApplicationCredentialID]),
		ApplicationCredentialName:   string(m.secret.Data[openstack.ApplicationCredentialName]),
		ApplicationCredentialSecret: string(m.secret.Data[openstack.ApplicationCredentialSecret]),
	}, nil
}

// GetSecretReference return a reference to the secret which contain information
// to the in use managed application credential.
func (m *ManagedApplicationCredential) GetSecretReference() corev1.SecretReference {
	return corev1.SecretReference{
		Name:      applicationCredentialSecretName,
		Namespace: m.technicalName,
	}
}
