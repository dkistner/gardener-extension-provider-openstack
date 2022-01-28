// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package v1alpha1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// SetDefaults_ControllerConfiguration sets defaults for the ControllerConfiguration.
func SetDefaults_ControllerConfiguration(obj *ControllerConfiguration) {
	if obj.ApplicationCrendentialConfig == nil {
		obj.ApplicationCrendentialConfig = &ApplicationCrendentialConfig{}
	}
}

// SetDefaults_ApplicationCrendentialConfig sets defaults for the ApplicationCrendentialConfig.
func SetDefaults_ApplicationCrendentialConfig(obj *ApplicationCrendentialConfig) {
	if obj.Lifetime == nil {
		obj.Lifetime = &metav1.Duration{
			Duration: time.Hour * 24,
		}
	}

	if obj.ExpirationPeriod == nil {
		obj.ExpirationPeriod = &metav1.Duration{
			Duration: time.Hour * 48,
		}
	}
}
