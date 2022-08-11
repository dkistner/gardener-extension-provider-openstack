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

package internal_test

import (
	"context"
	"fmt"
	"time"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	mockopenstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client/mocks"
	. "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/managedappcredential/internal"

	"github.com/golang/mock/gomock"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/applicationcredentials"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/tokens"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclock "k8s.io/utils/clock/testing"
)

var _ = Describe("Application Credential", func() {
	var (
		appCredentialID   string
		appCredentialName string
		authURL           string
		domainName        string
		parentID          string
		shootName         string
		tenantName        string
		nameSuffix        string

		ctx         context.Context
		credentials *openstack.Credentials
		parent      *Parent

		ctrl                          *gomock.Controller
		openstackClientFactoryFactory *mockopenstackclient.MockFactoryFactory
		openstackClientFactory        *mockopenstackclient.MockFactory
		identityClient                *mockopenstackclient.MockIdentity
	)

	BeforeEach(func() {
		appCredentialID = "app-id-1"
		authURL = "auth-url"
		domainName = "domain-name"
		parentID = "parent-id"
		shootName = "shoot-test"
		tenantName = "tenant-name"
		nameSuffix = "id-1"
		appCredentialName = fmt.Sprintf("%s-%s", shootName, nameSuffix)

		credentials = &openstack.Credentials{
			AuthURL:    authURL,
			DomainName: domainName,
			TenantName: tenantName,
		}

		ctx = context.TODO()
		ctrl = gomock.NewController(GinkgoT())
		openstackClientFactoryFactory = mockopenstackclient.NewMockFactoryFactory(ctrl)
		openstackClientFactory = mockopenstackclient.NewMockFactory(ctrl)
		identityClient = mockopenstackclient.NewMockIdentity(ctrl)

		openstackClientFactoryFactory.EXPECT().NewFactory(credentials).Return(openstackClientFactory, nil)
		openstackClientFactory.EXPECT().Identity().Return(identityClient, nil)
		identityClient.EXPECT().GetClientUser().Return(&tokens.User{ID: parentID}, nil)

		parent = NewParentFromCredentials(credentials)
		Expect(parent.Init(openstackClientFactoryFactory)).NotTo(HaveOccurred())
	})

	AfterEach(func() { ctrl.Finish() })

	Describe("#RunGarbageCollection", func() {
		It("should garbage collect app credential", func() {
			identityClient.EXPECT().ListApplicationCredentials(ctx, parentID).Return([]applicationcredentials.ApplicationCredential{
				{Name: appCredentialName, ID: appCredentialID},
			}, nil)
			identityClient.EXPECT().DeleteApplicationCredential(ctx, parentID, appCredentialID).Return(nil)

			Expect(RunGarbageCollection(ctx, parent, nil, shootName)).NotTo(HaveOccurred())
		})

		It("should not garbage collect app credential as it is in use", func() {
			identityClient.EXPECT().ListApplicationCredentials(ctx, parentID).Return([]applicationcredentials.ApplicationCredential{
				{Name: appCredentialName, ID: appCredentialID},
			}, nil)

			Expect(RunGarbageCollection(ctx, parent, &appCredentialID, shootName)).NotTo(HaveOccurred())
		})

		It("should not garbage collect app credential as name is not matching", func() {
			appCredentialName = "not-matching-app-1"

			identityClient.EXPECT().ListApplicationCredentials(ctx, parentID).Return([]applicationcredentials.ApplicationCredential{
				{Name: appCredentialName, ID: appCredentialID},
			}, nil)

			Expect(RunGarbageCollection(ctx, parent, nil, shootName)).NotTo(HaveOccurred())
		})

		It("should return error if one app credential cannot be deleted", func() {
			var (
				appCredentialTwoName = fmt.Sprintf("%s-app-2", shootName)
				appCredentialTwoID   = "app-id-2"
			)

			appCredentialName = fmt.Sprintf("%s-%s", shootName, nameSuffix)

			identityClient.EXPECT().ListApplicationCredentials(ctx, parentID).Return([]applicationcredentials.ApplicationCredential{
				{Name: appCredentialName, ID: appCredentialID},
				{Name: appCredentialTwoName, ID: appCredentialTwoID},
			}, nil)

			identityClient.EXPECT().DeleteApplicationCredential(ctx, parentID, appCredentialID).Return(fmt.Errorf("failed to delete %s", appCredentialName))
			identityClient.EXPECT().DeleteApplicationCredential(ctx, parentID, appCredentialTwoID).Return(nil)

			Expect(RunGarbageCollection(ctx, parent, nil, shootName)).To(HaveOccurred())
		})
	})

	Describe("#IsExpired, #GetCredentials", func() {
		var (
			appCredential          *ApplicationCredential
			appCredentialFakeClock *testclock.FakeClock
			appCredentialSecret    string
			cfg                    *config.ApplicationCredentialConfig
		)

		BeforeEach(func() {
			appCredentialFakeClock = testclock.NewFakeClock(time.Now())
			appCredentialSecret = "app-1-secret"

			cfg = &config.ApplicationCredentialConfig{
				OpenstackExpirationPeriod: &metav1.Duration{Duration: 4 * time.Hour},
			}

			identityClient.EXPECT().CreateApplicationCredential(
				ctx,
				parentID,
				appCredentialName,
				fmt.Sprintf("Gardener managed application credential, shoot=%s", shootName),
				appCredentialFakeClock.Now().UTC().Add(cfg.OpenstackExpirationPeriod.Duration).Format(time.RFC3339),
			).Return(&applicationcredentials.ApplicationCredential{
				ID:     appCredentialID,
				Name:   appCredentialName,
				Secret: appCredentialSecret,
			}, nil)

			var err error
			appCredential, err = NewApplicationCredential(ctx, parent, shootName, nameSuffix, appCredentialFakeClock, cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(appCredential).NotTo(BeNil())
		})

		Context("#IsExpired", func() {
			It("should return false as application credential is not expired", func() {
				cfg.Lifetime = &metav1.Duration{Duration: time.Hour}
				cfg.RenewThreshold = &metav1.Duration{Duration: time.Minute}

				fakeClock := testclock.NewFakeClock(time.Now())

				Expect(appCredential.IsExpired(fakeClock, cfg)).To(BeFalse())
			})

			It("should return true as application credential lifetime is expired", func() {
				cfg.Lifetime = &metav1.Duration{Duration: time.Hour}
				cfg.RenewThreshold = &metav1.Duration{Duration: time.Hour}

				fakeClock := testclock.NewFakeClock(time.Now())
				fakeClock.Sleep(2 * time.Hour)

				Expect(appCredential.IsExpired(fakeClock, cfg)).To(BeTrue())
			})

			It("should return true as application credential reached renew threshold", func() {
				cfg.Lifetime = &metav1.Duration{Duration: 8 * time.Hour}
				cfg.RenewThreshold = &metav1.Duration{Duration: 3 * time.Hour}

				fakeClock := testclock.NewFakeClock(time.Now())
				fakeClock.Sleep(3 * time.Hour)

				Expect(appCredential.IsExpired(fakeClock, cfg)).To(BeTrue())
			})
		})

		Context("#GetCredentials", func() {
			It("should return valid credentials", func() {
				expectedCredentials := &openstack.Credentials{
					ApplicationCredentialID:     appCredentialID,
					ApplicationCredentialName:   appCredentialName,
					ApplicationCredentialSecret: appCredentialSecret,
					AuthURL:                     authURL,
					DomainName:                  domainName,
					TenantName:                  tenantName,
				}

				Expect(appCredential.GetCredentials(parent)).To(Equal(expectedCredentials))
			})
		})

	})
})
