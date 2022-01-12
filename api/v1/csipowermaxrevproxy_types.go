/*

Copyright (c) 2021 Dell Inc., or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package v1

import (
	corev1 "k8s.io/api/core/v1"
)

// ProxyLimits max limits
// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// ProxyLimits is used for storing the various types of limits
// applied for a particular proxy instance
type ProxyLimits struct {
	MaxActiveRead       int `json:"maxActiveRead,omitempty" yaml:"maxActiveRead,omitempty"`
	MaxActiveWrite      int `json:"maxActiveWrite,omitempty" yaml:"maxActiveWrite,omitempty"`
	MaxOutStandingRead  int `json:"maxOutStandingRead,omitempty" yaml:"maxOutStandingRead,omitempty"`
	MaxOutStandingWrite int `json:"maxOutStandingWrite,omitempty" yaml:"maxOutStandingWrite,omitempty"`
}

// ManagementServerConfig - represents a management server configuration for the management server
type ManagementServerConfig struct {
	URL                       string      `json:"url" yaml:"url"`
	ArrayCredentialSecret     string      `json:"arrayCredentialSecret,omitempty" yaml:"arrayCredentialSecret,omitempty"`
	SkipCertificateValidation bool        `json:"skipCertificateValidation,omitempty" yaml:"skipCertificateValidation,omitempty"`
	CertSecret                string      `json:"certSecret,omitempty" yaml:"certSecret,omitempty"`
	Limits                    ProxyLimits `json:"limits,omitempty" yaml:"limits,omitempty"`
}

// StorageArrayConfig represents a storage array managed by reverse proxy
type StorageArrayConfig struct {
	StorageArrayID         string   `json:"storageArrayId" yaml:"storageArrayId"`
	PrimaryURL             string   `json:"primaryURL" yaml:"primaryURL"`
	BackupURL              string   `json:"backupURL,omitempty" yaml:"backupURL,omitempty"`
	ProxyCredentialSecrets []string `json:"proxyCredentialSecrets" yaml:"proxyCredentialSecrets"`
}

// LinkConfig is one of the configuration modes for reverse proxy
type LinkConfig struct {
	Primary ManagementServerConfig `json:"primary" yaml:"primary"`
	Backup  ManagementServerConfig `json:"backup,omitempty" yaml:"backup,omitempty"`
}

// StandAloneConfig is one of the configuration modes for reverse proxy
type StandAloneConfig struct {
	StorageArrayConfig     []StorageArrayConfig     `json:"storageArrays" yaml:"storageArrays"`
	ManagementServerConfig []ManagementServerConfig `json:"managementServers" yaml:"managementServers"`
}

// RevProxyConfig represents the reverse proxy configuration
type RevProxyConfig struct {
	Mode             string            `json:"mode,omitempty" yaml:"mode,omitempty"`
	Port             int32             `json:"port,omitempty" yaml:"port,omitempty"`
	LinkConfig       *LinkConfig       `json:"linkConfig,omitempty" yaml:"linkConfig,omitempty"`
	StandAloneConfig *StandAloneConfig `json:"standAloneConfig,omitempty" yaml:"standAloneConfig,omitempty"`
}

// CSIPowerMaxRevProxySpec defines the desired state of CSIPowerMaxRevProxy
type CSIPowerMaxRevProxySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Image           string            `json:"image" yaml:"image"`
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty" yaml:"imagePullPolicy,omitempty"`
	TLSSecret       string            `json:"tlsSecret" yaml:"tlsSecret"`
	RevProxy        RevProxyConfig    `json:"config" yaml:"config"`
}
