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
	"context"
	"fmt"
	"strings"
	"time"

	controllerconfig "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"

	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/utils/clock"
)

// ApplicationCredential represent Gardener managed Openstack application credential.
type ApplicationCredential struct {
	// ID is the ID of the application credential.
	ID           string
	name         string
	secret       string
	creationTime time.Time
}

// NewApplicationCredential create a new Gardener managed application credential
// owned by a given parent Openstack user.
func NewApplicationCredential(ctx context.Context, parent *Parent, shootName, nameSuffix string, creationTime clock.Clock, config *controllerconfig.ApplicationCredentialConfig) (*ApplicationCredential, error) {
	var (
		creationTimeUTC         = creationTime.Now().UTC()
		name                    = fmt.Sprintf("%s-%s", shootName, nameSuffix)
		description             = fmt.Sprintf("Gardener managed application credential, shoot=%s", shootName)
		openstackExpirationTime = creationTimeUTC.Add(config.OpenstackExpirationPeriod.Duration).Format(time.RFC3339)
	)

	appCredential, err := parent.GetClient().CreateApplicationCredential(ctx, parent.GetID(), name, description, openstackExpirationTime)
	if err != nil {
		return nil, err
	}

	return &ApplicationCredential{
		ID:           appCredential.ID,
		name:         appCredential.Name,
		secret:       appCredential.Secret,
		creationTime: creationTimeUTC,
	}, nil
}

// RunGarbageCollection will remove existing Gardener manahed application credentials
// belonging to a given Openstack user.
// The in-use application credential can be omited by passing its ID.
func RunGarbageCollection(ctx context.Context, parent *Parent, managedApplicationCredentialID *string, shootName string) error {
	appCredentialList, err := parent.GetClient().ListApplicationCredentials(ctx, parent.GetID())
	if err != nil {
		return err
	}

	var errorList []error
	for _, ac := range appCredentialList {
		// Ignore application credentials which name is not matching to the Shoot name.
		if !strings.HasPrefix(ac.Name, shootName) {
			continue
		}

		// Skip the is-use application credential.
		if managedApplicationCredentialID != nil && ac.ID == *managedApplicationCredentialID {
			continue
		}

		if err := parent.GetClient().DeleteApplicationCredential(ctx, parent.GetID(), ac.ID); err != nil {
			errorList = append(errorList, fmt.Errorf("could not delete application credential %q owned by user %q: %w", ac.ID, parent.GetID(), err))
		}
	}

	return errors.Flatten(errors.NewAggregate(errorList))
}

// IsExpired determine if the application credential is expired.
func (a *ApplicationCredential) IsExpired(clock clock.Clock, config *controllerconfig.ApplicationCredentialConfig) bool {
	var (
		nowUTC                  = clock.Now().UTC()
		lifetime                = a.creationTime.Add(config.Lifetime.Duration)
		openstackExpirationTime = a.creationTime.Add(config.OpenstackExpirationPeriod.Duration)
		renewThreshold          = openstackExpirationTime.Add(-config.RenewThreshold.Duration)
	)

	if nowUTC.After(renewThreshold) {
		return true
	}

	if nowUTC.After(lifetime) {
		return true
	}

	return false
}

// GetCredentials return an Openstack credential bundle for the application credential.
func (a *ApplicationCredential) GetCredentials(parent *Parent) *openstack.Credentials {
	return &openstack.Credentials{
		ApplicationCredentialID:     a.ID,
		ApplicationCredentialName:   a.name,
		ApplicationCredentialSecret: a.secret,

		// Auth information inherited from the parent Openstack user.
		AuthURL:    parent.credentials.AuthURL,
		DomainName: parent.credentials.DomainName,
		TenantName: parent.credentials.TenantName,
	}
}
