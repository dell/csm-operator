//  Copyright © 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package operatorutils

import (
	"context"
	"os"
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeImageRef(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty",
			input:    "",
			expected: "",
		},
		{
			name:     "bare image with tag",
			input:    "redis:8.4.0-alpine",
			expected: "docker.io/library/redis:8.4.0-alpine",
		},
		{
			name:     "bare image without tag",
			input:    "redis",
			expected: "docker.io/library/redis",
		},
		{
			name:     "docker hub user image",
			input:    "openpolicyagent/opa:0.70.0",
			expected: "docker.io/openpolicyagent/opa:0.70.0",
		},
		{
			name:     "fully qualified quay",
			input:    "quay.io/dell/container-storage-modules/csi-powerstore:v2.18.0",
			expected: "quay.io/dell/container-storage-modules/csi-powerstore:v2.18.0",
		},
		{
			name:     "registry.k8s.io",
			input:    "registry.k8s.io/sig-storage/csi-attacher:v4.12.0",
			expected: "registry.k8s.io/sig-storage/csi-attacher:v4.12.0",
		},
		{
			name:     "with https scheme",
			input:    "https://my-harbor.example.com/csm/csi-powerstore:v2.18.0",
			expected: "my-harbor.example.com/csm/csi-powerstore:v2.18.0",
		},
		{
			name:     "with http scheme",
			input:    "http://my-harbor.example.com/csm/csi-powerstore:v2.18.0",
			expected: "my-harbor.example.com/csm/csi-powerstore:v2.18.0",
		},
		{
			name:     "with digest",
			input:    "quay.io/dell/csi-powerstore@sha256:abcdef1234567890",
			expected: "quay.io/dell/csi-powerstore@sha256:abcdef1234567890",
		},
		{
			name:     "mixed case",
			input:    "Quay.IO/Dell/CSI-Powerstore:V2.18.0",
			expected: "quay.io/dell/csi-powerstore:V2.18.0",
		},
		{
			name:     "localhost with port",
			input:    "localhost:5000/csm/csi-powerstore:v2.18.0",
			expected: "localhost:5000/csm/csi-powerstore:v2.18.0",
		},
		{
			name:     "docker.io explicit",
			input:    "docker.io/library/redis:8.4.0-alpine",
			expected: "docker.io/library/redis:8.4.0-alpine",
		},
		{
			name:     "attacker image",
			input:    "attacker/evil:tag",
			expected: "docker.io/attacker/evil:tag",
		},
		{
			name:     "attacker image with registry",
			input:    "evil-registry.com/attacker/evil:tag",
			expected: "evil-registry.com/attacker/evil:tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeImageRef(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsImageAllowed(t *testing.T) {
	allowlist := &ImageAllowlistConfig{
		AllowedRegistries: []string{
			"quay.io/dell",
			"registry.k8s.io/sig-storage",
			"docker.io/openpolicyagent",
			"docker.io/library/redis",
			"my-harbor.example.com/csm",
		},
	}

	tests := []struct {
		name    string
		image   string
		allowed bool
	}{
		{
			name:    "empty image allowed (template default)",
			image:   "",
			allowed: true,
		},
		{
			name:    "dell image allowed",
			image:   "quay.io/dell/container-storage-modules/csi-powerstore:v2.18.0",
			allowed: true,
		},
		{
			name:    "k8s sidecar allowed",
			image:   "registry.k8s.io/sig-storage/csi-attacher:v4.12.0",
			allowed: true,
		},
		{
			name:    "bare redis allowed via explicit docker.io/library/redis entry",
			image:   "redis:8.4.0-alpine",
			allowed: true,
		},
		{
			name:    "explicit redis with tag allowed",
			image:   "docker.io/library/redis:8.4.0-alpine",
			allowed: true,
		},
		{
			name:    "redis with digest allowed",
			image:   "docker.io/library/redis@sha256:0804c395e634e624243387d3c3a9c45fcaca876d313c2c8b52c3fdf9a912dded",
			allowed: true,
		},
		{
			name:    "openpolicyagent allowed",
			image:   "openpolicyagent/opa:0.70.0",
			allowed: true,
		},
		{
			name:    "private harbor allowed",
			image:   "my-harbor.example.com/csm/csi-powerstore:v2.18.0",
			allowed: true,
		},
		{
			name:    "attacker bare image denied",
			image:   "attacker/evil:tag",
			allowed: false,
		},
		{
			name:    "attacker registry denied",
			image:   "evil-registry.com/attacker/evil:tag",
			allowed: false,
		},
		{
			name:    "bare attacker image denied (security fix)",
			image:   "attacker-csm-powerstore:proof",
			allowed: false, // normalizes to docker.io/library/attacker-csm-powerstore:proof, NOT in allowlist
		},
		{
			name:    "bare nginx denied (not in explicit allowlist)",
			image:   "nginx:latest",
			allowed: false,
		},
		{
			name:    "different harbor path denied",
			image:   "my-harbor.example.com/other/csi-powerstore:v2.18.0",
			allowed: false,
		},
		{
			name:    "docker hub random user denied",
			image:   "randomuser/malicious:latest",
			allowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := allowlist.IsImageAllowed(tt.image)
			assert.Equal(t, tt.allowed, result, "image: %s", tt.image)
		})
	}
}

