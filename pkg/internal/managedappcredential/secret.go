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
	"strconv"
	"time"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	applicationCredentialSecretName                  = "cloudprovider-application-credential"
	applicationCredentialSecretCreationTime          = "creationTime"
	applicationCredentialSecretParentID              = "parentID"
	applicationCredentialSecretParentName            = "parentName"
	applicationCredentialSecretParentSecret          = "parentSecret"
	applicationCredentialSecretParentIsAppCredential = "parentIsApplicationCredential"
)

func (m *ManagedApplicationCredential) getApplicationCredentialSecret(ctx context.Context) (*corev1.Secret, error) {
	var secret = corev1.Secret{}
	if err := m.k8sClient.Get(ctx, kutil.Key(m.technicalName, applicationCredentialSecretName), &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return &secret, nil
}

func (m *ManagedApplicationCredential) ensureApplicationCredentialSecret(ctx context.Context, parent *parent, id, name, secret string) (*corev1.Secret, error) {
	appCredentialSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      applicationCredentialSecretName,
			Namespace: m.technicalName,
		},
		Data: map[string][]byte{
			openstack.ApplicationCredentialID:                []byte(id),
			openstack.ApplicationCredentialName:              []byte(name),
			openstack.ApplicationCredentialSecret:            []byte(secret),
			applicationCredentialSecretCreationTime:          []byte(time.Now().UTC().Format(time.RFC1123)),
			applicationCredentialSecretParentID:              []byte(parent.id),
			applicationCredentialSecretParentSecret:          []byte(parent.secret),
			applicationCredentialSecretParentName:            []byte(parent.name),
			applicationCredentialSecretParentIsAppCredential: []byte(strconv.FormatBool(parent.isApplicationCredential)),
		},
	}

	if err := m.k8sClient.Update(ctx, appCredentialSecret); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}

		if err := m.k8sClient.Create(ctx, appCredentialSecret); err != nil {
			return nil, err
		}
	}

	return appCredentialSecret, nil
}

func (m *ManagedApplicationCredential) deleteApplicationCredentialSecret(ctx context.Context) error {
	appCredentialSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      applicationCredentialSecretName,
			Namespace: m.technicalName,
		},
	}

	if err := m.k8sClient.Delete(ctx, appCredentialSecret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}
