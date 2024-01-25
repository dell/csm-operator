//  Copyright © 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package modules

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/drivers"
	"github.com/dell/csm-operator/pkg/logger"
	"github.com/dell/csm-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Constants to be used in reverse proxy config files
const (
	ReverseProxyServerComponent = "csipowermax-reverseproxy"
	ReverseProxyDeployement     = "controller.yaml"
	ReverseProxyImage           = "<REVERSEPROXY_PROXY_SERVER_IMAGE>"
	ReverseProxyTLSSecret       = "<X_CSI_REVPROXY_TLS_SECRET>" // #nosec G101
	ReverseProxyConfigMap       = "<X_CSI_CONFIG_MAP_NAME>"
	ReverseProxyPort            = "<X_CSI_REVPROXY_PORT>"
)

var (
	proxyPort      string
	proxyTLSSecret string
	proxyConfig    string
)

// ReverseproxySupportedDrivers is a map containing the CSI Drivers supported by CSM Reverseproxy. The key is driver name and the value is the driver plugin identifier
var ReverseproxySupportedDrivers = map[string]SupportedDriverParam{
	string(csmv1.PowerMax): {
		PluginIdentifier:              drivers.PowerMaxPluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerMaxConfigParamsVolumeMount,
	},
}

// ReverseProxyPrecheck  - runs precheck for CSM ReverseProxy
func ReverseProxyPrecheck(ctx context.Context, op utils.OperatorConfig, revproxy csmv1.Module, cr csmv1.ContainerStorageModule, r utils.ReconcileCSM) error {
	log := logger.GetLogger(ctx)

	if _, ok := ReverseproxySupportedDrivers[string(cr.Spec.Driver.CSIDriverType)]; !ok {
		return fmt.Errorf("CSM Reverseproxy does not support %s driver", string(cr.Spec.Driver.CSIDriverType))
	}

	// check if provided version is supported
	if revproxy.ConfigVersion != "" {
		err := checkVersion(string(csmv1.ReverseProxy), revproxy.ConfigVersion, op.ConfigDirectory)
		if err != nil {
			return err
		}
	}
	// Check for secrets
	proxyServerSecret := "csirevproxy-tls-secret" // #nosec G101
	proxyConfigMap := "powermax-reverseproxy-config"
	if len(revproxy.Components) < 1 {
		return fmt.Errorf("revproxy components can not be nil")
	}
	for _, env := range revproxy.Components[0].Envs {
		if env.Name == "X_CSI_REVPROXY_TLS_SECRET" {
			proxyServerSecret = env.Value
		}
		if env.Name == "X_CSI_CONFIG_MAP_NAME" {
			proxyConfigMap = env.Value
		}
	}

	err := r.GetClient().Get(ctx, types.NamespacedName{Name: proxyServerSecret, Namespace: cr.GetNamespace()}, &corev1.Secret{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to find secret %s", proxyServerSecret)
		}
	}

	err = r.GetClient().Get(ctx, types.NamespacedName{Name: proxyConfigMap, Namespace: cr.GetNamespace()}, &corev1.ConfigMap{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to find configmap %s", proxyConfigMap)
		}
	}
	log.Infof("\nperformed pre checks for: %s", revproxy.Name)
	return nil
}

// ReverseProxyServer - apply/delete deployment objects
func ReverseProxyServer(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	log := logger.GetLogger(ctx)
	YamlString, err := getReverseProxyDeployment(op, cr, csmv1.Module{})
	if err != nil {
		return err
	}
	deployObjects, err := utils.GetModuleComponentObj([]byte(YamlString))
	if err != nil {
		return err
	}

	for _, ctrlObj := range deployObjects {
		log.Infof("Object: %v -----\n", ctrlObj)
		if isDeleting {
			if err := utils.DeleteObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		} else {
			if err := utils.ApplyObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		}
	}

	return nil
}

func getReverseProxyModule(cr csmv1.ContainerStorageModule) (csmv1.Module, error) {
	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.ReverseProxy {
			return m, nil
		}
	}
	return csmv1.Module{}, fmt.Errorf("reverseproxy module not found")
}

// getReverseProxyDeployment - updates deployment manifest with reverseproxy CRD values
func getReverseProxyDeployment(op utils.OperatorConfig, cr csmv1.ContainerStorageModule, revProxy csmv1.Module) (string, error) {
	YamlString := ""
	revProxy, err := getReverseProxyModule(cr)
	if err != nil {
		return YamlString, err
	}

	deploymentPath := fmt.Sprintf("%s/moduleconfig/%s/%s/%s", op.ConfigDirectory, csmv1.ReverseProxy, revProxy.ConfigVersion, ReverseProxyDeployement)
	buf, err := os.ReadFile(filepath.Clean(deploymentPath))
	if err != nil {
		return YamlString, err
	}

	YamlString = string(buf)
	proxyNamespace := cr.Namespace

	for _, component := range revProxy.Components {
		if component.Name == ReverseProxyServerComponent {
			YamlString = strings.ReplaceAll(YamlString, ReverseProxyImage, string(component.Image))
			for _, env := range component.Envs {
				if env.Name == "X_CSI_REVPROXY_TLS_SECRET" {
					proxyTLSSecret = env.Value
				}
				if env.Name == "X_CSI_REVPROXY_PORT" {
					proxyPort = env.Value
				}
				if env.Name == "X_CSI_CONFIG_MAP_NAME" {
					proxyConfig = env.Value
				}
			}
		}
	}

	YamlString = strings.ReplaceAll(YamlString, utils.DefaultReleaseNamespace, proxyNamespace)
	YamlString = strings.ReplaceAll(YamlString, ReverseProxyPort, proxyPort)
	YamlString = strings.ReplaceAll(YamlString, ReverseProxyTLSSecret, proxyTLSSecret)
	YamlString = strings.ReplaceAll(YamlString, ReverseProxyConfigMap, proxyConfig)
	return YamlString, nil
}
