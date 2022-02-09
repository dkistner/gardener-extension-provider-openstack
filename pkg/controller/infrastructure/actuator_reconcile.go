// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package infrastructure

import (
	"context"
	"fmt"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal/infrastructure"
	appcredential "github.com/gardener/gardener-extension-provider-openstack/pkg/internal/managedappcredential"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (a *actuator) Reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) error {
	logger := a.logger.WithValues("infrastructure", client.ObjectKeyFromObject(infra), "operation", "reconcile")
	return a.reconcile(ctx, logger, infra, cluster, terraformer.StateConfigMapInitializerFunc(terraformer.CreateState))
}

func (a *actuator) reconcile(ctx context.Context, logger logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster, stateInitializer terraformer.StateConfigMapInitializer) error {
	config, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		return err
	}

	terraformFiles, err := infrastructure.RenderTerraformerTemplate(infra, config, cluster)
	if err != nil {
		return err
	}

	// need to known if application credentials are used
	credentials, err := openstack.GetCredentials(ctx, a.Client(), infra.Spec.SecretRef, false)
	if err != nil {
		return err
	}

	managedAppCredential, err := appcredential.NewManagedApplicationCredential(ctx, a.Client(), infra.Name, a.logger)
	if err != nil {
		return err
	}
	managedAppCredential.InjectConfig(a.managedAppCredentialConfig)
	managedAppCredential.InjectParentUserCredentials(credentials)

	credentialsSecretRef := infra.Spec.SecretRef
	if managedAppCredential.IsEnabled() {
		if err := managedAppCredential.Ensure(ctx); err != nil {
			return err
		}

		if err := managedAppCredential.CleanupOrphans(ctx); err != nil {
			return err
		}

		credentialsSecretRef = managedAppCredential.GetSecretReference()
		credentials, err = managedAppCredential.GetCredentials()
		if err != nil {
			return err
		}
	} else {
		managedAppCredential.DeleteIfExists(ctx)
	}

	tf, err := internal.NewTerraformerWithAuth(logger, a.RESTConfig(), infrastructure.TerraformerPurpose, infra, credentials, &credentialsSecretRef)
	if err != nil {
		return err
	}

	if err := tf.
		InitializeWith(ctx, terraformer.DefaultInitializer(a.Client(), terraformFiles.Main, terraformFiles.Variables, terraformFiles.TFVars, stateInitializer)).
		Apply(ctx); err != nil {

		return fmt.Errorf("failed to apply the terraform config: %w", err)
	}

	return a.updateProviderStatus(ctx, tf, infra, config)
}