func TestIsRegistryAllowed(t *testing.T) {
	allowlist := &ImageAllowlistConfig{
		AllowedRegistries: []string{
			"quay.io/dell",
			"registry.k8s.io/sig-storage",
			"my-harbor.example.com/csm",
		},
	}

	tests := []struct {
		name     string
		registry string
		allowed  bool
	}{
		{
			name:     "exact match",
			registry: "quay.io/dell",
			allowed:  true,
		},
		{
			name:     "exact match with trailing slash",
			registry: "quay.io/dell/",
			allowed:  true,
		},
		{
			name:     "subpath match",
			registry: "quay.io/dell/container-storage-modules",
			allowed:  true,
		},
		{
			name:     "with https scheme",
			registry: "https://my-harbor.example.com/csm",
			allowed:  true,
		},
		{
			name:     "unrelated registry",
			registry: "evil-registry.com/attacker",
			allowed:  false,
		},
		{
			name:     "partial hostname mismatch",
			registry: "quay.io/dellx",
			allowed:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := allowlist.IsRegistryAllowed(tt.registry)
			assert.Equal(t, tt.allowed, result, "registry: %s", tt.registry)
		})
	}
}

func TestCollectUserSuppliedImages(t *testing.T) {
	trueVal := true
	cr := &csmv1.ContainerStorageModule{
		Spec: csmv1.ContainerStorageModuleSpec{
			Driver: csmv1.Driver{
				CSIDriverType: csmv1.PowerStore,
				ConfigVersion: "v2.18.0",
				Common: &csmv1.ContainerTemplate{
					Image: "attacker/evil:tag",
				},
				SideCars: []csmv1.ContainerTemplate{
					{Name: "provisioner", Image: "evil-registry.com/provisioner:latest"},
					{Name: "attacher", Image: ""},
				},
				InitContainers: []csmv1.ContainerTemplate{
					{Name: "init", Image: "evil-registry.com/init:latest"},
				},
			},
			Modules: []csmv1.Module{
				{
					Name:    csmv1.Authorization,
					Enabled: trueVal,
					Components: []csmv1.ContainerTemplate{
						{Name: "proxy", Image: "evil-registry.com/proxy:latest"},
					},
					InitContainer: []csmv1.ContainerTemplate{
						{Name: "init-auth", Image: "evil-registry.com/init-auth:latest"},
					},
				},
			},
		},
	}

	images := collectUserSuppliedImages(cr)

	// Should find 5 non-empty images (attacher has empty image):
	// common.image + provisioner sidecar + init + module component + module init-container
	assert.Equal(t, 5, len(images))
	assert.Contains(t, images, "spec.driver.common.image")
	assert.Equal(t, "attacker/evil:tag", images["spec.driver.common.image"])
}

