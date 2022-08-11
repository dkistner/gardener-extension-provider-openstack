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
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	mockopenstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client/mocks"
	. "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/managedappcredential/internal"

	"github.com/golang/mock/gomock"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/tokens"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
)

var _ = Describe("Parent", func() {

	Describe("#Init, #GetID, #GetClient", func() {
		var (
			credentials                   *openstack.Credentials
			parentID                      string
			ctrl                          *gomock.Controller
			openstackClientFactoryFactory *mockopenstackclient.MockFactoryFactory
			openstackClientFactory        *mockopenstackclient.MockFactory
			identityClient                *mockopenstackclient.MockIdentity
		)

		BeforeEach(func() {
			credentials = &openstack.Credentials{}
			parentID = "parent-id"
			ctrl = gomock.NewController(GinkgoT())
			openstackClientFactoryFactory = mockopenstackclient.NewMockFactoryFactory(ctrl)
			openstackClientFactory = mockopenstackclient.NewMockFactory(ctrl)
			identityClient = mockopenstackclient.NewMockIdentity(ctrl)
		})

		AfterEach(func() { ctrl.Finish() })

		Context("#GetID", func() {
			It("should panic as parent is not initalized", func() {
				parent := NewParentFromCredentials(credentials)
				Expect(func() { parent.GetID() }).Should(Panic())
			})

			It("should return id as parent is initalized", func() {
				openstackClientFactoryFactory.EXPECT().NewFactory(credentials).Return(openstackClientFactory, nil)
				openstackClientFactory.EXPECT().Identity().Return(identityClient, nil)
				identityClient.EXPECT().GetClientUser().Return(&tokens.User{ID: parentID}, nil)

				parent := NewParentFromCredentials(credentials)
				Expect(parent.Init(openstackClientFactoryFactory)).NotTo(HaveOccurred())
				Expect(parent.GetID()).To(Equal(parentID))
			})
		})

		Context("#GetClient", func() {
			It("should panic as parent is not initalized", func() {
				parent := NewParentFromCredentials(credentials)
				Expect(func() { parent.GetClient() }).Should(Panic())
			})

			It("should return identity client as parent is initalized", func() {
				openstackClientFactoryFactory.EXPECT().NewFactory(credentials).Return(openstackClientFactory, nil)
				openstackClientFactory.EXPECT().Identity().Return(identityClient, nil)
				identityClient.EXPECT().GetClientUser().Return(&tokens.User{ID: parentID}, nil)

				parent := NewParentFromCredentials(credentials)
				Expect(parent.Init(openstackClientFactoryFactory)).NotTo(HaveOccurred())

				Expect(parent.GetClient()).To(Equal(identityClient))
			})
		})

	})

	var _ = DescribeTable("#IsEqual",
		func(p1, p2 *openstack.Credentials, matcher gomegatypes.GomegaMatcher) {
			parent1 := NewParentFromCredentials(p1)
			parent2 := NewParentFromCredentials(p2)
			Expect(parent1.IsEqual(parent2)).To(matcher)
		},
		Entry("should return true as parent are equal",
			&openstack.Credentials{DomainName: "test", Username: "p1", Password: "p1-secret"},
			&openstack.Credentials{DomainName: "test", Username: "p1", Password: "p1-secret"},
			BeTrue(),
		),
		Entry("should return true even if parents have different passwords",
			&openstack.Credentials{DomainName: "test", Username: "p1", Password: "p1-secret"},
			&openstack.Credentials{DomainName: "test", Username: "p1", Password: "different"},
			BeTrue(),
		),
		Entry("should return false as parent are different",
			&openstack.Credentials{DomainName: "test", Username: "p1", Password: "p1-secret"},
			&openstack.Credentials{DomainName: "test", Username: "p2", Password: "p2-secret"},
			BeFalse(),
		),
	)

	var _ = DescribeTable("#HaveEqualSecrets",
		func(p1, p2 *openstack.Credentials, matcher gomegatypes.GomegaMatcher) {
			parent1 := NewParentFromCredentials(p1)
			parent2 := NewParentFromCredentials(p2)
			Expect(parent1.HaveEqualSecrets(parent2)).To(matcher)
		},
		Entry("should return true as secrets/passwords are equal",
			&openstack.Credentials{DomainName: "test", Username: "p1", Password: "p1-secret"},
			&openstack.Credentials{DomainName: "test", Username: "p1", Password: "p1-secret"},
			BeTrue(),
		),
		Entry("should return false as secrets/passwords are false",
			&openstack.Credentials{DomainName: "test", Username: "p1", Password: "p1-secret"},
			&openstack.Credentials{DomainName: "test", Username: "p1", Password: "different"},
			BeFalse(),
		),
	)

	var _ = DescribeTable("#IsApplicationCredential",
		func(credentials *openstack.Credentials, matcher gomegatypes.GomegaMatcher) {
			parent := NewParentFromCredentials(credentials)
			Expect(parent.IsApplicationCredential()).To(matcher)
		},
		Entry("should return false as parent user is not an application credential",
			&openstack.Credentials{DomainName: "test", Username: "p1", Password: "p1-secret"},
			BeFalse(),
		),
		Entry("should return true as parent credentials contain an application credential id",
			&openstack.Credentials{ApplicationCredentialID: "app-id"},
			BeTrue(),
		),
		Entry("should return true as parent credentials contain an application credential name",
			&openstack.Credentials{ApplicationCredentialName: "app-name"},
			BeTrue(),
		),
	)

})
