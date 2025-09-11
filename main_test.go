//  Copyright © 2025 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/dell/csm-operator/controllers"
	"github.com/dell/csm-operator/k8s"
	"github.com/dell/csm-operator/pkg/logger"
	operatorutils "github.com/dell/csm-operator/pkg/operatorutils"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/meta"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	crzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/yaml"
)

func TestPrintVersion(_ *testing.T) {
	_, log := logger.GetNewContextWithLogger("main")
	printVersion(log)
}

func TestGetOperatorConfig(t *testing.T) {
	tests := []struct {
		name                            string
		isOpenShift                     func(_ *zap.SugaredLogger) (bool, error)
		getKubeAPIServerVersion         func() (*version.Info, error)
		getConfigDir                    func() string
		getK8sPathFn                    func(_ *zap.SugaredLogger, _ string, _, _, _ float64) string
		getK8sMinimumSupportedVersionFn func() string
		getK8sMaximumSupportedVersionFn func() string
		yamlUnmarshal                   func(data []byte, v interface{}, opts ...yaml.JSONOpt) error
		expectedConfig                  operatorutils.OperatorConfig
		wantErr                         bool
	}{
		{
			name:                    "Openshift environment",
			isOpenShift:             func(_ *zap.SugaredLogger) (bool, error) { return true, nil },
			getKubeAPIServerVersion: func() (*version.Info, error) { return &version.Info{Major: "1", Minor: "31"}, nil },
			getConfigDir:            func() string { return "testdata" },
			getK8sPathFn: func(_ *zap.SugaredLogger, _ string, _, _, _ float64) string {
				return "/default.yaml"
			},
			getK8sMinimumSupportedVersionFn: getK8sMinimumSupportedVersion,
			getK8sMaximumSupportedVersionFn: getK8sMaximumSupportedVersion,
			yamlUnmarshal:                   yaml.Unmarshal,
			expectedConfig: operatorutils.OperatorConfig{
				IsOpenShift:     true,
				ConfigDirectory: "testdata",
				K8sVersion: operatorutils.K8sImagesConfig{
					Images: struct {
						Attacher              string `json:"attacher" yaml:"attacher"`
						Provisioner           string `json:"provisioner" yaml:"provisioner"`
						Snapshotter           string `json:"snapshotter" yaml:"snapshotter"`
						Registrar             string `json:"registrar" yaml:"registrar"`
						Resizer               string `json:"resizer" yaml:"resizer"`
						Externalhealthmonitor string `json:"externalhealthmonitorcontroller" yaml:"externalhealthmonitorcontroller"`
						Sdc                   string `json:"sdc" yaml:"sdc"`
						Sdcmonitor            string `json:"sdcmonitor" yaml:"sdcmonitor"`
						Podmon                string `json:"podmon" yaml:"podmon"`
						CSIRevProxy           string `json:"csiReverseProxy" yaml:"csiReverseProxy"`
					}{
						Attacher:              "registry.k8s.io/sig-storage/csi-attacher:v4.8.0",
						Provisioner:           "registry.k8s.io/sig-storage/csi-provisioner:v5.1.0",
						Snapshotter:           "registry.k8s.io/sig-storage/csi-snapshotter:v8.2.0",
						Registrar:             "registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.13.0",
						Resizer:               "registry.k8s.io/sig-storage/csi-resizer:v1.13.1",
						Externalhealthmonitor: "registry.k8s.io/sig-storage/csi-external-health-monitor-controller:v0.14.0",
						Sdcmonitor:            "quay.io/dell/storage/powerflex/sdc:4.5.2.1",
					},
				},
			},
		},
		{
			name:                    "Kubernetes environment",
			isOpenShift:             func(_ *zap.SugaredLogger) (bool, error) { return false, nil },
			getKubeAPIServerVersion: func() (*version.Info, error) { return &version.Info{Major: "1", Minor: "31"}, nil },
			getConfigDir:            func() string { return "testdata" },
			getK8sPathFn: func(_ *zap.SugaredLogger, _ string, _, _, _ float64) string {
				return "/default.yaml"
			},
			getK8sMinimumSupportedVersionFn: getK8sMinimumSupportedVersion,
			getK8sMaximumSupportedVersionFn: getK8sMaximumSupportedVersion,
			yamlUnmarshal:                   yaml.Unmarshal,
			expectedConfig: operatorutils.OperatorConfig{
				IsOpenShift:     false,
				ConfigDirectory: "testdata",
				K8sVersion: operatorutils.K8sImagesConfig{
					Images: struct {
						Attacher              string `json:"attacher" yaml:"attacher"`
						Provisioner           string `json:"provisioner" yaml:"provisioner"`
						Snapshotter           string `json:"snapshotter" yaml:"snapshotter"`
						Registrar             string `json:"registrar" yaml:"registrar"`
						Resizer               string `json:"resizer" yaml:"resizer"`
						Externalhealthmonitor string `json:"externalhealthmonitorcontroller" yaml:"externalhealthmonitorcontroller"`
						Sdc                   string `json:"sdc" yaml:"sdc"`
						Sdcmonitor            string `json:"sdcmonitor" yaml:"sdcmonitor"`
						Podmon                string `json:"podmon" yaml:"podmon"`
						CSIRevProxy           string `json:"csiReverseProxy" yaml:"csiReverseProxy"`
					}{
						Attacher:              "registry.k8s.io/sig-storage/csi-attacher:v4.8.0",
						Provisioner:           "registry.k8s.io/sig-storage/csi-provisioner:v5.1.0",
						Snapshotter:           "registry.k8s.io/sig-storage/csi-snapshotter:v8.2.0",
						Registrar:             "registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.13.0",
						Resizer:               "registry.k8s.io/sig-storage/csi-resizer:v1.13.1",
						Externalhealthmonitor: "registry.k8s.io/sig-storage/csi-external-health-monitor-controller:v0.14.0",
						Sdcmonitor:            "quay.io/dell/storage/powerflex/sdc:4.5.2.1",
					},
				},
			},
		},
		{
			name:                    "Bad config directory",
			isOpenShift:             func(_ *zap.SugaredLogger) (bool, error) { return false, nil },
			getKubeAPIServerVersion: func() (*version.Info, error) { return &version.Info{Major: "1", Minor: "31"}, nil },
			getConfigDir:            func() string { return "/bad/path/does/not/exist" },
			getK8sPathFn: func(_ *zap.SugaredLogger, _ string, _, _, _ float64) string {
				return "/default.yaml"
			},
			getK8sMinimumSupportedVersionFn: getK8sMinimumSupportedVersion,
			getK8sMaximumSupportedVersionFn: getK8sMaximumSupportedVersion,
			yamlUnmarshal:                   yaml.Unmarshal,
			expectedConfig: operatorutils.OperatorConfig{
				IsOpenShift:     false,
				ConfigDirectory: "operatorconfig",
				K8sVersion: operatorutils.K8sImagesConfig{
					Images: struct {
						Attacher              string `json:"attacher" yaml:"attacher"`
						Provisioner           string `json:"provisioner" yaml:"provisioner"`
						Snapshotter           string `json:"snapshotter" yaml:"snapshotter"`
						Registrar             string `json:"registrar" yaml:"registrar"`
						Resizer               string `json:"resizer" yaml:"resizer"`
						Externalhealthmonitor string `json:"externalhealthmonitorcontroller" yaml:"externalhealthmonitorcontroller"`
						Sdc                   string `json:"sdc" yaml:"sdc"`
						Sdcmonitor            string `json:"sdcmonitor" yaml:"sdcmonitor"`
						Podmon                string `json:"podmon" yaml:"podmon"`
						CSIRevProxy           string `json:"csiReverseProxy" yaml:"csiReverseProxy"`
					}{},
				},
			},
		},
		{
			name:                    "Fail get openshift",
			isOpenShift:             func(_ *zap.SugaredLogger) (bool, error) { return false, errors.New("error") },
			getKubeAPIServerVersion: func() (*version.Info, error) { return &version.Info{Major: "1", Minor: "31"}, errors.New("error") },
			getConfigDir:            func() string { return "/bad/path/does/not/exist" },
			getK8sPathFn: func(_ *zap.SugaredLogger, _ string, _, _, _ float64) string {
				return "/default.yaml"
			},
			getK8sMinimumSupportedVersionFn: getK8sMinimumSupportedVersion,
			getK8sMaximumSupportedVersionFn: getK8sMaximumSupportedVersion,
			yamlUnmarshal:                   yaml.Unmarshal,
			expectedConfig: operatorutils.OperatorConfig{
				IsOpenShift:     false,
				ConfigDirectory: "operatorconfig",
				K8sVersion: operatorutils.K8sImagesConfig{
					Images: struct {
						Attacher              string `json:"attacher" yaml:"attacher"`
						Provisioner           string `json:"provisioner" yaml:"provisioner"`
						Snapshotter           string `json:"snapshotter" yaml:"snapshotter"`
						Registrar             string `json:"registrar" yaml:"registrar"`
						Resizer               string `json:"resizer" yaml:"resizer"`
						Externalhealthmonitor string `json:"externalhealthmonitorcontroller" yaml:"externalhealthmonitorcontroller"`
						Sdc                   string `json:"sdc" yaml:"sdc"`
						Sdcmonitor            string `json:"sdcmonitor" yaml:"sdcmonitor"`
						Podmon                string `json:"podmon" yaml:"podmon"`
						CSIRevProxy           string `json:"csiReverseProxy" yaml:"csiReverseProxy"`
					}{},
				},
			},
		},
		{
			name:                    "Fail get kube api version",
			isOpenShift:             func(_ *zap.SugaredLogger) (bool, error) { return false, nil },
			getKubeAPIServerVersion: func() (*version.Info, error) { return &version.Info{Major: "1", Minor: "31"}, errors.New("error") },
			getConfigDir:            func() string { return "/bad/path/does/not/exist" },
			getK8sPathFn: func(_ *zap.SugaredLogger, _ string, _, _, _ float64) string {
				return "/default.yaml"
			},
			getK8sMinimumSupportedVersionFn: getK8sMinimumSupportedVersion,
			getK8sMaximumSupportedVersionFn: getK8sMaximumSupportedVersion,
			yamlUnmarshal:                   yaml.Unmarshal,
			expectedConfig: operatorutils.OperatorConfig{
				IsOpenShift:     false,
				ConfigDirectory: "operatorconfig",
				K8sVersion: operatorutils.K8sImagesConfig{
					Images: struct {
						Attacher              string `json:"attacher" yaml:"attacher"`
						Provisioner           string `json:"provisioner" yaml:"provisioner"`
						Snapshotter           string `json:"snapshotter" yaml:"snapshotter"`
						Registrar             string `json:"registrar" yaml:"registrar"`
						Resizer               string `json:"resizer" yaml:"resizer"`
						Externalhealthmonitor string `json:"externalhealthmonitorcontroller" yaml:"externalhealthmonitorcontroller"`
						Sdc                   string `json:"sdc" yaml:"sdc"`
						Sdcmonitor            string `json:"sdcmonitor" yaml:"sdcmonitor"`
						Podmon                string `json:"podmon" yaml:"podmon"`
						CSIRevProxy           string `json:"csiReverseProxy" yaml:"csiReverseProxy"`
					}{},
				},
			},
		},
		{
			name:                    "Fail parse K8sMinimumSupportedVersion",
			isOpenShift:             func(_ *zap.SugaredLogger) (bool, error) { return false, nil },
			getKubeAPIServerVersion: func() (*version.Info, error) { return &version.Info{Major: "1", Minor: "31"}, errors.New("error") },
			getConfigDir:            func() string { return "/bad/path/does/not/exist" },
			getK8sPathFn: func(_ *zap.SugaredLogger, _ string, _, _, _ float64) string {
				return "/default.yaml"
			},
			getK8sMinimumSupportedVersionFn: func() string { return "test" },
			getK8sMaximumSupportedVersionFn: getK8sMaximumSupportedVersion,
			yamlUnmarshal:                   yaml.Unmarshal,
			expectedConfig: operatorutils.OperatorConfig{
				IsOpenShift:     false,
				ConfigDirectory: "operatorconfig",
				K8sVersion: operatorutils.K8sImagesConfig{
					Images: struct {
						Attacher              string `json:"attacher" yaml:"attacher"`
						Provisioner           string `json:"provisioner" yaml:"provisioner"`
						Snapshotter           string `json:"snapshotter" yaml:"snapshotter"`
						Registrar             string `json:"registrar" yaml:"registrar"`
						Resizer               string `json:"resizer" yaml:"resizer"`
						Externalhealthmonitor string `json:"externalhealthmonitorcontroller" yaml:"externalhealthmonitorcontroller"`
						Sdc                   string `json:"sdc" yaml:"sdc"`
						Sdcmonitor            string `json:"sdcmonitor" yaml:"sdcmonitor"`
						Podmon                string `json:"podmon" yaml:"podmon"`
						CSIRevProxy           string `json:"csiReverseProxy" yaml:"csiReverseProxy"`
					}{},
				},
			},
		},
		{
			name:                    "Fail parse K8sMaximumSupportedVersion",
			isOpenShift:             func(_ *zap.SugaredLogger) (bool, error) { return false, nil },
			getKubeAPIServerVersion: func() (*version.Info, error) { return &version.Info{Major: "1", Minor: "31"}, errors.New("error") },
			getConfigDir:            func() string { return "/bad/path/does/not/exist" },
			getK8sPathFn: func(_ *zap.SugaredLogger, _ string, _, _, _ float64) string {
				return "/default.yaml"
			},
			getK8sMinimumSupportedVersionFn: getK8sMinimumSupportedVersion,
			getK8sMaximumSupportedVersionFn: func() string { return "test" },
			yamlUnmarshal:                   yaml.Unmarshal,
			expectedConfig: operatorutils.OperatorConfig{
				IsOpenShift:     false,
				ConfigDirectory: "operatorconfig",
				K8sVersion: operatorutils.K8sImagesConfig{
					Images: struct {
						Attacher              string `json:"attacher" yaml:"attacher"`
						Provisioner           string `json:"provisioner" yaml:"provisioner"`
						Snapshotter           string `json:"snapshotter" yaml:"snapshotter"`
						Registrar             string `json:"registrar" yaml:"registrar"`
						Resizer               string `json:"resizer" yaml:"resizer"`
						Externalhealthmonitor string `json:"externalhealthmonitorcontroller" yaml:"externalhealthmonitorcontroller"`
						Sdc                   string `json:"sdc" yaml:"sdc"`
						Sdcmonitor            string `json:"sdcmonitor" yaml:"sdcmonitor"`
						Podmon                string `json:"podmon" yaml:"podmon"`
						CSIRevProxy           string `json:"csiReverseProxy" yaml:"csiReverseProxy"`
					}{},
				},
			},
		},
		{
			name:                    "Fail parse kube version",
			isOpenShift:             func(_ *zap.SugaredLogger) (bool, error) { return false, nil },
			getKubeAPIServerVersion: func() (*version.Info, error) { return &version.Info{Major: "test", Minor: "test"}, nil },
			getConfigDir:            func() string { return "/bad/path/does/not/exist" },
			getK8sPathFn: func(_ *zap.SugaredLogger, _ string, _, _, _ float64) string {
				return "/default.yaml"
			},
			getK8sMinimumSupportedVersionFn: getK8sMinimumSupportedVersion,
			getK8sMaximumSupportedVersionFn: func() string { return "test" },
			yamlUnmarshal:                   yaml.Unmarshal,
			expectedConfig: operatorutils.OperatorConfig{
				IsOpenShift:     false,
				ConfigDirectory: "operatorconfig",
				K8sVersion: operatorutils.K8sImagesConfig{
					Images: struct {
						Attacher              string `json:"attacher" yaml:"attacher"`
						Provisioner           string `json:"provisioner" yaml:"provisioner"`
						Snapshotter           string `json:"snapshotter" yaml:"snapshotter"`
						Registrar             string `json:"registrar" yaml:"registrar"`
						Resizer               string `json:"resizer" yaml:"resizer"`
						Externalhealthmonitor string `json:"externalhealthmonitorcontroller" yaml:"externalhealthmonitorcontroller"`
						Sdc                   string `json:"sdc" yaml:"sdc"`
						Sdcmonitor            string `json:"sdcmonitor" yaml:"sdcmonitor"`
						Podmon                string `json:"podmon" yaml:"podmon"`
						CSIRevProxy           string `json:"csiReverseProxy" yaml:"csiReverseProxy"`
					}{},
				},
			},
		},
		{
			name:                    "Fail yaml unmarshal",
			isOpenShift:             func(_ *zap.SugaredLogger) (bool, error) { return false, nil },
			getKubeAPIServerVersion: func() (*version.Info, error) { return &version.Info{Major: "1", Minor: "31"}, nil },
			getConfigDir:            func() string { return "/bad/path/does/not/exist" },
			getK8sPathFn: func(_ *zap.SugaredLogger, _ string, _, _, _ float64) string {
				return "/default.yaml"
			},
			getK8sMinimumSupportedVersionFn: getK8sMinimumSupportedVersion,
			getK8sMaximumSupportedVersionFn: func() string { return "test" },
			yamlUnmarshal:                   func(_ []byte, _ interface{}, _ ...yaml.JSONOpt) error { return errors.New("error") },
			wantErr:                         true,
			expectedConfig: operatorutils.OperatorConfig{
				IsOpenShift:     false,
				ConfigDirectory: "operatorconfig",
				K8sVersion: operatorutils.K8sImagesConfig{
					Images: struct {
						Attacher              string `json:"attacher" yaml:"attacher"`
						Provisioner           string `json:"provisioner" yaml:"provisioner"`
						Snapshotter           string `json:"snapshotter" yaml:"snapshotter"`
						Registrar             string `json:"registrar" yaml:"registrar"`
						Resizer               string `json:"resizer" yaml:"resizer"`
						Externalhealthmonitor string `json:"externalhealthmonitorcontroller" yaml:"externalhealthmonitorcontroller"`
						Sdc                   string `json:"sdc" yaml:"sdc"`
						Sdcmonitor            string `json:"sdcmonitor" yaml:"sdcmonitor"`
						Podmon                string `json:"podmon" yaml:"podmon"`
						CSIRevProxy           string `json:"csiReverseProxy" yaml:"csiReverseProxy"`
					}{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalIsOpenShift := isOpenShift
			originalGetKubeAPIServerVersion := getKubeAPIServerVersion
			originalGetConfigDir := getConfigDir
			originalGetK8sPathFn := getk8sPathFn
			originalgetK8sMinimumSupportedVersion := getK8sMinimumSupportedVersion
			originalgetK8sMaximumSupportedVersion := getK8sMaximumSupportedVersion
			originalYamlUnmarshal := yamlUnmarshal
			defer func() {
				isOpenShift = originalIsOpenShift
				getKubeAPIServerVersion = originalGetKubeAPIServerVersion
				getConfigDir = originalGetConfigDir
				getk8sPathFn = originalGetK8sPathFn
				getK8sMinimumSupportedVersion = originalgetK8sMinimumSupportedVersion
				getK8sMaximumSupportedVersion = originalgetK8sMaximumSupportedVersion
				yamlUnmarshal = originalYamlUnmarshal
			}()

			isOpenShift = tt.isOpenShift
			getKubeAPIServerVersion = tt.getKubeAPIServerVersion
			getConfigDir = tt.getConfigDir
			getk8sPathFn = tt.getK8sPathFn
			getK8sMinimumSupportedVersion = tt.getK8sMinimumSupportedVersionFn
			getK8sMaximumSupportedVersion = tt.getK8sMaximumSupportedVersionFn
			yamlUnmarshal = tt.yamlUnmarshal

			// Create a logger
			logger, _ := zap.NewProduction()
			defer logger.Sync()
			sugar := logger.Sugar()

			// Call the function
			cfg, err := getOperatorConfig(sugar)

			// Assert the results
			if tt.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.Equal(t, tt.expectedConfig, cfg)
			}
		})
	}
}

func TestIsOpenshift(t *testing.T) {
	// Create a fake kubeconfig and set the KUBECONFIG environment variable.
	err := k8s.CreateTempKubeconfig("./fake-kubeconfig")
	assert.NoError(t, err)
	_ = os.Setenv("KUBECONFIG", "./fake-kubeconfig")
	var log *zap.SugaredLogger
	var isOpenShiftResult bool
	isOpenShiftResult, _ = isOpenShift(log)
	if isOpenShiftResult != false {
		t.Errorf("IsOpenShift() = %v, want %v", isOpenShiftResult, true)
	}
}

func TestGetKubeAPIServerVersion(t *testing.T) {
	// Create a fake kubeconfig and set the KUBECONFIG environment variable.
	err := k8s.CreateTempKubeconfig("./fake-kubeconfig")
	assert.NoError(t, err)
	_ = os.Setenv("KUBECONFIG", "./fake-kubeconfig")
	_, err = getKubeAPIServerVersion()
	assert.NotNil(t, err)
}

func TestGetConfigDir(t *testing.T) {
	result := getConfigDir()
	assert.Equal(t, ConfigDir, result)
}

func TestNewManager(t *testing.T) {
	_, err := newManager(nil, manager.Options{})
	assert.NotNil(t, err)
}

func TestNewConfigOrDie(t *testing.T) {
	result := newConfigOrDie(&rest.Config{
		Host: "https://10.0.0.1:6443",
	})
	assert.NotNil(t, result)
}

func TestGetSetupWithManagerFn(t *testing.T) {
	r := &controllers.ContainerStorageModuleReconciler{}
	f := getSetupWithManagerFn(r)
	assert.NotNil(t, f)
}

func TestGetk8sPath(t *testing.T) {
	tests := []struct {
		name           string
		kubeVersion    string
		currentVersion float64
		minVersion     float64
		maxVersion     float64
		expectedPath   string
	}{
		{
			name:           "Current version less than minimum",
			kubeVersion:    "1.29",
			currentVersion: 1.29,
			minVersion:     1.32,
			maxVersion:     1.34,
			expectedPath:   "/driverconfig/common/default.yaml",
		},
		{
			name:           "Current version greater than maximum",
			kubeVersion:    "1.35",
			currentVersion: 1.35,
			minVersion:     1.32,
			maxVersion:     1.34,
			expectedPath:   "/driverconfig/common/k8s-1.34-values.yaml",
		},
		{
			name:           "Current version within range",
			kubeVersion:    "1.33",
			currentVersion: 1.33,
			minVersion:     1.32,
			maxVersion:     1.34,
			expectedPath:   "/driverconfig/common/k8s-1.33-values.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a logger
			logger, _ := zap.NewProduction()
			defer logger.Sync()
			sugar := logger.Sugar()

			// Call the function
			actualPath := getk8sPathFn(sugar, tt.kubeVersion, tt.currentVersion, tt.minVersion, tt.maxVersion)

			// Assert the results
			assert.Equal(t, tt.expectedPath, actualPath)
		})
	}
}

