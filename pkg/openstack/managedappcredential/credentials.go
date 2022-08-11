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

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/managedappcredential/internal"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AppCredentialAuth contain auth information about a managed application credential.
type AppCredentialAuth struct {
	// Credentials contain the openstack credentials of the application credential.
	Credentials *openstack.Credentials
	// SecretRef contain the secret reference that hold the application credential auth information.
	SecretRef *corev1.SecretReference
}

// GetCredentials return the credentials and the secret reference
// for the in-use application credential.
// If no application credential exits nil will be returned.
func GetCredentials(ctx context.Context, client client.Client, namespace string) (*AppCredentialAuth, error) {
	appCredential, parentUser, err := internal.NewStorage(client, namespace).ReadAppCredential(ctx)
	if err != nil {
		return nil, err
	}

	if appCredential == nil || parentUser == nil {
		return nil, nil
	}

	return &AppCredentialAuth{
		Credentials: appCredential.GetCredentials(parentUser),
		SecretRef:   getSecretRef(namespace),
	}, nil
}

func getSecretRef(namespace string) *corev1.SecretReference {
	return &corev1.SecretReference{
		Name:      internal.ApplicationCredentialSecretName,
		Namespace: namespace,
	}
}
