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
			name: "Module does not exist",
			instance: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Modules: []csmv1.Module{
						{
							Name:    csmv1.Replication,
							Enabled: true,
						},
					},
				},
			},
			mod:            csmv1.Observability,
			componentType:  "metrics-powerflex",
			expectedResult: false,
		},
		{
			name: "Module exist and component does not exist",
			instance: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Modules: []csmv1.Module{
						{
							Name:    csmv1.Observability,
							Enabled: false,
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
		{
			name: "Module exist and component exists",
			instance: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Modules: []csmv1.Module{
						{
							Name:    csmv1.Observability,
							Enabled: false,
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
			name: "Module does not exist",
			args: args{
				instance: &csmv1.ContainerStorageModule{
					Spec: csmv1.ContainerStorageModuleSpec{
						Modules: []csmv1.Module{
							{
								Name:    csmv1.Replication,
								Enabled: false,
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
							Name:    csmv1.Replication,
							Enabled: false,
						},
					},
				},
			},
		},
		{
			name: "Module exists and component is empty",
			args: args{
				instance: &csmv1.ContainerStorageModule{
					Spec: csmv1.ContainerStorageModuleSpec{
						Modules: []csmv1.Module{
							{
								Name:    csmv1.Observability,
								Enabled: false,
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
							Enabled: false,
							Components: []csmv1.ContainerTemplate{
								{Name: "topology"}},
						},
					},
				},
			},
		},
		{
			name: "Module exists and component is not empty",
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

func TestLoadDefaultComponents(t *testing.T) {
	invalidOp := OperatorConfig{
		ConfigDirectory: "invalid/path",
	}
	validOp := OperatorConfig{
		ConfigDirectory: "../../operatorconfig",
	}
	enabled := new(bool)
	*enabled = true
	disabled := new(bool)
	*disabled = false

	// Helper function to create ContainerStorageModule object
	createCR := func(driverType csmv1.DriverType, moduleType csmv1.ModuleType, moduleEnabled bool, components []csmv1.ContainerTemplate) *csmv1.ContainerStorageModule {
		return &csmv1.ContainerStorageModule{
			Spec: csmv1.ContainerStorageModuleSpec{
				Driver:  csmv1.Driver{CSIDriverType: driverType},
				Modules: []csmv1.Module{{Name: moduleType, Enabled: moduleEnabled, Components: components}},
			},
		}
	}

	type args struct {
		ctx context.Context
		cr  *csmv1.ContainerStorageModule
		op  OperatorConfig
	}
	tests := []struct {
		name    string
		args    args
		want    *csmv1.ContainerStorageModule
		wantErr bool
	}{
		{
			name: "Observability module does not exist",
			args: args{
				ctx: context.Background(),
				cr:  createCR(csmv1.PowerFlex, csmv1.Replication, true, nil),
				op:  validOp,
			},
			want:    createCR(csmv1.PowerFlex, csmv1.Replication, true, nil),
			wantErr: false,
		},
		{
			name: "Default components not found",
			args: args{
				ctx: context.Background(),
				cr:  createCR(csmv1.PowerFlex, csmv1.Observability, true, nil),
				op:  invalidOp,
			},
			want:    createCR(csmv1.PowerFlex, csmv1.Observability, true, nil),
			wantErr: true,
		},
		{
			name: "Module disabled and components empty",
			args: args{
				ctx: context.Background(),
				cr:  createCR(csmv1.PowerScale, csmv1.Observability, false, nil),
				op:  validOp,
			},
			want: createCR(csmv1.PowerScale, csmv1.Observability, false, []csmv1.ContainerTemplate{
				{Name: "topology", Enabled: enabled},
				{Name: "otel-collector", Enabled: enabled},
				{Name: "cert-manager", Enabled: disabled},
				{Name: "metrics-powerscale", Enabled: enabled},
			}),
			wantErr: false,
		},
		{
			name: "Module disabled and topology component missing",
			args: args{
				ctx: context.Background(),
				cr: createCR(csmv1.PowerFlex, csmv1.Observability, false, []csmv1.ContainerTemplate{
					{Name: "otel-collector", Enabled: enabled},
					{Name: "metrics-powerflex", Enabled: enabled},
				}),
				op: validOp,
			},
			want: createCR(csmv1.PowerFlex, csmv1.Observability, false, []csmv1.ContainerTemplate{
				{Name: "otel-collector", Enabled: enabled},
				{Name: "metrics-powerflex", Enabled: enabled},
				{Name: "topology", Enabled: enabled},
				{Name: "cert-manager", Enabled: disabled},
			}),
			wantErr: false,
		},
		{
			name: "Module enabled and cert-manager component missing",
			args: args{
				ctx: context.Background(),
				cr: createCR(csmv1.PowerFlex, csmv1.Observability, true, []csmv1.ContainerTemplate{
					{Name: "topology", Enabled: enabled},
					{Name: "otel-collector", Enabled: enabled},
					{Name: "metrics-powerflex", Enabled: enabled},
				}),
				op: validOp,
			},
			want: createCR(csmv1.PowerFlex, csmv1.Observability, true, []csmv1.ContainerTemplate{
				{Name: "topology", Enabled: enabled},
				{Name: "otel-collector", Enabled: enabled},
				{Name: "metrics-powerflex", Enabled: enabled},
				{Name: "cert-manager", Enabled: disabled},
			}),
			wantErr: false,
		},
		{
			name: "Module disabled and all components exist",
			args: args{
				ctx: context.Background(),
				cr: createCR(csmv1.PowerFlex, csmv1.Observability, false, []csmv1.ContainerTemplate{
					{Name: "topology", Enabled: enabled},
					{Name: "otel-collector", Enabled: enabled},
					{Name: "cert-manager", Enabled: disabled},
					{Name: "metrics-powerflex", Enabled: enabled},
				}),
				op: validOp,
			},
			want: createCR(csmv1.PowerFlex, csmv1.Observability, false, []csmv1.ContainerTemplate{
				{Name: "topology", Enabled: enabled},
				{Name: "otel-collector", Enabled: enabled},
				{Name: "cert-manager", Enabled: disabled},
				{Name: "metrics-powerflex", Enabled: enabled},
			}),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := LoadDefaultComponents(tt.args.ctx, tt.args.cr, tt.args.op)

			if tt.wantErr {
				assert.Error(t, err)
			}
			assert.Equal(t, tt.want, tt.args.cr)
		})
	}
}