var mainCh = make(chan struct{})

func TestMain(_ *testing.T) {
	originalIsOpenShift := isOpenShift
	originalGetKubeAPIServerVersion := getKubeAPIServerVersion
	originalGetConfigDir := getConfigDir
	originalGetK8sPathFn := getk8sPathFn
	originalgetSetupWithManagerFn := getSetupWithManagerFn
	originalInitFlags := initFlags
	originalInitZapFlags := initZapFlags
	originalSetupSignalHandler := setupSignalHandler
	defer func() {
		isOpenShift = originalIsOpenShift
		getKubeAPIServerVersion = originalGetKubeAPIServerVersion
		getConfigDir = originalGetConfigDir
		getk8sPathFn = originalGetK8sPathFn
		getSetupWithManagerFn = originalgetSetupWithManagerFn
		initFlags = originalInitFlags
		initZapFlags = originalInitZapFlags
		setupSignalHandler = originalSetupSignalHandler
	}()

	isOpenShift = func(_ *zap.SugaredLogger) (bool, error) { return true, nil }
	getKubeAPIServerVersion = func() (*version.Info, error) { return &version.Info{Major: "1", Minor: "31"}, nil }
	getConfigDir = func() string { return "testdata" }
	getk8sPathFn = func(_ *zap.SugaredLogger, _ string, _, _, _ float64) string {
		return "/default.yaml"
	}
	getSetupWithManagerFn = func(_ *controllers.ContainerStorageModuleReconciler) func(_ ctrl.Manager, _ workqueue.TypedRateLimiter[reconcile.Request], _ int) error {
		return func(_ ctrl.Manager, _ workqueue.TypedRateLimiter[reconcile.Request], _ int) error {
			return nil
		}
	}

	getConfigOrDie = func() *rest.Config {
		return &rest.Config{
			Host: "https://127.0.0.1:6443",
		}
	}

	setupSignalHandler = func() context.Context {
		return context.Background()
	}

	initZapFlags = func() crzap.Options {
		return crzap.Options{}
	}

	initFlags = func() crzap.Options {
		LEEnabled := false
		metricsBindAddress := ":8082"
		healthProbeBindAddress := ":8081"
		flags.metricsBindAddress = &metricsBindAddress
		flags.healthProbeBindAddress = &healthProbeBindAddress
		flags.leaderElect = &LEEnabled
		opts := initZapFlags()
		return opts
	}

	newManager = func(_ *rest.Config, _ manager.Options) (manager.Manager, error) {
		return &mockManager{
			Cluster: &mockCluster{},
			startFn: func(ctx context.Context) error {
				ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
				defer cancel()

				<-ctx.Done()
				return nil
			},
			addHealthzCheckFn: func(_ string, _ healthz.Checker) error {
				return nil
			},
			addReadyzCheckFn: func(_ string, _ healthz.Checker) error {
				return nil
			},
		}, nil
	}

	newConfigOrDie = func(_ *rest.Config) *kubernetes.Clientset {
		return &kubernetes.Clientset{}
	}

	go func() {
		main()
		mainCh <- struct{}{}
	}()

	<-mainCh
}

