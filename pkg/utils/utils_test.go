//  Copyright © 2024 Dell Inc. or its subsidiaries. All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//       http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package utils

import (
	"context"
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/stretchr/testify/assert"
)

// TestHasModuleComponent tests HasModuleComponent directly, assuming IsModuleEnabled behaves as expected
func TestHasModuleComponent(t *testing.T) {
	tests := []struct {
		name           string
		instance       csmv1.ContainerStorageModule
		mod            csmv1.ModuleType
		componentType  string
		expectedResult bool
	}{
		{
			name: "Module disabled",
			instance: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Modules: []csmv1.Module{
						{
							Name:    csmv1.Observability,
							Enabled: false,
						},
					},
				},
			},
			mod:            csmv1.Observability,
			componentType:  "metrics-powerflex",
			expectedResult: false,
		},
		{
			name: "Module enabled, component exists",
			instance: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Modules: []csmv1.Module{
						{
							Name:    csmv1.Observability,
							Enabled: true,
							Components: []csmv1.ContainerTemplate{
								{Name: "metrics-powerflex"},
								{Name: "topology"},
							},
						},
					},
				},
			},
			mod:            csmv1.Observability,
			componentType:  "metrics-powerflex",
			expectedResult: true,
		},
		{
			name: "Module enabled, component does not exist",
			instance: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Modules: []csmv1.Module{
						{
							Name:    csmv1.Observability,
							Enabled: true,
							Components: []csmv1.ContainerTemplate{
								{Name: "topology"},
							},
						},
					},
				},
			},
			mod:            csmv1.Observability,
			componentType:  "metrics-powerflex",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			result := HasModuleComponent(ctx, tt.instance, tt.mod, tt.componentType)

			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestAddModuleComponent(t *testing.T) {
	type args struct {
		instance  *csmv1.ContainerStorageModule
		mod       csmv1.ModuleType
		component csmv1.ContainerTemplate
	}
	tests := []struct {
		name string
		args args
		want *csmv1.ContainerStorageModule
	}{
		{
			name: "Add component to a module",
			args: args{
				instance: &csmv1.ContainerStorageModule{
					Spec: csmv1.ContainerStorageModuleSpec{
						Modules: []csmv1.Module{
							{
								Name:    csmv1.Observability,
								Enabled: true,
								Components: []csmv1.ContainerTemplate{
									{Name: "metrics-powerflex"},
								},
							},
						},
					},
				},
				mod:       csmv1.Observability,
				component: csmv1.ContainerTemplate{Name: "topology"},
			},
			want: &csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Modules: []csmv1.Module{
						{
							Name:    csmv1.Observability,
							Enabled: true,
							Components: []csmv1.ContainerTemplate{
								{Name: "metrics-powerflex"},
								{Name: "topology"},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AddModuleComponent(tt.args.instance, tt.args.mod, tt.args.component)
			assert.Equal(t, tt.want, tt.args.instance)
		})
	}
}
