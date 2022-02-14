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

package managedappcredential_test

import (
	"context"
	"time"

	controllerconfig "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config"
	. "github.com/gardener/gardener-extension-provider-openstack/pkg/internal/managedappcredential"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("ManagedApplicationCredential", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		ctx                     context.Context
		technicalName           string
		appCredentialSecretName string
		appCredentialSecretKey  client.ObjectKey

		appCredentialSecret *corev1.Secret

		appCredentialConfig *controllerconfig.ApplicationCrendentialConfig
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		ctx = context.TODO()

		technicalName = "foo"

		appCredentialSecretName = "cloudprovider-application-credential"
		appCredentialSecretKey = client.ObjectKey{Namespace: technicalName, Name: appCredentialSecretName}

		appCredentialSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      appCredentialSecretName,
				Namespace: technicalName,
			},
		}

		appCredentialConfig = &controllerconfig.ApplicationCrendentialConfig{
			Enabled:          true,
			Lifetime:         &metav1.Duration{Duration: time.Hour * 24},
			ExpirationPeriod: &metav1.Duration{Duration: time.Hour * 24},
		}
		_ = appCredentialConfig // TODO remove

	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should create new managed application credential while an application credential secret exists", func() {
		expectAppCredentialSecretExists(c, appCredentialSecretKey, appCredentialSecret)

		_, err := NewManagedApplicationCredential(ctx, c, technicalName, nil)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should create new managed application credential while an application credential secret not exists", func() {
		expectAppCredentialSecretNotExists(c, appCredentialSecretKey)

		_, err := NewManagedApplicationCredential(ctx, c, technicalName, nil)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("#InjectParentUserCredentials", func() {
		expectAppCredentialSecretExists(c, appCredentialSecretKey, appCredentialSecret)

		managedAppCredential, err := NewManagedApplicationCredential(ctx, c, technicalName, nil)
		Expect(err).NotTo(HaveOccurred())

		// TODO another mock for locking up the id from parent user is reqired to call here.
		managedAppCredential.InjectParentUserCredentials(&openstack.Credentials{
			Username:   "asd",
			Password:   "asd",
			DomainName: "asd",
			TenantName: "asd",
			AuthURL:    "asd",
		})

	})
})

func expectAppCredentialSecretExists(c *mockclient.MockClient, key client.ObjectKey, secret *corev1.Secret) {
	c.EXPECT().Get(context.TODO(), key, &corev1.Secret{}).DoAndReturn(
		func(result runtime.Object) interface{} {
			return func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *corev1.Secret:
					*obj.(*corev1.Secret) = *result.(*corev1.Secret)
				}
				return nil
			}
		}(secret),
	)
}

func expectAppCredentialSecretNotExists(c *mockclient.MockClient, key client.ObjectKey) {
	c.EXPECT().Get(context.TODO(), key, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(
		&apierrors.StatusError{
			ErrStatus: metav1.Status{
				Reason: metav1.StatusReasonNotFound,
			},
		},
	)
}