func TestMainGetOperatorConfigError(_ *testing.T) {
	originalIsOpenShift := isOpenShift
	originalGetKubeAPIServerVersion := getKubeAPIServerVersion
	originalGetConfigDir := getConfigDir
	originalGetK8sPathFn := getk8sPathFn
	originalgetSetupWithManagerFn := getSetupWithManagerFn
	originalInitFlags := initFlags
	originalInitZapFlags := initZapFlags
	originalSetupSignalHandler := setupSignalHandler
	originalYamlUnmarshal := yamlUnmarshal
	originalOsExit := osExit
	defer func() {
		isOpenShift = originalIsOpenShift
		getKubeAPIServerVersion = originalGetKubeAPIServerVersion
		getConfigDir = originalGetConfigDir
		getk8sPathFn = originalGetK8sPathFn
		getSetupWithManagerFn = originalgetSetupWithManagerFn
		initFlags = originalInitFlags
		initZapFlags = originalInitZapFlags
		setupSignalHandler = originalSetupSignalHandler
		yamlUnmarshal = originalYamlUnmarshal
		osExit = originalOsExit
	}()

	isOpenShift = func(_ *zap.SugaredLogger) (bool, error) { return true, nil }
	getKubeAPIServerVersion = func() (*version.Info, error) { return &version.Info{Major: "1", Minor: "31"}, nil }
	getConfigDir = func() string { return "testdata" }
	getk8sPathFn = func(_ *zap.SugaredLogger, _ string, _, _, _ float64) string {
		return "/default.yaml"
	}
	getSetupWithManagerFn = func(_ *controllers.ContainerStorageModuleReconciler) func(_ ctrl.Manager, _ workqueue.TypedRateLimiter[reconcile.Request], _ int) error {
		return func(_ ctrl.Manager, _ workqueue.TypedRateLimiter[reconcile.Request], _ int) error {
			return nil
		}
	}

	osExitCalled := make(chan struct{})
	osExit = func(_ int) {
		osExitCalled <- struct{}{}
	}

	getConfigOrDie = func() *rest.Config {
		return &rest.Config{
			Host: "https://127.0.0.1:6443",
		}
	}

	yamlUnmarshal = func(_ []byte, _ interface{}, _ ...yaml.JSONOpt) error {
		return errors.New("error")
	}

	setupSignalHandler = func() context.Context {
		return context.Background()
	}

	initZapFlags = func() crzap.Options {
		return crzap.Options{}
	}

	initFlags = func() crzap.Options {
		LEEnabled := false
		metricsBindAddress := ":8082"
		healthProbeBindAddress := ":8081"
		flags.metricsBindAddress = &metricsBindAddress
		flags.healthProbeBindAddress = &healthProbeBindAddress
		flags.leaderElect = &LEEnabled
		opts := initZapFlags()
		return opts
	}

	newManager = func(_ *rest.Config, _ manager.Options) (manager.Manager, error) {
		return &mockManager{
			Cluster: &mockCluster{},
			startFn: func(ctx context.Context) error {
				ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
				defer cancel()

				<-ctx.Done()
				return nil
			},
			addHealthzCheckFn: func(_ string, _ healthz.Checker) error {
				return nil
			},
			addReadyzCheckFn: func(_ string, _ healthz.Checker) error {
				return nil
			},
		}, nil
	}

	newConfigOrDie = func(_ *rest.Config) *kubernetes.Clientset {
		return &kubernetes.Clientset{}
	}

	go func() {
		main()
	}()

	<-osExitCalled
}

