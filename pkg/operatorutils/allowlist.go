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
	"fmt"
	"os"
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	// ImageAllowlistConfigMapName is the name of the ConfigMap that holds the
	// operator-managed image/registry allowlist.
	ImageAllowlistConfigMapName = "csm-image-allowlist"

	// ImageAllowlistKey is the data key inside the allowlist ConfigMap.
	ImageAllowlistKey = "allowlist.yaml"

	// DefaultOperatorNamespace is the default namespace for the CSM Operator.
	DefaultOperatorNamespace = "dell-csm-operator"

	// OperatorNamespaceEnvVar is the environment variable name for the operator namespace.
	OperatorNamespaceEnvVar = "POD_NAMESPACE"
)

// defaultAllowedRegistries is the built-in set of registries that are allowed
// when no csm-image-allowlist ConfigMap is deployed. It covers every registry
// used by the shipped operator templates (manager.yaml, sidecar-images.yaml,
// and module configs).
var defaultAllowedRegistries = []string{
	"quay.io/dell",
	"registry.k8s.io/sig-storage",
	"ghcr.io/open-telemetry",
	"quay.io/jetstack",
	"docker.io/openpolicyagent",
	"docker.io/rediscommander",
	"docker.io/library/redis",
	"quay.io/nginx",
	"gcr.io/k8s-staging-sig-storage",
	"quay.io/dell/storage",
	"quay.io/csiaddons",
	"registry.connect.redhat.com/dell-emc",
}

// ImageAllowlistConfig is the deserialized form of the allowlist.yaml key
// inside the csm-image-allowlist ConfigMap.
type ImageAllowlistConfig struct {
	// AllowedRegistries contains registry prefixes that are permitted.
	// An image is allowed if its normalized reference starts with any entry.
	// Examples: "quay.io/dell", "my-harbor.example.com/csm"
	AllowedRegistries []string `json:"allowedRegistries" yaml:"allowedRegistries"`
}

// GetOperatorNamespace returns the operator namespace from the POD_NAMESPACE
// environment variable, or DefaultOperatorNamespace if the variable is not set.
// This supports multi-tenant deployments where the operator may run in a
// namespace other than dell-csm-operator.
func GetOperatorNamespace() string {
	if ns := os.Getenv(OperatorNamespaceEnvVar); ns != "" {
		return ns
	}
	return DefaultOperatorNamespace
}

// LoadImageAllowlist reads the csm-image-allowlist ConfigMap from the operator
// namespace. If the ConfigMap does not exist, it returns a config populated
// with the built-in defaults. If the ConfigMap exists but is malformed, it
// returns an error.
func LoadImageAllowlist(ctx context.Context, ctrlClient crclient.Client, operatorNamespace string) (*ImageAllowlistConfig, error) {
	log := logger.GetLogger(ctx)

	if operatorNamespace == "" {
		operatorNamespace = DefaultOperatorNamespace
	}

	var cm corev1.ConfigMap
	key := types.NamespacedName{Name: ImageAllowlistConfigMapName, Namespace: operatorNamespace}
	if err := ctrlClient.Get(ctx, key, &cm); err != nil {
		// ConfigMap not found — use built-in defaults.
		log.Infow("csm-image-allowlist ConfigMap not found, using built-in defaults",
			"namespace", operatorNamespace)
		return &ImageAllowlistConfig{
			AllowedRegistries: defaultAllowedRegistries,
		}, nil
	}

	data, ok := cm.Data[ImageAllowlistKey]
	if !ok || strings.TrimSpace(data) == "" {
		log.Infow("csm-image-allowlist ConfigMap found but allowlist.yaml key is empty, using built-in defaults")
		return &ImageAllowlistConfig{
			AllowedRegistries: defaultAllowedRegistries,
		}, nil
	}

	var cfg ImageAllowlistConfig
	if err := yaml.Unmarshal([]byte(data), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s/%s: %w", ImageAllowlistConfigMapName, ImageAllowlistKey, err)
	}

	if len(cfg.AllowedRegistries) == 0 {
		log.Infow("csm-image-allowlist ConfigMap has empty allowedRegistries, using built-in defaults")
		return &ImageAllowlistConfig{
			AllowedRegistries: defaultAllowedRegistries,
		}, nil
	}

	log.Infow("Loaded image allowlist from ConfigMap",
		"namespace", operatorNamespace,
		"registryCount", len(cfg.AllowedRegistries))
	return &cfg, nil
}