func TestValidateImageOverrides_AllAllowed(t *testing.T) {
	allowlist := &ImageAllowlistConfig{
		AllowedRegistries: []string{
			"quay.io/dell",
			"registry.k8s.io/sig-storage",
		},
	}

	cr := &csmv1.ContainerStorageModule{
		Spec: csmv1.ContainerStorageModuleSpec{
			Driver: csmv1.Driver{
				CSIDriverType: csmv1.PowerStore,
				ConfigVersion: "v2.18.0",
				Common: &csmv1.ContainerTemplate{
					Image: "quay.io/dell/container-storage-modules/csi-powerstore:v2.18.0",
				},
				SideCars: []csmv1.ContainerTemplate{
					{Name: "provisioner", Image: "registry.k8s.io/sig-storage/csi-provisioner:v6.3.0"},
				},
			},
		},
	}

	ctx := context.Background()
	err := ValidateImageOverrides(ctx, cr, allowlist)
	assert.NoError(t, err)
}

func TestValidateImageOverrides_Denied(t *testing.T) {
	allowlist := &ImageAllowlistConfig{
		AllowedRegistries: []string{
			"quay.io/dell",
			"registry.k8s.io/sig-storage",
		},
	}

	cr := &csmv1.ContainerStorageModule{
		Spec: csmv1.ContainerStorageModuleSpec{
			Driver: csmv1.Driver{
				CSIDriverType: csmv1.PowerStore,
				ConfigVersion: "v2.18.0",
				Common: &csmv1.ContainerTemplate{
					Image: "attacker/evil:tag",
				},
			},
		},
	}

	ctx := context.Background()
	err := ValidateImageOverrides(ctx, cr, allowlist)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "image allowlist validation failed")
	assert.Contains(t, err.Error(), "attacker/evil:tag")
}

func TestValidateImageOverrides_CustomRegistryDenied(t *testing.T) {
	allowlist := &ImageAllowlistConfig{
		AllowedRegistries: []string{
			"quay.io/dell",
		},
	}

	cr := &csmv1.ContainerStorageModule{
		Spec: csmv1.ContainerStorageModuleSpec{
			Version:        "v1.18.0",
			CustomRegistry: "evil-registry.com/attacker",
			Driver: csmv1.Driver{
				CSIDriverType: csmv1.PowerStore,
			},
		},
	}

	ctx := context.Background()
	err := ValidateImageOverrides(ctx, cr, allowlist)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "spec.customRegistry")
	assert.Contains(t, err.Error(), "evil-registry.com/attacker")
}

func TestValidateImageOverrides_NoOverrides(t *testing.T) {
	allowlist := &ImageAllowlistConfig{
		AllowedRegistries: []string{
			"quay.io/dell",
		},
	}

	cr := &csmv1.ContainerStorageModule{
		Spec: csmv1.ContainerStorageModuleSpec{
			Driver: csmv1.Driver{
				CSIDriverType: csmv1.PowerStore,
				ConfigVersion: "v2.18.0",
			},
		},
	}

	ctx := context.Background()
	err := ValidateImageOverrides(ctx, cr, allowlist)
	assert.NoError(t, err)
}

func TestValidateImageOverrides_NilAllowlist(t *testing.T) {
	cr := &csmv1.ContainerStorageModule{
		Spec: csmv1.ContainerStorageModuleSpec{
			Driver: csmv1.Driver{
				CSIDriverType: csmv1.PowerStore,
				ConfigVersion: "v2.18.0",
			},
		},
	}

	ctx := context.Background()
	err := ValidateImageOverrides(ctx, cr, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "image allowlist is nil")
}