func TestMainNewManagerError(_ *testing.T) {
	originalIsOpenShift := isOpenShift
	originalGetKubeAPIServerVersion := getKubeAPIServerVersion
	originalGetConfigDir := getConfigDir
	originalGetK8sPathFn := getk8sPathFn
	originalgetSetupWithManagerFn := getSetupWithManagerFn
	originalInitFlags := initFlags
	originalInitZapFlags := initZapFlags
	originalSetupSignalHandler := setupSignalHandler
	originalOsExit := osExit
	defer func() {
		isOpenShift = originalIsOpenShift
		getKubeAPIServerVersion = originalGetKubeAPIServerVersion
		getConfigDir = originalGetConfigDir
		getk8sPathFn = originalGetK8sPathFn
		getSetupWithManagerFn = originalgetSetupWithManagerFn
		initFlags = originalInitFlags
		initZapFlags = originalInitZapFlags
		setupSignalHandler = originalSetupSignalHandler
		osExit = originalOsExit
	}()

	isOpenShift = func(_ *zap.SugaredLogger) (bool, error) { return true, nil }
	getKubeAPIServerVersion = func() (*version.Info, error) { return &version.Info{Major: "1", Minor: "31"}, nil }
	getConfigDir = func() string { return "testdata" }
	getk8sPathFn = func(_ *zap.SugaredLogger, _ string, _, _, _ float64) string {
		return "/default.yaml"
	}
	getSetupWithManagerFn = func(_ *controllers.ContainerStorageModuleReconciler) func(_ ctrl.Manager, _ workqueue.TypedRateLimiter[reconcile.Request], _ int) error {
		return func(_ ctrl.Manager, _ workqueue.TypedRateLimiter[reconcile.Request], _ int) error {
			return nil
		}
	}

	getConfigOrDie = func() *rest.Config {
		return &rest.Config{
			Host: "https://127.0.0.1:6443",
		}
	}

	osExitCalled := make(chan struct{})
	osExit = func(_ int) {
		osExitCalled <- struct{}{}
	}

	initZapFlags = func() crzap.Options {
		return crzap.Options{}
	}

	initFlags = func() crzap.Options {
		LEEnabled := false
		metricsBindAddress := ":8082"
		healthProbeBindAddress := ":8081"
		flags.metricsBindAddress = &metricsBindAddress
		flags.healthProbeBindAddress = &healthProbeBindAddress
		flags.leaderElect = &LEEnabled
		opts := initZapFlags()
		return opts
	}

	newManager = func(_ *rest.Config, _ manager.Options) (manager.Manager, error) {
		return nil, errors.New("error")
	}

	newConfigOrDie = func(_ *rest.Config) *kubernetes.Clientset {
		return &kubernetes.Clientset{}
	}

	go func() {
		main()
	}()

	<-osExitCalled
}