// normalizeImageRef normalizes an image reference to a consistent form:
//   - Strips http:// or https:// schemes.
//   - Prepends "docker.io/library/" for bare images like "redis:tag".
//   - Prepends "docker.io/" for single-slash images like "openpolicyagent/opa:tag".
//   - Lowercases the registry/repo portion (before the tag/digest separator).
//
// This ensures "redis:8.4.0-alpine" matches an allowlist entry for
// "docker.io/library" and "openpolicyagent/opa:0.70.0" matches "docker.io/openpolicyagent".
func normalizeImageRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}

	// Strip scheme prefixes
	for _, prefix := range []string{"https://", "http://"} {
		ref = strings.TrimPrefix(ref, prefix)
	}

	// Separate tag/digest from the image path
	var suffix string
	if idx := strings.LastIndex(ref, "@"); idx != -1 {
		suffix = ref[idx:]
		ref = ref[:idx]
	} else if idx := strings.LastIndex(ref, ":"); idx != -1 {
		// Only treat as tag if there's no slash after the colon (port detection)
		afterColon := ref[idx+1:]
		if !strings.Contains(afterColon, "/") {
			suffix = ref[idx:]
			ref = ref[:idx]
		}
	}

	// Determine if this is a bare image or has a registry.
	// A reference contains a registry if the first path component contains a
	// dot or colon (e.g., "quay.io/dell/img" or "localhost:5000/img").
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) == 1 {
		// Bare image name, e.g. "redis" -> "docker.io/library/redis"
		ref = "docker.io/library/" + ref
	} else if !strings.Contains(parts[0], ".") && !strings.Contains(parts[0], ":") {
		// No dots or colons in first segment -> Docker Hub user image
		// e.g. "openpolicyagent/opa" -> "docker.io/openpolicyagent/opa"
		ref = "docker.io/" + ref
	}

	return strings.ToLower(ref) + suffix
}

// IsRegistryAllowed checks whether a registry prefix matches any entry in the
// allowlist. The registry is normalized before comparison.
func (c *ImageAllowlistConfig) IsRegistryAllowed(registry string) bool {
	normalized := strings.ToLower(strings.TrimSpace(registry))
	for _, prefix := range []string{"https://", "http://"} {
		normalized = strings.TrimPrefix(normalized, prefix)
	}
	normalized = strings.TrimRight(normalized, "/")

	for _, allowed := range c.AllowedRegistries {
		allowed = strings.ToLower(strings.TrimSpace(allowed))
		allowed = strings.TrimRight(allowed, "/")
		// Check for exact match or registry sub-path match
		if normalized == allowed || strings.HasPrefix(normalized, allowed+"/") {
			return true
		}
	}
	return false
}

// IsImageAllowed checks whether a fully-qualified image reference is from an
// allowed registry. The image is normalized before comparison.
//
// Supports both registry-level and image-level allow-list entries:
//   - Registry-level: "quay.io/dell" matches "quay.io/dell/csi-powerstore:v2.18.0"
//   - Image-level: "docker.io/library/redis" matches "docker.io/library/redis:8.4.0-alpine"
func (c *ImageAllowlistConfig) IsImageAllowed(image string) bool {
	normalized := normalizeImageRef(image)
	if normalized == "" {
		return true // empty image -> will use template default
	}

	for _, allowed := range c.AllowedRegistries {
		allowed = strings.ToLower(strings.TrimSpace(allowed))
		allowed = strings.TrimRight(allowed, "/")

		// Check for registry-level match (followed by path separator)
		if strings.HasPrefix(normalized, allowed+"/") {
			return true
		}

		// Check for image-level match (followed by tag or digest separator)
		if strings.HasPrefix(normalized, allowed+":") || strings.HasPrefix(normalized, allowed+"@") {
			return true
		}
	}
	return false
}

