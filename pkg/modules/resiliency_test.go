// Copyright © 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package modules

import (
	"context"
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1"
	drivers "github.com/dell/csm-operator/pkg/drivers"
	utils "github.com/dell/csm-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
)

func TestResiliencyInjectClusterRole(t *testing.T) {
	ctx := context.Background()

	tests := map[string]func(t *testing.T) (bool, rbacv1.ClusterRole, utils.OperatorConfig, csmv1.ContainerStorageModule){
		"success - greenfield injection": func(*testing.T) (bool, rbacv1.ClusterRole, utils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerStore)
			if err != nil {
				panic(err)
			}
			return true, controllerYAML.Rbac.ClusterRole, operatorConfig, customResource
		},
		"fail - bad config path": func(*testing.T) (bool, rbacv1.ClusterRole, utils.OperatorConfig, csmv1.ContainerStorageModule) {
			tmpOperatorConfig := operatorConfig
			tmpOperatorConfig.ConfigDirectory = "bad/path"

			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerStore)
			if err != nil {
				panic(err)
			}
			return false, controllerYAML.Rbac.ClusterRole, tmpOperatorConfig, customResource
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, clusterRole, opConfig, cr := tc(t)
			newClusterRole, err := ResiliencyInjectClusterRole(clusterRole, cr, opConfig, "node")
			if success {
				assert.NoError(t, err)
				assert.NoError(t, CheckClusterRoleReplica(newClusterRole.Rules))
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestResiliencyPreCheck(t *testing.T) {

}