func TestMainSetupWithManagerError(_ *testing.T) {
	originalIsOpenShift := isOpenShift
	originalGetKubeAPIServerVersion := getKubeAPIServerVersion
	originalGetConfigDir := getConfigDir
	originalGetK8sPathFn := getk8sPathFn
	originalgetSetupWithManagerFn := getSetupWithManagerFn
	originalOsExit := osExit
	originalInitFlags := initFlags
	originalInitZapFlags := initZapFlags
	defer func() {
		isOpenShift = originalIsOpenShift
		getKubeAPIServerVersion = originalGetKubeAPIServerVersion
		getConfigDir = originalGetConfigDir
		getk8sPathFn = originalGetK8sPathFn
		getSetupWithManagerFn = originalgetSetupWithManagerFn
		osExit = originalOsExit
		initFlags = originalInitFlags
		initZapFlags = originalInitZapFlags
	}()

	isOpenShift = func(_ *zap.SugaredLogger) (bool, error) { return true, nil }
	getKubeAPIServerVersion = func() (*version.Info, error) { return &version.Info{Major: "1", Minor: "31"}, nil }
	getConfigDir = func() string { return "testdata" }
	getk8sPathFn = func(_ *zap.SugaredLogger, _ string, _, _, _ float64) string {
		return "/default.yaml"
	}
	getSetupWithManagerFn = func(_ *controllers.ContainerStorageModuleReconciler) func(_ ctrl.Manager, _ workqueue.TypedRateLimiter[reconcile.Request], _ int) error {
		return func(_ ctrl.Manager, _ workqueue.TypedRateLimiter[reconcile.Request], _ int) error {
			return errors.New("error")
		}
	}

	initZapFlags = func() crzap.Options {
		return crzap.Options{}
	}
	initFlags = func() crzap.Options {
		LEEnabled := false
		metricsBindAddress := ":8082"
		healthProbeBindAddress := ":8081"
		flags.metricsBindAddress = &metricsBindAddress
		flags.healthProbeBindAddress = &healthProbeBindAddress
		flags.leaderElect = &LEEnabled
		opts := initZapFlags()
		return opts
	}

	osExitCalled := make(chan struct{})
	osExit = func(_ int) {
		osExitCalled <- struct{}{}
	}

	getConfigOrDie = func() *rest.Config {
		return &rest.Config{
			Host: "https://127.0.0.1:6443",
		}
	}

	newManager = func(_ *rest.Config, _ manager.Options) (manager.Manager, error) {
		return &mockManager{
			Cluster: &mockCluster{},
		}, nil
	}

	newConfigOrDie = func(_ *rest.Config) *kubernetes.Clientset {
		return &kubernetes.Clientset{}
	}

	go func() {
		main()
	}()

	<-osExitCalled
}

