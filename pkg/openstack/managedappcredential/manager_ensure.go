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

	"github.com/gardener/gardener-extension-provider-openstack/pkg/features"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/managedappcredential/internal"

	"github.com/gardener/gardener/pkg/utils"
	"k8s.io/utils/clock"
)

// Ensure will ensure the managed application credential of an Openstack Shoot cluster.
func (m *Manager) Ensure(ctx context.Context, credentials *openstack.Credentials) (*AppCredentialAuth, error) {
	// Create a storage backend for the managed application credential.
	storage := internal.NewStorage(m.client, m.namespace)

	// Read the in use app credential and its parent user if exists.
	appCredential, oldParentUser, err := storage.ReadAppCredential(ctx)
	if err != nil {
		return nil, err
	}

	desiredParentUser := internal.NewParentFromCredentials(credentials)
	if err := desiredParentUser.Init(m.openstackClientFactory); err != nil {
		return nil, err
	}

	var (
		appCredentialExists bool
		parentChanged       bool
		oldParentUserUsable bool
	)

	if appCredential != nil && oldParentUser != nil {
		if oldParentUser.IsEqual(desiredParentUser) {
			if !oldParentUser.HaveEqualSecrets(desiredParentUser) {
				if err := storage.UpdateParentSecret(ctx, desiredParentUser); err != nil {
					return nil, err
				}
			}
		} else {
			parentChanged = true
		}

		if err := oldParentUser.Init(m.openstackClientFactory); err == nil {
			oldParentUserUsable = true

			if _, err := oldParentUser.GetClient().GetApplicationCredential(ctx, oldParentUser.GetID(), appCredential.ID); !openstackclient.IsNotFoundError(err) {
				appCredentialExists = true
			}
		}
	}

	if oldParentUserUsable {
		if parentChanged {
			if err := internal.RunGarbageCollection(ctx, oldParentUser, nil, m.shootName); err != nil {
				return nil, err
			}
		} else {
			if err := internal.RunGarbageCollection(ctx, oldParentUser, &appCredential.ID, m.shootName); err != nil {
				return nil, err
			}
		}
	}

	// Abort in case the managed application credential feature is disabled or
	// the new parent user itself is an appplication.
	if desiredParentUser.IsApplicationCredential() || !features.ExtensionFeatureGate.Enabled(features.ManagedApplicationCredential) {
		return nil, storage.DeleteAppCredential(ctx)
	}

	// Create a new application credential in case none exists, the old is expired
	// or the parent Openstack user has changed.
	if !appCredentialExists || appCredential.IsExpired(clock.RealClock{}, m.config) || parentChanged {
		nameSuffix, err := utils.GenerateRandomString(8)
		if err != nil {
			return nil, err
		}

		newAppCredential, err := internal.NewApplicationCredential(ctx, desiredParentUser, m.shootName, nameSuffix, clock.RealClock{}, m.config)
		if err != nil {
			return nil, err
		}

		if parentChanged && oldParentUserUsable {
			if err := internal.RunGarbageCollection(ctx, oldParentUser, &appCredential.ID, m.shootName); err != nil {
				return nil, err
			}
		}

		appCredential = newAppCredential
	}

	if err := internal.RunGarbageCollection(ctx, desiredParentUser, &appCredential.ID, m.shootName); err != nil {
		return nil, err
	}

	return &AppCredentialAuth{
		Credentials: appCredential.GetCredentials(desiredParentUser),
		SecretRef:   getSecretRef(m.namespace),
	}, storage.StoreAppCredential(ctx, appCredential, desiredParentUser)
}