// collectUserSuppliedImages gathers all non-empty, user-supplied image
// references from the CR spec. These are the fields that could be used to
// inject arbitrary images when configVersion is set (the spec.version path
// already blocks these via CEL validation).
func collectUserSuppliedImages(cr *csmv1.ContainerStorageModule) map[string]string {
	images := make(map[string]string)

	// Driver common image
	if cr.Spec.Driver.Common != nil && cr.Spec.Driver.Common.Image != "" {
		images["spec.driver.common.image"] = string(cr.Spec.Driver.Common.Image)
	}

	// Driver sidecar images
	for i, sc := range cr.Spec.Driver.SideCars {
		if sc.Image != "" {
			key := fmt.Sprintf("spec.driver.sideCars[%d].image (%s)", i, sc.Name)
			images[key] = string(sc.Image)
		}
	}

	// Driver init-container images
	for i, ic := range cr.Spec.Driver.InitContainers {
		if ic.Image != "" {
			key := fmt.Sprintf("spec.driver.initContainers[%d].image (%s)", i, ic.Name)
			images[key] = string(ic.Image)
		}
	}

	// Module component and init-container images
	for mi, m := range cr.Spec.Modules {
		for ci, comp := range m.Components {
			if comp.Image != "" {
				key := fmt.Sprintf("spec.modules[%d].components[%d].image (%s/%s)", mi, ci, m.Name, comp.Name)
				images[key] = string(comp.Image)
			}
		}
		for ii, ic := range m.InitContainer {
			if ic.Image != "" {
				key := fmt.Sprintf("spec.modules[%d].initContainer[%d].image (%s/%s)", mi, ii, m.Name, ic.Name)
				images[key] = string(ic.Image)
			}
		}
	}

	return images
}

// ValidateImageOverrides checks all user-supplied image references in the CR
// against the allowlist. It returns an error describing every disallowed image
// found, or nil if all images are allowed (or no overrides are present).
//
// This function validates:
//   - spec.driver.common.image
//   - spec.driver.sideCars[*].image
//   - spec.driver.initContainers[*].image
//   - spec.modules[*].components[*].image
//   - spec.modules[*].initContainer[*].image
//   - spec.customRegistry (registry-level check)
func ValidateImageOverrides(ctx context.Context, cr *csmv1.ContainerStorageModule, allowlist *ImageAllowlistConfig) error {
	log := logger.GetLogger(ctx)

	if allowlist == nil {
		return fmt.Errorf("image allowlist is nil")
	}

	var violations []string

	// Check customRegistry (only valid with spec.version, but validate it
	// here as defense-in-depth).
	if cr.Spec.CustomRegistry != "" {
		if !allowlist.IsRegistryAllowed(cr.Spec.CustomRegistry) {
			violations = append(violations,
				fmt.Sprintf("spec.customRegistry %q is not in the allowed registries list", cr.Spec.CustomRegistry))
		}
	}

	// Check all explicit image overrides
	userImages := collectUserSuppliedImages(cr)
	for field, image := range userImages {
		if !allowlist.IsImageAllowed(image) {
			violations = append(violations,
				fmt.Sprintf("%s: image %q is not from an allowed registry", field, image))
		}
	}

	if len(violations) > 0 {
		log.Errorw("Image allowlist validation failed", "violations", violations)
		return fmt.Errorf("image allowlist validation failed: %s", strings.Join(violations, "; "))
	}

	log.Infow("Image allowlist validation passed",
		"checkedImages", len(userImages),
		"customRegistry", cr.Spec.CustomRegistry != "")
	return nil
}

// ValidateVersionSpecImages checks all images in a VersionSpec (typically resolved
// from the csm-images ConfigMap) against the allowlist. It is intended to be
// called during PreChecks when spec.version is set so that operator-managed
// image mappings cannot bypass the registry allowlist.
func ValidateVersionSpecImages(ctx context.Context, matched VersionSpec, allowlist *ImageAllowlistConfig) error {
	log := logger.GetLogger(ctx)

	if allowlist == nil {
		return fmt.Errorf("image allowlist is nil")
	}

	// If no version or images are present, there is nothing to validate. This
	// covers cases where the csm-images ConfigMap is missing, empty, or does not
	// contain an entry for the requested spec.version; in those scenarios the
	// operator falls back to template defaults which are already covered by the
	// built-in allowlist.
	if matched.Version == "" || len(matched.Images) == 0 {
		return nil
	}

	var violations []string
	for key, image := range matched.Images {
		if strings.TrimSpace(image) == "" {
			continue
		}
		if !allowlist.IsImageAllowed(image) {
			violations = append(violations,
				fmt.Sprintf("csm-images[%s]: image %q is not from an allowed registry", key, image))
		}
	}

	if len(violations) > 0 {
		log.Errorw("Image allowlist validation failed for csm-images",
			"version", matched.Version,
			"violations", violations)
		return fmt.Errorf("image allowlist validation failed for csm-images version %s: %s", matched.Version, strings.Join(violations, "; "))
	}

	log.Infow("Image allowlist validation passed for csm-images",
		"version", matched.Version,
		"checkedImages", len(matched.Images))
	return nil
}
