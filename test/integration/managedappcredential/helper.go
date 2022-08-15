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
	"fmt"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"

	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/utils"
	"github.com/gophercloud/gophercloud"
	gophercloudopenstack "github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/utils/openstack/clientconfig"
)

const (
	namespacePrefix = "openstack--managedappcredential-it"
	shootNamePrefix = "shoot--it"
)

func generateRandomName(prefix string) string {
	suffix, err := utils.GenerateRandomStringFromCharset(5, "0123456789abcdefghijklmnopqrstuvwxyz")
	Expect(err).NotTo(HaveOccurred())
	return fmt.Sprintf("%s--%s", prefix, suffix)
}

func newOpenstackClient(credentials *openstack.Credentials) *gophercloud.ServiceClient {
	var clientOptions = &clientconfig.ClientOpts{
		AuthInfo: &clientconfig.AuthInfo{
			AuthURL:     credentials.AuthURL,
			DomainName:  credentials.DomainName,
			ProjectName: credentials.TenantName,
			Username:    credentials.Username,
			Password:    credentials.Password,
		},
	}

	authOptions, err := clientconfig.AuthOptions(clientOptions)
	Expect(err).NotTo(HaveOccurred())
	authOptions.AllowReauth = true

	baseClient, err := gophercloudopenstack.AuthenticatedClient(*authOptions)
	Expect(err).NotTo(HaveOccurred())

	identityClient, err := gophercloudopenstack.NewIdentityV3(baseClient, gophercloud.EndpointOpts{})
	Expect(err).NotTo(HaveOccurred())

	return identityClient
}
