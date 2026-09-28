// Copyright (c) 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0

package operatorutils

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// ValidateYAMLSubstitutionValue validates that a value is safe for YAML template substitution.
// It rejects values containing YAML metacharacters that could enable document injection attacks.
//
// This function prevents YAML injection by rejecting:
// - YAML document separators (---)
// - YAML directive end markers (...)
// - Line breaks (\n, \r) that could start new YAML documents
// - Null bytes and other control characters
//
// Security context: VRT-46245 - Prevents privilege escalation via manifest template injection.
func ValidateYAMLSubstitutionValue(value, fieldName string) error {
	if value == "" {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}

	// Reject YAML metacharacters that enable document injection
	forbidden := map[string]string{
		"\n":   "newline",
		"\r":   "carriage return",
		"---":  "YAML document separator",
		"...":  "YAML directive end marker",
		"\x00": "null byte",
	}

	for char, desc := range forbidden {
		if strings.Contains(value, char) {
			return fmt.Errorf("%s contains forbidden %s character(s)", fieldName, desc)
		}
	}

	// Reject other control characters (0x00-0x1F except space)
	for _, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("%s contains forbidden control character (code: %d)", fieldName, r)
		}
	}

	return nil
}

// ValidateKubernetesName validates that a name follows Kubernetes DNS-1123 subdomain rules.
// Used for resource names like ContainerStorageModule CR names.
//
// Rules:
// - Lowercase letters, numbers, hyphens, and dots only
// - Max 253 characters
// - Cannot start or end with hyphen or dot
// - Must start and end with alphanumeric character
func ValidateKubernetesName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	if len(name) > 253 {
		return fmt.Errorf("name exceeds maximum length of 253 characters (got %d)", len(name))
	}

	// DNS-1123 subdomain regex: [a-z0-9]([-a-z0-9.]*[a-z0-9])?
	dns1123SubdomainRegex := regexp.MustCompile(`^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`)
	if !dns1123SubdomainRegex.MatchString(name) {
		return fmt.Errorf("name must be a valid DNS-1123 subdomain (lowercase alphanumeric, '-', '.' only)")
	}

	return nil
}

// ValidateKubernetesNamespace validates that a namespace name follows Kubernetes DNS-1123 label rules.
//
// Rules:
// - Lowercase letters, numbers, and hyphens only (no dots)
// - Max 63 characters
// - Cannot start or end with hyphen
// - Must start and end with alphanumeric character
func ValidateKubernetesNamespace(namespace string) error {
	if namespace == "" {
		return fmt.Errorf("namespace cannot be empty")
	}

	if len(namespace) > 63 {
		return fmt.Errorf("namespace exceeds maximum length of 63 characters (got %d)", len(namespace))
	}

	// DNS-1123 label regex: [a-z0-9]([-a-z0-9]*[a-z0-9])?
	dns1123LabelRegex := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	if !dns1123LabelRegex.MatchString(namespace) {
		return fmt.Errorf("namespace must be a valid DNS-1123 label (lowercase alphanumeric and '-' only)")
	}

	return nil
}

// ValidateImageString validates container image references for YAML substitution safety.
// Allows standard Docker image reference characters plus ':', '/', and '@' for digests.
func ValidateImageString(image, fieldName string) error {
	if image == "" {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}

	// First, apply general YAML safety validation
	if err := ValidateYAMLSubstitutionValue(image, fieldName); err != nil {
		return err
	}

	// Image format: [REGISTRY/]NAME[:TAG|@DIGEST]
	// Allow: alphanumeric, dots, hyphens, underscores, slashes, colons, @, + (for build metadata)
	// Example: registry.io/myimage:v1.0.0+build.123@sha256:abc...
	imageRegex := regexp.MustCompile(`^[a-zA-Z0-9._\-/:@+]+$`)
	if !imageRegex.MatchString(image) {
		return fmt.Errorf("%s contains invalid characters for container image reference", fieldName)
	}

	return nil
}
