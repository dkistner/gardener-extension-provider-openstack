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

	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal/managedappcredential/internal"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

// Delete deletes the managed application credentials of an Openstack Shoot cluster.
func (m *Manager) Delete(ctx context.Context, credentials *openstack.Credentials) error {

	desiredParentUser := internal.NewParentFromCredentials(credentials)
	if err := desiredParentUser.Init(m.openstackClientFactory); err != nil {
		return err
	}

	// Run garbage collection with the desired Openstack user.
	if err := internal.RunGarbageCollection(ctx, desiredParentUser, nil, m.shootName); err != nil {
		return err
	}

	// Create a storage backend for the managed application credential.
	storage := internal.NewStorage(m.client, m.namespace)

	_, curParentUser, err := storage.ReadAppCredential(ctx)
	if err != nil {
		return err
	}

	if curParentUser == nil {
		return storage.DeleteAppCredential(ctx)
	}

	if curParentUser.IsEqual(desiredParentUser) && !curParentUser.HaveEqualSecrets(desiredParentUser) {
		if err := storage.UpdateParentSecret(ctx, desiredParentUser); err != nil {
			return err
		}
	}

	// Try to initialize the stored/current Openstack user.
	// If possible then run also a garbage collection with those user.
	if err := curParentUser.Init(m.openstackClientFactory); err == nil {
		if err := internal.RunGarbageCollection(ctx, curParentUser, nil, m.shootName); err != nil {
			return err
		}
	}

	return storage.DeleteAppCredential(ctx)
}
