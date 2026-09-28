// Copyright (c) 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0

package operatorutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateYAMLSubstitutionValue(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		fieldName string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid simple value",
			value:     "my-csm-instance",
			fieldName: "CR.Name",
			wantErr:   false,
		},
		{
			name:      "valid value with dots",
			value:     "csm.dell.com",
			fieldName: "CR.Name",
			wantErr:   false,
		},
		{
			name:      "reject YAML document separator",
			value:     "myname\n---\nkind: ClusterRole",
			fieldName: "CR.Name",
			wantErr:   true,
			errMsg:    "", // Accept either "newline" or "YAML document separator" since both are present
		},
		{
			name:      "reject newline character",
			value:     "myname\nmalicious",
			fieldName: "CR.Name",
			wantErr:   true,
			errMsg:    "newline",
		},
		{
			name:      "reject carriage return",
			value:     "myname\rmalicious",
			fieldName: "CR.Name",
			wantErr:   true,
			errMsg:    "carriage return",
		},
		{
			name:      "reject YAML directive end",
			value:     "myname\n...\n---",
			fieldName: "CR.Name",
			wantErr:   true,
			errMsg:    "", // Accept any error since multiple forbidden characters are present
		},
		{
			name:      "reject null byte",
			value:     "myname\x00malicious",
			fieldName: "CR.Name",
			wantErr:   true,
			errMsg:    "null byte",
		},
		{
			name:      "reject control character (tab)",
			value:     "myname\tmalicious",
			fieldName: "CR.Name",
			wantErr:   true,
			errMsg:    "control character",
		},
		{
			name:      "reject empty value",
			value:     "",
			fieldName: "CR.Name",
			wantErr:   true,
			errMsg:    "cannot be empty",
		},
		{
			name:      "valid address with colon",
			value:     "otel-collector:55680",
			fieldName: "CollectorAddress",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateYAMLSubstitutionValue(tt.value, tt.fieldName)
			if tt.wantErr {
				assert.Error(t, err, "expected error for value: %q", tt.value)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg, "error message should mention the specific issue")
				}
			} else {
				assert.NoError(t, err, "expected no error for value: %q", tt.value)
			}
		})
	}
}

func TestValidateKubernetesName(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid simple name",
			value:   "my-csm",
			wantErr: false,
		},
		{
			name:    "valid name with dots",
			value:   "csm.dell.com",
			wantErr: false,
		},
		{
			name:    "valid name starting with number",
			value:   "1csm-test",
			wantErr: false,
		},
		{
			name:    "reject uppercase",
			value:   "MyCSM",
			wantErr: true,
			errMsg:  "DNS-1123 subdomain",
		},
		{
			name:    "reject starting with hyphen",
			value:   "-csm",
			wantErr: true,
			errMsg:  "DNS-1123 subdomain",
		},
		{
			name:    "reject ending with hyphen",
			value:   "csm-",
			wantErr: true,
			errMsg:  "DNS-1123 subdomain",
		},
		{
			name:    "reject starting with dot",
			value:   ".csm",
			wantErr: true,
			errMsg:  "DNS-1123 subdomain",
		},
		{
			name:    "reject special characters",
			value:   "csm_test",
			wantErr: true,
			errMsg:  "DNS-1123 subdomain",
		},
		{
			name:    "reject empty",
			value:   "",
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name:    "reject too long (>253)",
			value:   string(make([]byte, 254)),
			wantErr: true,
			errMsg:  "maximum length",
		},
		{
			name:    "max valid length (253)",
			value:   "a" + string(make([]byte, 251)) + "z",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For max length test, fill with valid chars
			testVal := tt.value
			if len(testVal) > 253 {
				testVal = "a"
				for j := 0; j < 252; j++ {
					testVal += "b"
				}
				testVal += "z"
			} else if len(testVal) == 253 {
				testVal = "a"
				for j := 0; j < 251; j++ {
					testVal += "b"
				}
				testVal += "z"
			}

			err := ValidateKubernetesName(testVal)
			if tt.wantErr {
				assert.Error(t, err, "expected error for name: %q", testVal)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg, "error message should mention the specific issue")
				}
			} else {
				assert.NoError(t, err, "expected no error for name: %q", testVal)
			}
		})
	}
}