func TestMainAddHealthzCheckError(_ *testing.T) {
	originalIsOpenShift := isOpenShift
	originalGetKubeAPIServerVersion := getKubeAPIServerVersion
	originalGetConfigDir := getConfigDir
	originalGetK8sPathFn := getk8sPathFn
	originalgetSetupWithManagerFn := getSetupWithManagerFn
	originalOsExit := osExit
	originalInitFlags := initFlags
	originalInitZapFlags := initZapFlags
	originalGetControllerWatchCh := getControllerWatchCh
	defer func() {
		isOpenShift = originalIsOpenShift
		getKubeAPIServerVersion = originalGetKubeAPIServerVersion
		getConfigDir = originalGetConfigDir
		getk8sPathFn = originalGetK8sPathFn
		getSetupWithManagerFn = originalgetSetupWithManagerFn
		osExit = originalOsExit
		initFlags = originalInitFlags
		initZapFlags = originalInitZapFlags
		getControllerWatchCh = originalGetControllerWatchCh
	}()

	isOpenShift = func(_ *zap.SugaredLogger) (bool, error) { return true, nil }
	getKubeAPIServerVersion = func() (*version.Info, error) { return &version.Info{Major: "1", Minor: "31"}, nil }
	getConfigDir = func() string { return "testdata" }
	getk8sPathFn = func(_ *zap.SugaredLogger, _ string, _, _, _ float64) string {
		return "/default.yaml"
	}
	getSetupWithManagerFn = func(_ *controllers.ContainerStorageModuleReconciler) func(_ ctrl.Manager, _ workqueue.TypedRateLimiter[reconcile.Request], _ int) error {
		return func(_ ctrl.Manager, _ workqueue.TypedRateLimiter[reconcile.Request], _ int) error {
			return nil
		}
	}

	initZapFlags = func() crzap.Options {
		return crzap.Options{}
	}
	initFlags = func() crzap.Options {
		LEEnabled := false
		metricsBindAddress := ":8082"
		healthProbeBindAddress := ":8081"
		flags.metricsBindAddress = &metricsBindAddress
		flags.healthProbeBindAddress = &healthProbeBindAddress
		flags.leaderElect = &LEEnabled
		opts := initZapFlags()
		return opts
	}

	osExitCalled := make(chan struct{})
	osExit = func(_ int) {
		osExitCalled <- struct{}{}
	}

	getControllerWatchCh = func() chan struct{} {
		return make(chan struct{})
	}

	getConfigOrDie = func() *rest.Config {
		return &rest.Config{
			Host: "https://127.0.0.1:6443",
		}
	}

	newManager = func(_ *rest.Config, _ manager.Options) (manager.Manager, error) {
		return &mockManager{
			Cluster:           &mockCluster{},
			addHealthzCheckFn: func(_ string, _ healthz.Checker) error { return errors.New("error") },
		}, nil
	}

	newConfigOrDie = func(_ *rest.Config) *kubernetes.Clientset {
		return &kubernetes.Clientset{}
	}

	go func() {
		main()
	}()

	<-osExitCalled
}

