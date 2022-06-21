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
	"time"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
	"github.com/gardener/gardener/pkg/utils"
)

// Ensure will ensure the managed application credential of an Openstack Shoot cluster.
func (m *Manager) Ensure(ctx context.Context, credentials *openstack.Credentials) error {
	newParentUser, err := m.newParentFromCredentials(credentials)
	if err != nil {
		return err
	}

	appCredential, err := readApplicationCredential(ctx, m.client, m.namespace)
	if err != nil {
		return err
	}

	var (
		appCredentialExists bool
		parentChanged       bool
		oldParentUserUsable bool
		oldParentUser       *parent
	)

	if appCredential != nil {
		if err := m.updateParentPasswordIfRequired(ctx, appCredential, newParentUser); err != nil {
			return err
		}

		if user, err := m.newParentFromSecret(appCredential.secret); err == nil && user != nil {
			oldParentUser = user
			oldParentUserUsable = true
		}

		if oldParentUserUsable {
			if _, err := oldParentUser.identityClient.GetApplicationCredential(ctx, oldParentUser.id, appCredential.id); !openstackclient.IsNotFoundError(err) {
				appCredentialExists = true
			}

			if oldParentUser.id != newParentUser.id {
				parentChanged = true
			}
		}
	}

	if parentChanged {
		// Try to clean up the application credentials owned by the old parent user.
		// This might not work as the information about this user could be stale,
		// because the user credentials are rotated, the user is not associated to
		// Openstack project anymore or it is deleted.
		if err := m.runGarbageCollection(ctx, oldParentUser, nil); err != nil {
			m.logger.Error(err, "could not clean up application credential(s) as the owning user has changed and information about owning user might be stale")
		}
	}

	// In case the application credential usage is disabled or the new parent user
	// itself is an appplication, it is tried to clean up old application credentials before aborting.
	if !m.config.Enabled || newParentUser.isApplicationCredential() {
		if oldParentUserUsable {
			if err := m.runGarbageCollection(ctx, oldParentUser, nil); err != nil {
				return err
			}
		}

		if appCredential != nil {
			return m.removeApplicationCredentialStore(ctx, appCredential.secret)
		}

		return nil
	}

	if !appCredentialExists || m.isExpired(appCredential) || parentChanged {
		newAppCredential, err := m.createApplicationCredential(ctx, newParentUser)
		if err != nil {
			return err
		}

		if parentChanged {
			// Try to delete the old application credential owned by the old user.
			// This might not work as the information about this user could be stale,
			// because the user credentials are rotated, the user is not associated to
			// Openstack project anymore or it is deleted.
			if err := m.runGarbageCollection(ctx, oldParentUser, nil); err != nil {
				m.logger.Error(err, "could not delete application credential as the owning user has changed and information about owning user might be stale")
			}
		} else {
			if err := m.runGarbageCollection(ctx, newParentUser, &newAppCredential.id); err != nil {
				return err
			}
		}

		return m.storeApplicationCredential(ctx, newAppCredential, newParentUser)
	}

	if err := m.runGarbageCollection(ctx, newParentUser, &appCredential.id); err != nil {
		return err
	}

	return m.storeApplicationCredential(ctx, appCredential, newParentUser)
}

func (m *Manager) isExpired(appCredential *applicationCredential) bool {
	var (
		now                     = time.Now().UTC()
		creationTime            = appCredential.creationTime
		lifetime                = creationTime.Add(m.config.Lifetime.Duration)
		openstackExpirationTime = creationTime.Add(m.config.OpenstackExpirationPeriod.Duration)
		renewThreshold          = openstackExpirationTime.Add(-m.config.RenewThreshold.Duration)
	)

	if now.After(renewThreshold) {
		return true
	}

	if now.After(lifetime) {
		return true
	}

	return false
}

func (m *Manager) createApplicationCredential(ctx context.Context, parent *parent) (*applicationCredential, error) {
	randomNamePart, err := utils.GenerateRandomString(8)
	if err != nil {
		return nil, err
	}

	var (
		name                    = fmt.Sprintf("%s-%s", m.identifier, randomNamePart)
		description             = fmt.Sprintf("Gardener managed application credential, shoot=%s", m.identifier)
		openstackExpirationTime = time.Now().UTC().Add(m.config.OpenstackExpirationPeriod.Duration).Format(time.RFC3339)
	)

	appCredential, err := parent.identityClient.CreateApplicationCredential(ctx, parent.id, name, description, openstackExpirationTime)
	if err != nil {
		return nil, err
	}

	return &applicationCredential{
		id:       appCredential.ID,
		name:     appCredential.Name,
		password: appCredential.Secret,
	}, nil
}
