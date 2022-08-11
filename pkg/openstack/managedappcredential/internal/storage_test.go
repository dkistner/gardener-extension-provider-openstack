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

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	mockopenstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client/mocks"
	. "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/managedappcredential/internal"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/tokens"

	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var apiNotFound = &apierrors.StatusError{
	ErrStatus: metav1.Status{
		Reason: metav1.StatusReasonNotFound,
	},
}

var _ = Describe("Storage", func() {
	var (
		ctx  context.Context
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		namespace string
		secretKey client.ObjectKey
		secret    *corev1.Secret
	)

	BeforeEach(func() {
		ctx = context.TODO()
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		namespace = "shoot-test-test"
		secret = &corev1.Secret{
			Data: map[string][]byte{},
		}

		secretKey = client.ObjectKey{
			Namespace: namespace,
			Name:      "cloudprovider-application-credential",
		}

	})

	AfterEach(func() { ctrl.Finish() })

	Describe("#ReadAppCredential", func() {
		It("should read app credential secret", func() {
			var (
				appCredentialID = "app-credential-id"
				parentID        = "parent-id"
			)

			secret.Data = map[string][]byte{
				"creationTime":                []byte("2009-11-10T23:00:00Z"),
				"applicationCredentialID":     []byte(appCredentialID),
				"applicationCredentialName":   []byte("app-credential-name"),
				"applicationCredentialSecret": []byte("app-credential-secret"),
				"parentID":                    []byte(parentID),
			}

			c.EXPECT().Get(ctx, secretKey, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(getSecret(secret))

			appCredential, parent, err := NewStorage(c, namespace).ReadAppCredential(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(appCredential.ID).To(Equal(appCredentialID))
			Expect(parent.GetID()).To(Equal(parentID))
		})

		It("should return an error as app credential date format in invalid", func() {
			secret.Data = map[string][]byte{
				"creationTime": []byte("invalid-date-format"),
			}

			c.EXPECT().Get(ctx, secretKey, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(getSecret(secret))

			appCredential, parent, err := NewStorage(c, namespace).ReadAppCredential(ctx)
			Expect(err).To(HaveOccurred())
			Expect(appCredential).To(BeNil())
			Expect(parent).To(BeNil())
		})

		It("should not read app credential secret as none exists", func() {
			c.EXPECT().Get(ctx, secretKey, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(apiNotFound)

			appCredential, parent, err := NewStorage(c, namespace).ReadAppCredential(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(appCredential).To(BeNil())
			Expect(parent).To(BeNil())
		})
	})

	Describe("#DeleteAppCredential", func() {
		var (
			credentials                   *openstack.Credentials
			openstackClientFactoryFactory *mockopenstackclient.MockFactoryFactory
			openstackClientFactory        *mockopenstackclient.MockFactory
			identityClient                *mockopenstackclient.MockIdentity
			parentID                      string
			parent                        *Parent
			appCredential                 *ApplicationCredential
		)

		BeforeEach(func() {
			openstackClientFactoryFactory = mockopenstackclient.NewMockFactoryFactory(ctrl)
			openstackClientFactory = mockopenstackclient.NewMockFactory(ctrl)
			identityClient = mockopenstackclient.NewMockIdentity(ctrl)

			credentials = &openstack.Credentials{}
			parentID = "parent-id"

			openstackClientFactoryFactory.EXPECT().NewFactory(credentials).Return(openstackClientFactory, nil)
			openstackClientFactory.EXPECT().Identity().Return(identityClient, nil)
			identityClient.EXPECT().GetClientUser().Return(&tokens.User{ID: parentID}, nil)

			parent = NewParentFromCredentials(credentials)
			Expect(parent.Init(openstackClientFactoryFactory)).NotTo(HaveOccurred())
			Expect(parent.GetID()).To(Equal(parentID))
			appCredential = &ApplicationCredential{ID: "app-id"}
		})

		It("should store app credential in existing secret", func() {
			c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(nil)

			Expect(NewStorage(c, namespace).StoreAppCredential(ctx, appCredential, parent)).NotTo(HaveOccurred())
		})

		It("should store app credential in non-existing secret", func() {
			c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(apiNotFound)
			c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(nil)

			Expect(NewStorage(c, namespace).StoreAppCredential(ctx, appCredential, parent)).NotTo(HaveOccurred())
		})
	})

	Describe("#DeleteAppCredential", func() {
		It("should delete app credential secret", func() {
			c.EXPECT().Get(ctx, secretKey, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(getSecret(secret))
			c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).Return(nil)
			c.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(nil)

			Expect(NewStorage(c, namespace).DeleteAppCredential(ctx)).NotTo(HaveOccurred())
		})

		It("should not try to delete app credential secret as it not exists", func() {
			c.EXPECT().Get(ctx, secretKey, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(apiNotFound)

			Expect(NewStorage(c, namespace).DeleteAppCredential(ctx)).NotTo(HaveOccurred())
		})

		It("should fail to patch app credential secret as it not exists", func() {
			c.EXPECT().Get(ctx, secretKey, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(getSecret(secret))
			c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).Return(apiNotFound)

			Expect(NewStorage(c, namespace).DeleteAppCredential(ctx)).NotTo(HaveOccurred())
		})

		It("should fail to delete app credential secret as it not exists", func() {
			c.EXPECT().Get(ctx, secretKey, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(getSecret(secret))
			c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).Return(nil)
			c.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(apiNotFound)

			Expect(NewStorage(c, namespace).DeleteAppCredential(ctx)).NotTo(HaveOccurred())
		})
	})

	Describe("#UpdateParentSecret", func() {
		var (
			parentSecret string
			parent       *Parent
		)

		BeforeEach(func() {
			parentSecret = "parent-secret"
			parent = NewParentFromCredentials(&openstack.Credentials{Password: parentSecret})
		})

		It("should update the parent secret", func() {
			secret.Data = map[string][]byte{
				"parentSecret": []byte(parentSecret),
			}

			c.EXPECT().Get(ctx, secretKey, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(getSecret(secret))
			c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).Return(nil)

			Expect(NewStorage(c, namespace).UpdateParentSecret(ctx, parent)).NotTo(HaveOccurred())
		})

		It("should fail to update parent secret as app credential secret does not exists", func() {
			c.EXPECT().Get(ctx, secretKey, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(getSecretNotFound())

			Expect(NewStorage(c, namespace).UpdateParentSecret(ctx, parent)).To(HaveOccurred())
		})
	})
})

func getSecret(secret *corev1.Secret) func(context.Context, client.ObjectKey, *corev1.Secret) error {
	return func(_ context.Context, key client.ObjectKey, obj *corev1.Secret) error {
		*obj = *secret
		return nil
	}
}

func getSecretNotFound() func(context.Context, client.ObjectKey, *corev1.Secret) error {
	return func(_ context.Context, key client.ObjectKey, obj *corev1.Secret) error {
		return apiNotFound
	}
}
