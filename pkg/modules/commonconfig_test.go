// Copyright (c) 2022 Dell Inc., or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0

package modules

import (
	"os"
	"testing"

	"fmt"
	"log"

	csmv1 "github.com/dell/csm-operator/api/v1"
	utils "github.com/dell/csm-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

var (
	operatorConfig    utils.OperatorConfig
	badOperatorConfig utils.OperatorConfig
)

func TestMain(m *testing.M) {
	status := 0

	operatorConfig = utils.OperatorConfig{}
	operatorConfig.ConfigDirectory = "../../operatorconfig"

	if st := m.Run(); st > status {
		status = st
	}

	os.Exit(status)
}

func getCustomResource(path string) (csmv1.ContainerStorageModule, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("failed to read testdata: %v", err)

	}
	customResource := csmv1.ContainerStorageModule{}
	err = yaml.Unmarshal(b, &customResource)
	if err != nil {
		return customResource, fmt.Errorf("failed to read unmarshal CSM Custom resource: %v", err)
	}

	return customResource, nil

}

func getSecret(namespace, secretName string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"data": []byte(secretName),
		},
	}
}
