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
	"flag"
	"fmt"
	"time"

	controllerconfig "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/features"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/managedappcredential"

	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/test/framework"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/applicationcredentials"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	ctx     context.Context
	testEnv *envtest.Environment
	c       client.Client

	credentials      *openstack.Credentials
	testUserRotation bool
)

var _ = BeforeSuite(func() {
	flag.Parse()
	validateMandatoryFlags()
	testUserRotation = validateRotationUserFlags()

	ctx = context.Background()
	features.RegisterExtensionFeatureGate()

	// enable manager logs
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))

	mgrContext, mgrCancel := context.WithCancel(ctx)
	DeferCleanup(func() {
		defer func() {
			By("stopping manager")
			mgrCancel()
		}()

		By("running cleanup actions")
		framework.RunCleanupActions()

		By("stopping test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	By("starting test environment")
	testEnv = &envtest.Environment{
		UseExistingCluster: pointer.BoolPtr(true),
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	By("setup manager")
	mgr, err := manager.New(cfg, manager.Options{
		MetricsBindAddress: "0",
	})
	Expect(err).NotTo(HaveOccurred())

	By("start manager")
	go func() {
		Expect(mgr.Start(mgrContext)).NotTo(HaveOccurred())
	}()

	c = mgr.GetClient()
	Expect(c).NotTo(BeNil())

	credentials = &openstack.Credentials{
		AuthURL:    *mandatoryFlags[authURLKey],
		DomainName: *mandatoryFlags[domainNameKey],
		TenantName: *mandatoryFlags[tenantNameKey],
		Username:   *mandatoryFlags[usernameKey],
		Password:   *mandatoryFlags[passwordKey],
	}
})

var _ = Describe("Managed Application Credential tests", func() {
	var config *controllerconfig.ApplicationCredentialConfig

	BeforeEach(func() {
		config = &controllerconfig.ApplicationCredentialConfig{
			Lifetime:                  &metav1.Duration{Duration: time.Hour},
			OpenstackExpirationPeriod: &metav1.Duration{Duration: 4 * time.Hour},
			RenewThreshold:            &metav1.Duration{Duration: time.Hour},
		}

		managedappcredential.Config = *config
		features.ExtensionFeatureGate.Set(fmt.Sprintf("%s=true", features.ManagedApplicationCredential))
	})

	AfterEach(func() {
		framework.RunCleanupActions()
	})

	It("should ensure and delete a managed application credential", func() {
		var (
			namespace = prepareTestNamespace(ctx, c)
			shootName = generateRandomName(shootNamePrefix)
		)

		manager := managedappcredential.NewManager(
			openstackclient.FactoryFactoryFunc(openstackclient.NewOpenstackClientFromCredentials),
			c,
			namespace,
			shootName,
		)

		By("ensure managed application credential")
		_, err := manager.Ensure(ctx, credentials)
		Expect(err).NotTo(HaveOccurred())

		_ = verifyApplicationCredential(ctx, c, namespace, credentials)

		By("delete managed application credential")
		Expect(manager.Delete(ctx, credentials)).NotTo(HaveOccurred())
	})

	It("should ensure and delete a managed application credential with lifetime expiration", func() {
		var (
			namespace = prepareTestNamespace(ctx, c)
			shootName = generateRandomName(shootNamePrefix)
		)

		config.Lifetime = &metav1.Duration{Duration: time.Second}
		managedappcredential.Config = *config

		manager := managedappcredential.NewManager(
			openstackclient.FactoryFactoryFunc(openstackclient.NewOpenstackClientFromCredentials),
			c,
			namespace,
			shootName,
		)

		By("ensure managed application credential")
		_, err := manager.Ensure(ctx, credentials)
		Expect(err).NotTo(HaveOccurred())

		appCredentialID := verifyApplicationCredential(ctx, c, namespace, credentials)

		By("wait 10s to expire the application credential lifetime")
		time.Sleep(10 * time.Second)

		By("ensure managed application credential after lifetime expired")
		_, err = manager.Ensure(ctx, credentials)
		Expect(err).NotTo(HaveOccurred())

		appCredentialID2 := verifyApplicationCredential(ctx, c, namespace, credentials)
		Expect(appCredentialID).NotTo(Equal(appCredentialID2))

		By("delete managed application credential")
		Expect(manager.Delete(ctx, credentials)).NotTo(HaveOccurred())
	})

	It("should ensure and delete a managed application credential with parent change", func() {
		if !testUserRotation {
			Skip("skip as no user for rotation is provided")
		}

		var (
			namespace = prepareTestNamespace(ctx, c)
			shootName = generateRandomName(shootNamePrefix)
		)

		manager := managedappcredential.NewManager(
			openstackclient.FactoryFactoryFunc(openstackclient.NewOpenstackClientFromCredentials),
			c,
			namespace,
			shootName,
		)

		By("ensure managed application credential")
		_, err := manager.Ensure(ctx, credentials)
		Expect(err).NotTo(HaveOccurred())

		appCredentialID := verifyApplicationCredential(ctx, c, namespace, credentials)

		credentialsNewParent := &openstack.Credentials{
			AuthURL:    credentials.AuthURL,
			DomainName: credentials.DomainName,
			TenantName: credentials.TenantName,
			Username:   *optionalFlags[usernameRotationKey],
			Password:   *optionalFlags[passwordRotationKey],
		}

		By("ensure managed application credential with changed parent user")
		_, err = manager.Ensure(ctx, credentialsNewParent)
		Expect(err).NotTo(HaveOccurred())

		appCredentialID2 := verifyApplicationCredential(ctx, c, namespace, credentialsNewParent)
		Expect(appCredentialID).NotTo(Equal(appCredentialID2))

		By("delete managed application credential")
		Expect(manager.Delete(ctx, credentialsNewParent)).NotTo(HaveOccurred())
	})
})

func verifyApplicationCredential(ctx context.Context, c client.Client, namespace string, credentials *openstack.Credentials) string {
	By("verify application credential secret")
	var appCredentialSecret = &corev1.Secret{}
	err := c.Get(ctx, client.ObjectKey{Name: "cloudprovider-application-credential", Namespace: namespace}, appCredentialSecret)
	Expect(err).NotTo(HaveOccurred())

	var (
		parentUserID            = string(appCredentialSecret.Data["parentID"])
		applicationCredentialID = string(appCredentialSecret.Data["applicationCredentialID"])
	)
	Expect(parentUserID).NotTo(BeEmpty())
	Expect(applicationCredentialID).NotTo(BeEmpty())

	framework.AddCleanupAction(func() {
		By("ensure orphan test application credential secret is deleted")
		patch := client.MergeFrom(appCredentialSecret.DeepCopy())
		appCredentialSecret.ObjectMeta.Finalizers = []string{}
		Expect(client.IgnoreNotFound(c.Patch(ctx, appCredentialSecret, patch))).To(Succeed())
		Expect(client.IgnoreNotFound(c.Delete(ctx, appCredentialSecret))).To(Succeed())
	})

	By("prepare openstack identity/application credential client")
	identitiyClient := newOpenstackClient(credentials)

	By("verify application credential exists on infrastructure")
	appCredential, err := applicationcredentials.Get(identitiyClient, parentUserID, applicationCredentialID).Extract()
	Expect(err).NotTo(HaveOccurred())
	Expect(appCredential.ID).To(Equal(applicationCredentialID))

	framework.AddCleanupAction(func() {
		By("ensure orphan test application credential is deleted")
		Expect(openstackclient.IgnoreNotFoundError(
			applicationcredentials.Delete(identitiyClient, parentUserID, applicationCredentialID).ExtractErr(),
		)).To(Succeed())
	})

	return applicationCredentialID
}

func prepareTestNamespace(ctx context.Context, c client.Client) string {
	var (
		namespaceName = generateRandomName(namespacePrefix)
		namespace     = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespaceName,
			},
		}
	)

	framework.AddCleanupAction(func() {
		By("delete test namespace")
		Expect(client.IgnoreNotFound(c.Delete(ctx, namespace))).To(Succeed())
	})

	By("create namespace for test execution")
	Expect(c.Create(ctx, namespace)).NotTo(HaveOccurred())

	return namespaceName
}