func TestValidateKubernetesNamespace(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid simple namespace",
			value:   "csm-isilon",
			wantErr: false,
		},
		{
			name:    "valid namespace with numbers",
			value:   "ns1",
			wantErr: false,
		},
		{
			name:    "reject dots (not allowed in namespaces)",
			value:   "csm.isilon",
			wantErr: true,
			errMsg:  "DNS-1123 label",
		},
		{
			name:    "reject uppercase",
			value:   "CSM-Isilon",
			wantErr: true,
			errMsg:  "DNS-1123 label",
		},
		{
			name:    "reject starting with hyphen",
			value:   "-namespace",
			wantErr: true,
			errMsg:  "DNS-1123 label",
		},
		{
			name:    "reject ending with hyphen",
			value:   "namespace-",
			wantErr: true,
			errMsg:  "DNS-1123 label",
		},
		{
			name:    "reject empty",
			value:   "",
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name:    "reject too long (>63)",
			value:   string(make([]byte, 64)),
			wantErr: true,
			errMsg:  "maximum length",
		},
		{
			name:    "max valid length (63)",
			value:   string(make([]byte, 63)),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For length tests, fill with valid chars
			testVal := tt.value
			if len(testVal) >= 63 {
				testVal = "a"
				for j := 0; j < len(tt.value)-2; j++ {
					testVal += "b"
				}
				testVal += "z"
			}

			err := ValidateKubernetesNamespace(testVal)
			if tt.wantErr {
				assert.Error(t, err, "expected error for namespace: %q", testVal)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg, "error message should mention the specific issue")
				}
			} else {
				assert.NoError(t, err, "expected no error for namespace: %q", testVal)
			}
		})
	}
}

func TestValidateImageString(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		fieldName string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid image with tag",
			value:     "dellemc/csm-topology:v1.0.0",
			fieldName: "TopologyImage",
			wantErr:   false,
		},
		{
			name:      "valid image with digest",
			value:     "dellemc/csm-topology@sha256:abc123",
			fieldName: "TopologyImage",
			wantErr:   false,
		},
		{
			name:      "valid image with registry",
			value:     "registry.k8s.io/kube-proxy:v1.28.0",
			fieldName: "Image",
			wantErr:   false,
		},
		{
			name:      "reject image with newline",
			value:     "dellemc/image\nmalicious",
			fieldName: "Image",
			wantErr:   true,
			errMsg:    "newline",
		},
		{
			name:      "reject image with YAML separator",
			value:     "dellemc/image:v1\n---\nkind: ClusterRole",
			fieldName: "Image",
			wantErr:   true,
			errMsg:    "", // Accept either "newline" or "YAML document separator" since both are present
		},
		{
			name:      "reject empty image",
			value:     "",
			fieldName: "Image",
			wantErr:   true,
			errMsg:    "cannot be empty",
		},
		{
			name:      "reject image with spaces",
			value:     "dellemc/image with spaces",
			fieldName: "Image",
			wantErr:   true,
			errMsg:    "invalid characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateImageString(tt.value, tt.fieldName)
			if tt.wantErr {
				assert.Error(t, err, "expected error for image: %q", tt.value)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg, "error message should mention the specific issue")
				}
			} else {
				assert.NoError(t, err, "expected no error for image: %q", tt.value)
			}
		})
	}
}

// Test the specific VRT-46245 exploit pattern
func TestValidateYAMLSubstitutionValue_VRT46245Exploit(t *testing.T) {
	// Simulated attack payload from VRT-46245
	exploitPayload := `attacker-ns
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: csm-inject-pwn
rules:
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: csm-inject-pwn
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: csm-inject-pwn
subjects:
- kind: ServiceAccount
  name: attacker-sa
  namespace: attacker-ns`

	err := ValidateYAMLSubstitutionValue(exploitPayload, "CR.Namespace")
	assert.Error(t, err, "exploit payload should be rejected")
	assert.Contains(t, err.Error(), "forbidden", "should reject the exploit payload")
}

// TestValidateImageString_BuildMetadata tests that + character is allowed in image tags
func TestValidateImageString_BuildMetadata(t *testing.T) {
	tests := []struct {
		name    string
		image   string
		wantErr bool
	}{
		{
			name:    "semantic versioning with build metadata",
			image:   "registry.io/myimage:v1.0.0+build.123",
			wantErr: false,
		},
		{
			name:    "alpine with sha",
			image:   "alpine:3.18+sha256",
			wantErr: false,
		},
		{
			name:    "complex tag with plus",
			image:   "dellemc/csm-operator:v1.13.0+feature-branch",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateImageString(tt.image, "TestImage")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