func TestValidateImageOverrides_MultipleViolations(t *testing.T) {
	allowlist := &ImageAllowlistConfig{
		AllowedRegistries: []string{
			"quay.io/dell",
		},
	}

	cr := &csmv1.ContainerStorageModule{
		Spec: csmv1.ContainerStorageModuleSpec{
			Driver: csmv1.Driver{
				CSIDriverType: csmv1.PowerStore,
				ConfigVersion: "v2.18.0",
				Common: &csmv1.ContainerTemplate{
					Image: "attacker/evil:tag",
				},
				SideCars: []csmv1.ContainerTemplate{
					{Name: "provisioner", Image: "evil-registry.com/provisioner:latest"},
				},
			},
		},
	}

	ctx := context.Background()
	err := ValidateImageOverrides(ctx, cr, allowlist)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "attacker/evil:tag")
	assert.Contains(t, err.Error(), "evil-registry.com/provisioner:latest")
}

func TestDefaultAllowedRegistries(t *testing.T) {
	// Verify that all default registries are reasonable
	assert.NotEmpty(t, defaultAllowedRegistries)
	for _, r := range defaultAllowedRegistries {
		assert.NotEmpty(t, r)
		assert.NotContains(t, r, "http://")
		assert.NotContains(t, r, "https://")
	}
	// Verify that the broad docker.io/library entry has been replaced with specific image
	assert.NotContains(t, defaultAllowedRegistries, "docker.io/library", "docker.io/library should not be in the allow-list (security hardening)")
	assert.Contains(t, defaultAllowedRegistries, "docker.io/library/redis", "docker.io/library/redis should be explicitly allowed")
}

func TestGetOperatorNamespace(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		setEnv   bool
		expected string
	}{
		{
			name:     "POD_NAMESPACE set returns custom namespace",
			envValue: "custom-operator-namespace",
			setEnv:   true,
			expected: "custom-operator-namespace",
		},
		{
			name:     "POD_NAMESPACE not set returns default",
			setEnv:   false,
			expected: DefaultOperatorNamespace,
		},
		{
			name:     "POD_NAMESPACE empty returns default",
			envValue: "",
			setEnv:   true,
			expected: DefaultOperatorNamespace,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original env var
			originalValue, wasSet := os.LookupEnv(OperatorNamespaceEnvVar)
			defer func() {
				// Restore original env var
				if wasSet {
					os.Setenv(OperatorNamespaceEnvVar, originalValue)
				} else {
					os.Unsetenv(OperatorNamespaceEnvVar)
				}
			}()

			// Set or unset env var for test
			if tt.setEnv {
				os.Setenv(OperatorNamespaceEnvVar, tt.envValue)
			} else {
				os.Unsetenv(OperatorNamespaceEnvVar)
			}

			result := GetOperatorNamespace()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateVersionSpecImages_AllAllowed(t *testing.T) {
	allowlist := &ImageAllowlistConfig{
		AllowedRegistries: []string{
			"quay.io/dell",
		},
	}

	matched := VersionSpec{
		Version: "v1.18.0",
		Images: map[string]string{
			"isilon": "quay.io/dell/container-storage-modules/csi-isilon:v1.18.0",
		},
	}

	ctx := context.Background()
	err := ValidateVersionSpecImages(ctx, matched, allowlist)
	assert.NoError(t, err)
}

func TestValidateVersionSpecImages_Denied(t *testing.T) {
	allowlist := &ImageAllowlistConfig{
		AllowedRegistries: []string{
			"quay.io/dell",
		},
	}

	matched := VersionSpec{
		Version: "v1.18.0",
		Images: map[string]string{
			"isilon": "evil-registry.com/csi-isilon:v1.18.0",
		},
	}

	ctx := context.Background()
	err := ValidateVersionSpecImages(ctx, matched, allowlist)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "evil-registry.com/csi-isilon:v1.18.0")
}
