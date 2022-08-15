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
)

const (
	authURLKey          = "auth-url"
	domainNameKey       = "domain-name"
	tenantNameKey       = "tenant-name"
	usernameKey         = "user-name"
	passwordKey         = "password"
	usernameRotationKey = "user-name-rotation"
	passwordRotationKey = "password-rotation"
)

var (
	mandatoryFlags = map[string]*string{
		authURLKey:    flag.String(authURLKey, "", "Authorization URL for openstack"),
		domainNameKey: flag.String(domainNameKey, "", "Domain name for openstack"),
		tenantNameKey: flag.String(tenantNameKey, "", "Name of openstack tenant to use"),
		usernameKey:   flag.String(usernameKey, "", "Username of openstack user"),
		passwordKey:   flag.String(passwordKey, "", "Password of openstack user"),
	}

	optionalFlags = map[string]*string{
		usernameRotationKey: flag.String(usernameRotationKey, "", "Username of openstack user for rotation"),
		passwordRotationKey: flag.String(passwordRotationKey, "", "Password of openstack user for rotation"),
	}
)

func validateMandatoryFlags() {
	for k, v := range mandatoryFlags {
		panicFlagNotSet(v, k)
	}
}

func validateRotationUserFlags() bool {
	if len(optionalFlags) != 2 {
		return false
	}

	for _, v := range optionalFlags {
		if v == nil || len(*v) == 0 {
			return false
		}
	}

	return true
}

func panicFlagNotSet(flagVal *string, flagName string) {
	if flagVal == nil || len(*flagVal) == 0 {
		panic(fmt.Sprintf("--%s is not specified", flagName))
	}
}
