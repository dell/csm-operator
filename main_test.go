package main

import (
	"errors"
	"testing"

	"github.com/dell/csm-operator/pkg/logger"
	"github.com/dell/csm-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/version"
)

func TestPrintVersion(t *testing.T) {
	_, log := logger.GetNewContextWithLogger("main")
	// TODO: Hook onto the output and verify that it matches the expected
	printVersion(log)
}

func TestGetOperatorConfig(t *testing.T) {
	tests := []struct {
		name                    string
		isOpenShift             func() (bool, error)
		getKubeAPIServerVersion func() (*version.Info, error)
		getConfigDir            func() string
		getK8sPathFn            func(log *zap.SugaredLogger, kubeVersion string, currentVersion, minVersion, maxVersion float64) string
		expectedConfig          utils.OperatorConfig
	}{
		{
			name:                    "Openshift environment",
			isOpenShift:             func() (bool, error) { return true, nil },
			getKubeAPIServerVersion: func() (*version.Info, error) { return &version.Info{Major: "1", Minor: "31"}, nil },
			getConfigDir:            func() string { return "testdata" },
			getK8sPathFn: func(log *zap.SugaredLogger, kubeVersion string, currentVersion, minVersion, maxVersion float64) string {
				return "/default.yaml"
			},
			expectedConfig: utils.OperatorConfig{
				IsOpenShift:     true,
				ConfigDirectory: "testdata",
				K8sVersion: utils.K8sImagesConfig{
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
			isOpenShift:             func() (bool, error) { return false, nil },
			getKubeAPIServerVersion: func() (*version.Info, error) { return &version.Info{Major: "1", Minor: "31"}, nil },
			getConfigDir:            func() string { return "testdata" },
			getK8sPathFn: func(log *zap.SugaredLogger, kubeVersion string, currentVersion, minVersion, maxVersion float64) string {
				return "/default.yaml"
			},
			expectedConfig: utils.OperatorConfig{
				IsOpenShift:     false,
				ConfigDirectory: "testdata",
				K8sVersion: utils.K8sImagesConfig{
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
			isOpenShift:             func() (bool, error) { return false, nil },
			getKubeAPIServerVersion: func() (*version.Info, error) { return &version.Info{Major: "1", Minor: "31"}, nil },
			getConfigDir:            func() string { return "/bad/path/does/not/exist" },
			getK8sPathFn: func(log *zap.SugaredLogger, kubeVersion string, currentVersion, minVersion, maxVersion float64) string {
				return "/default.yaml"
			},
			expectedConfig: utils.OperatorConfig{
				IsOpenShift:     false,
				ConfigDirectory: "operatorconfig",
				K8sVersion: utils.K8sImagesConfig{
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
			isOpenShift:             func() (bool, error) { return false, errors.New("error") },
			getKubeAPIServerVersion: func() (*version.Info, error) { return &version.Info{Major: "1", Minor: "31"}, errors.New("error") },
			getConfigDir:            func() string { return "/bad/path/does/not/exist" },
			getK8sPathFn: func(log *zap.SugaredLogger, kubeVersion string, currentVersion, minVersion, maxVersion float64) string {
				return "/default.yaml"
			},
			expectedConfig: utils.OperatorConfig{
				IsOpenShift:     false,
				ConfigDirectory: "operatorconfig",
				K8sVersion: utils.K8sImagesConfig{
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
			isOpenShift:             func() (bool, error) { return false, nil },
			getKubeAPIServerVersion: func() (*version.Info, error) { return &version.Info{Major: "1", Minor: "31"}, errors.New("error") },
			getConfigDir:            func() string { return "/bad/path/does/not/exist" },
			getK8sPathFn: func(log *zap.SugaredLogger, kubeVersion string, currentVersion, minVersion, maxVersion float64) string {
				return "/default.yaml"
			},
			expectedConfig: utils.OperatorConfig{
				IsOpenShift:     false,
				ConfigDirectory: "operatorconfig",
				K8sVersion: utils.K8sImagesConfig{
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
			defer func() {
				isOpenShift = originalIsOpenShift
				getKubeAPIServerVersion = originalGetKubeAPIServerVersion
				getConfigDir = originalGetConfigDir
				getk8sPathFn = originalGetK8sPathFn
			}()

			isOpenShift = tt.isOpenShift
			getKubeAPIServerVersion = tt.getKubeAPIServerVersion
			getConfigDir = tt.getConfigDir
			getk8sPathFn = tt.getK8sPathFn

			// Create a logger
			logger, _ := zap.NewProduction()
			defer logger.Sync()
			sugar := logger.Sugar()

			// Call the function
			cfg := getOperatorConfig(sugar)

			// Assert the results
			assert.Equal(t, tt.expectedConfig, cfg)
		})
	}
}