func TestMainAddReadyzCheckError(_ *testing.T) {
	originalIsOpenShift := isOpenShift
	originalGetKubeAPIServerVersion := getKubeAPIServerVersion
	originalGetConfigDir := getConfigDir
	originalGetK8sPathFn := getk8sPathFn
	originalgetSetupWithManagerFn := getSetupWithManagerFn
	originalOsExit := osExit
	originalInitFlags := initFlags
	originalInitZapFlags := initZapFlags
	originalGetControllerWatchCh := getControllerWatchCh
	defer func() {
		isOpenShift = originalIsOpenShift
		getKubeAPIServerVersion = originalGetKubeAPIServerVersion
		getConfigDir = originalGetConfigDir
		getk8sPathFn = originalGetK8sPathFn
		getSetupWithManagerFn = originalgetSetupWithManagerFn
		osExit = originalOsExit
		initFlags = originalInitFlags
		initZapFlags = originalInitZapFlags
		getControllerWatchCh = originalGetControllerWatchCh
	}()

	isOpenShift = func(_ *zap.SugaredLogger) (bool, error) { return true, nil }
	getKubeAPIServerVersion = func() (*version.Info, error) { return &version.Info{Major: "1", Minor: "31"}, nil }
	getConfigDir = func() string { return "testdata" }
	getk8sPathFn = func(_ *zap.SugaredLogger, _ string, _, _, _ float64) string {
		return "/default.yaml"
	}
	getSetupWithManagerFn = func(_ *controllers.ContainerStorageModuleReconciler) func(_ ctrl.Manager, _ workqueue.TypedRateLimiter[reconcile.Request], _ int) error {
		return func(_ ctrl.Manager, _ workqueue.TypedRateLimiter[reconcile.Request], _ int) error {
			return nil
		}
	}

	initZapFlags = func() crzap.Options {
		return crzap.Options{}
	}
	initFlags = func() crzap.Options {
		LEEnabled := false
		metricsBindAddress := ":8082"
		healthProbeBindAddress := ":8081"
		flags.metricsBindAddress = &metricsBindAddress
		flags.healthProbeBindAddress = &healthProbeBindAddress
		flags.leaderElect = &LEEnabled
		opts := initZapFlags()
		return opts
	}

	osExitCalled := make(chan struct{})
	osExit = func(_ int) {
		osExitCalled <- struct{}{}
	}

	getControllerWatchCh = func() chan struct{} {
		return make(chan struct{})
	}

	getConfigOrDie = func() *rest.Config {
		return &rest.Config{
			Host: "https://127.0.0.1:6443",
		}
	}

	newManager = func(_ *rest.Config, _ manager.Options) (manager.Manager, error) {
		return &mockManager{
			Cluster:           &mockCluster{},
			addHealthzCheckFn: func(_ string, _ healthz.Checker) error { return nil },
			addReadyzCheckFn:  func(_ string, _ healthz.Checker) error { return errors.New("error") },
		}, nil
	}

	newConfigOrDie = func(_ *rest.Config) *kubernetes.Clientset {
		return &kubernetes.Clientset{}
	}

	go func() {
		main()
	}()

	<-osExitCalled
}

