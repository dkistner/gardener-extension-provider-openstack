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
	"flag"
	"fmt"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal/managedappcredential"

	. "github.com/onsi/ginkgo/v2"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	// . "github.com/onsi/gomega"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func validateRotationUserFlags() bool {
	countRotationFlags := 0

	if len(*passwordRotation) == 0 {
		countRotationFlags++
	}
	if len(*usernnameRotation) == 0 {
		countRotationFlags++
	}

	if countRotationFlags == 2 {
		return true
	}

	return false
}

func validateMandatoryFlags() {
	panicNotSet(authURL, "auth-url")
	panicNotSet(domainName, "domainName")
	panicNotSet(tenantName, "tenant-name")
	panicNotSet(region, "region")
	panicNotSet(userName, "user-name")
	panicNotSet(password, "password")
}

func panicNotSet(flagVal *string, flagName string) {
	if flagVal == nil || len(*flagVal) == 0 {
		panic(fmt.Sprintf("--%s is not specified", flagName))
	}
}

var (
	authURL    = flag.String("auth-url", "", "Authorization URL for openstack")
	domainName = flag.String("domain-name", "", "Domain name for openstack")
	password   = flag.String("password", "", "Password of openstack user")
	region     = flag.String("region", "", "Openstack Region to use")
	tenantName = flag.String("tenant-name", "", "Name of openstack tenant to use")
	userName   = flag.String("user-name", "", "Username of openstack user")

	passwordRotation  = flag.String("alt-password", "", "Password of openstack user for rotation")
	usernnameRotation = flag.String("alt-user-name", "", "Username of openstack user for rotation")

	testUserRotation bool

	manager managedappcredential.Manager
)

var _ = BeforeSuite(func() {
	flag.Parse()
	validateMandatoryFlags()

	testUserRotation = validateRotationUserFlags()

	// enable manager logs
	logf.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter)))
	log := logrus.New()
	log.SetOutput(GinkgoWriter)
	logger := logrus.NewEntry(log)
	_ = logger

	// TODO: Clean up.

	manager managedappcredential.NewManager()
})

var _ = Describe("Managed Application Credential tests", func() {
	var ()

	BeforeEach(func() {

	})

	Context("parent user changed", func() {
		It("aa", func() {
			if !testUserRotation {
				Skip("skip as no alternative user is provided")
			}

			// TODO()
		})
	})

})
