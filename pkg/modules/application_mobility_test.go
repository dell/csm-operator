// Copyright (c) 2022 Dell Inc., or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0

package modules

import (
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1"
	utils "github.com/dell/csm-operator/pkg/utils"
	"github.com/stretchr/testify/assert"

)

func TestGetAppMobilityModuleDeployment(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, csmv1.ContainerStorageModule, utils.OperatorConfig){
		"success": func(*testing.T) (bool, csmv1.ContainerStorageModule, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/csm_application_mobility_v020.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			return true, tmpCR, operatorConfig
		},
		"fail - app mobility module not found": func(*testing.T) (bool, csmv1.ContainerStorageModule, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_auth_proxy.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			return false, tmpCR, operatorConfig
		},
		//"fail - app mob config file not found": func(*testing.T) (bool, csmv1.ContainerStorageModule, utils.OperatorConfig) {
		//	customResource, err := getCustomResource("./testdata/nonexist.yaml")
		//	if err != nil {
		//		panic(err)
		//	}

		//	tmpCR := customResource

		//	sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

		//	return false, tmpCR, operatorConfig
		//},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			success, cr, op := tc(t)

			_, err := getAppMobilityModuleDeployment(op, cr, csmv1.Module{})
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}

		})
	}
}
