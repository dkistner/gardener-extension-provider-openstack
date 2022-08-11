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
	"time"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ApplicationCredentialSecretName is the name of the secret that host the application credential information.
	ApplicationCredentialSecretName = "cloudprovider-application-credential"

	applicationCredentialSecretCreationTime = "creationTime"
	applicationCredentialSecretParentID     = "parentID"
	applicationCredentialSecretParentName   = "parentName"
	applicationCredentialSecretParentSecret = "parentSecret"
	finalizerName                           = "extensions.gardener.cloud/managed-application-credential"
)

type storage struct {
	namespace string
	client    client.Client
}

func NewStorage(client client.Client, namespace string) *storage {
	return &storage{
		namespace: namespace,
		client:    client,
	}
}

// ReadAppCredential read the application credential information from the secret in the namespace passed to the storage.
func (s *storage) ReadAppCredential(ctx context.Context) (*ApplicationCredential, *Parent, error) {
	secret, err := s.getAppCredentialSecret(ctx)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	creationTime, err := time.Parse(time.RFC3339, readSecretKey(secret, applicationCredentialSecretCreationTime))
	if err != nil {
		return nil, nil, err
	}

	return &ApplicationCredential{
		ID:           readSecretKey(secret, openstack.ApplicationCredentialID),
		name:         readSecretKey(secret, openstack.ApplicationCredentialName),
		secret:       readSecretKey(secret, openstack.ApplicationCredentialSecret),
		creationTime: creationTime,
	}, newParentFromSecret(secret), nil
}

// StoreAppCredential save the application credential information in the namespace passed to the storage.
func (s *storage) StoreAppCredential(ctx context.Context, appCredential *ApplicationCredential, parent *Parent) error {
	var secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:       ApplicationCredentialSecretName,
			Namespace:  s.namespace,
			Finalizers: []string{finalizerName},
		},
		Data: map[string][]byte{
			openstack.ApplicationCredentialID:       []byte(appCredential.ID),
			openstack.ApplicationCredentialName:     []byte(appCredential.name),
			openstack.ApplicationCredentialSecret:   []byte(appCredential.secret),
			applicationCredentialSecretCreationTime: []byte(time.Now().UTC().Format(time.RFC3339)),

			// parent user data
			applicationCredentialSecretParentID:     []byte(parent.GetID()),
			applicationCredentialSecretParentSecret: []byte(parent.credentials.Password),
			applicationCredentialSecretParentName:   []byte(parent.credentials.Username),
			openstack.DomainName:                    []byte(parent.credentials.DomainName),
			openstack.AuthURL:                       []byte(parent.credentials.AuthURL),
			openstack.TenantName:                    []byte(parent.credentials.TenantName),
		},
	}

	if err := s.client.Update(ctx, secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		if err := s.client.Create(ctx, secret); err != nil {
			return err
		}
	}

	return nil
}

// DeleteAppCredential deletes the secret which holds the application credential information.
func (s *storage) DeleteAppCredential(ctx context.Context) error {
	secret, err := s.getAppCredentialSecret(ctx)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
	}

	patch := client.MergeFrom(secret.DeepCopy())
	secret.ObjectMeta.Finalizers = []string{}
	if err := s.client.Patch(ctx, secret, patch); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if err := s.client.Delete(ctx, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return nil
}

// UpdateParentSecret update the stored secret of the parent Openstack user.
func (s *storage) UpdateParentSecret(ctx context.Context, parent *Parent) error {
	secret, err := s.getAppCredentialSecret(ctx)
	if err != nil {
		return err
	}

	patch := client.MergeFrom(secret.DeepCopy())
	secret.Data[applicationCredentialSecretParentSecret] = []byte(parent.credentials.Password)

	return s.client.Patch(ctx, secret, patch)
}

func (s *storage) getAppCredentialSecret(ctx context.Context) (*corev1.Secret, error) {
	var secret = &corev1.Secret{}
	if err := s.client.Get(ctx, kutil.Key(s.namespace, ApplicationCredentialSecretName), secret); err != nil {
		return nil, err
	}
	return secret, nil
}

func readSecretKey(secret *corev1.Secret, key string) string {
	return string(secret.Data[key])
}