func TestMainStartError(_ *testing.T) {
	originalIsOpenShift := isOpenShift
	originalGetKubeAPIServerVersion := getKubeAPIServerVersion
	originalGetConfigDir := getConfigDir
	originalGetK8sPathFn := getk8sPathFn
	originalgetSetupWithManagerFn := getSetupWithManagerFn
	originalOsExit := osExit
	originalInitFlags := initFlags
	originalInitZapFlags := initZapFlags
	originalGetControllerWatchCh := getControllerWatchCh
	originalSetupSignalHandler := setupSignalHandler
	defer func() {
		isOpenShift = originalIsOpenShift
		getKubeAPIServerVersion = originalGetKubeAPIServerVersion
		getConfigDir = originalGetConfigDir
		getk8sPathFn = originalGetK8sPathFn
		getSetupWithManagerFn = originalgetSetupWithManagerFn
		osExit = originalOsExit
		initFlags = originalInitFlags
		initZapFlags = originalInitZapFlags
		getControllerWatchCh = originalGetControllerWatchCh
		setupSignalHandler = originalSetupSignalHandler
	}()

	isOpenShift = func(_ *zap.SugaredLogger) (bool, error) { return true, nil }
	getKubeAPIServerVersion = func() (*version.Info, error) { return &version.Info{Major: "1", Minor: "31"}, nil }
	getConfigDir = func() string { return "testdata" }
	getk8sPathFn = func(_ *zap.SugaredLogger, _ string, _, _, _ float64) string {
		return "/default.yaml"
	}
	getSetupWithManagerFn = func(_ *controllers.ContainerStorageModuleReconciler) func(_ ctrl.Manager, _ workqueue.TypedRateLimiter[reconcile.Request], _ int) error {
		return func(_ ctrl.Manager, _ workqueue.TypedRateLimiter[reconcile.Request], _ int) error {
			return nil
		}
	}

	setupSignalHandler = func() context.Context {
		return context.Background()
	}

	initZapFlags = func() crzap.Options {
		return crzap.Options{}
	}
	initFlags = func() crzap.Options {
		LEEnabled := false
		metricsBindAddress := ":8082"
		healthProbeBindAddress := ":8081"
		flags.metricsBindAddress = &metricsBindAddress
		flags.healthProbeBindAddress = &healthProbeBindAddress
		flags.leaderElect = &LEEnabled
		opts := initZapFlags()
		return opts
	}

	osExitCalled := make(chan struct{})
	osExit = func(_ int) {
		osExitCalled <- struct{}{}
	}

	getControllerWatchCh = func() chan struct{} {
		return make(chan struct{})
	}

	getConfigOrDie = func() *rest.Config {
		return &rest.Config{
			Host: "https://127.0.0.1:6443",
		}
	}

	newManager = func(_ *rest.Config, _ manager.Options) (manager.Manager, error) {
		return &mockManager{
			Cluster:           &mockCluster{},
			addHealthzCheckFn: func(_ string, _ healthz.Checker) error { return nil },
			addReadyzCheckFn:  func(_ string, _ healthz.Checker) error { return nil },
			startFn:           func(_ context.Context) error { return errors.New("error") },
		}, nil
	}

	newConfigOrDie = func(_ *rest.Config) *kubernetes.Clientset {
		return &kubernetes.Clientset{}
	}

	go func() {
		main()
	}()

	<-osExitCalled
}

func TestInitFlags(t *testing.T) {
	opts := initFlags()
	// Should be set to true
	devFlag := opts.Development
	assert.Equal(t, true, devFlag)
}

type mockManager struct {
	cluster.Cluster
	startFn           func(ctx context.Context) error
	addHealthzCheckFn func(name string, check healthz.Checker) error
	addReadyzCheckFn  func(name string, check healthz.Checker) error
}

func (m *mockManager) Add(_ manager.Runnable) error {
	return nil
}

func (m *mockManager) Elected() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (m *mockManager) AddMetricsServerExtraHandler(_ string, _ http.Handler) error {
	return nil
}

func (m *mockManager) AddHealthzCheck(name string, check healthz.Checker) error {
	return m.addHealthzCheckFn(name, check)
}

func (m *mockManager) AddReadyzCheck(name string, check healthz.Checker) error {
	return m.addReadyzCheckFn(name, check)
}

func (m *mockManager) Start(ctx context.Context) error {
	return m.startFn(ctx)
}

func (m *mockManager) GetWebhookServer() webhook.Server {
	return nil
}

func (m *mockManager) GetLogger() logr.Logger {
	return logr.Discard()
}

func (m *mockManager) GetControllerOptions() config.Controller {
	return config.Controller{}
}

type mockCluster struct{}

func (m *mockCluster) GetHTTPClient() *http.Client {
	return &http.Client{}
}

func (m *mockCluster) GetConfig() *rest.Config {
	return &rest.Config{}
}

func (m *mockCluster) GetCache() cache.Cache {
	return nil
}

func (m *mockCluster) GetScheme() *k8sruntime.Scheme {
	return &k8sruntime.Scheme{}
}

func (m *mockCluster) GetClient() client.Client {
	return fake.NewFakeClient()
}

func (m *mockCluster) GetFieldIndexer() client.FieldIndexer {
	return nil
}

func (m *mockCluster) GetEventRecorderFor(_ string) record.EventRecorder {
	return nil
}

func (m *mockCluster) GetRESTMapper() meta.RESTMapper {
	return nil
}

func (m *mockCluster) GetAPIReader() client.Reader {
	return nil
}

func (m *mockCluster) Start(_ context.Context) error {
	return nil
}
