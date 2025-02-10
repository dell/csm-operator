package main

import (
	"context"
	"errors"
	"net/http"
	"runtime"
	"testing"
	"time"

	"github.com/dell/csm-operator/controllers"
	"github.com/dell/csm-operator/pkg/logger"
	"github.com/dell/csm-operator/pkg/utils"
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
			kubeVersion:    "1.18",
			currentVersion: 1.18,
			minVersion:     1.19,
			maxVersion:     1.21,
			expectedPath:   "/driverconfig/common/default.yaml",
		},
		{
			name:           "Current version greater than maximum",
			kubeVersion:    "1.22",
			currentVersion: 1.22,
			minVersion:     1.19,
			maxVersion:     1.21,
			expectedPath:   "/driverconfig/common/k8s-1.30-values.yaml",
		},
		{
			name:           "Current version within range",
			kubeVersion:    "1.20",
			currentVersion: 1.20,
			minVersion:     1.19,
			maxVersion:     1.21,
			expectedPath:   "/driverconfig/common/k8s-1.20-values.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a logger
			logger, _ := zap.NewProduction()
			defer logger.Sync()
			sugar := logger.Sugar()

			// Call the function
			actualPath := getk8sPath(sugar, tt.kubeVersion, tt.currentVersion, tt.minVersion, tt.maxVersion)

			// Assert the results
			assert.Equal(t, tt.expectedPath, actualPath)
		})
	}
}

var (
	mainCh = make(chan struct{})
)

func TestMain(t *testing.T) {
	originalIsOpenShift := isOpenShift
	originalGetKubeAPIServerVersion := getKubeAPIServerVersion
	originalGetConfigDir := getConfigDir
	originalGetK8sPathFn := getk8sPathFn
	originalgetSetupWithManagerFn := getSetupWithManagerFn
	originalInitFlags := initFlags
	originalInitZapFlags := initZapFlags
	defer func() {
		isOpenShift = originalIsOpenShift
		getKubeAPIServerVersion = originalGetKubeAPIServerVersion
		getConfigDir = originalGetConfigDir
		getk8sPathFn = originalGetK8sPathFn
		getSetupWithManagerFn = originalgetSetupWithManagerFn
		initFlags = originalInitFlags
		initZapFlags = originalInitZapFlags
	}()

	isOpenShift = func() (bool, error) { return true, nil }
	getKubeAPIServerVersion = func() (*version.Info, error) { return &version.Info{Major: "1", Minor: "31"}, nil }
	getConfigDir = func() string { return "testdata" }
	getk8sPathFn = func(log *zap.SugaredLogger, kubeVersion string, currentVersion, minVersion, maxVersion float64) string {
		return "/default.yaml"
	}
	getSetupWithManagerFn = func(r *controllers.ContainerStorageModuleReconciler) func(mgr ctrl.Manager, limiter workqueue.TypedRateLimiter[reconcile.Request], maxReconcilers int) error {
		return func(mgr ctrl.Manager, limiter workqueue.TypedRateLimiter[reconcile.Request], maxReconcilers int) error {
			return nil
		}
	}

	getConfigOrDie = func() *rest.Config {
		return &rest.Config{
			Host: "https://127.0.0.1:6443",
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

	newManager = func(_ *rest.Config, _ manager.Options) (manager.Manager, error) {
		return &mockManager{
			Cluster: &mockCluster{},
			startFn: func(ctx context.Context) error {
				ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
				defer cancel()

				<-ctx.Done()
				return nil
			},
			addHealthzCheckFn: func(name string, check healthz.Checker) error {
				return nil
			},
			addReadyzCheckFn: func(name string, check healthz.Checker) error {
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
func TestMainSetupWithManagerError(t *testing.T) {
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

	isOpenShift = func() (bool, error) { return true, nil }
	getKubeAPIServerVersion = func() (*version.Info, error) { return &version.Info{Major: "1", Minor: "31"}, nil }
	getConfigDir = func() string { return "testdata" }
	getk8sPathFn = func(log *zap.SugaredLogger, kubeVersion string, currentVersion, minVersion, maxVersion float64) string {
		return "/default.yaml"
	}
	getSetupWithManagerFn = func(r *controllers.ContainerStorageModuleReconciler) func(mgr ctrl.Manager, limiter workqueue.TypedRateLimiter[reconcile.Request], maxReconcilers int) error {
		return func(mgr ctrl.Manager, limiter workqueue.TypedRateLimiter[reconcile.Request], maxReconcilers int) error {
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
	osExit = func(code int) {
		osExitCalled <- struct{}{}
		runtime.Goexit()
	}

	getConfigOrDie = func() *rest.Config {
		return &rest.Config{
			Host: "https://127.0.0.1:6443",
		}
	}

	newManager = func(_ *rest.Config, _ manager.Options) (manager.Manager, error) {
		return &mockManager{
			Cluster: &mockCluster{}}, nil
	}

	newConfigOrDie = func(_ *rest.Config) *kubernetes.Clientset {
		return &kubernetes.Clientset{}
	}

	go func() {
		main()
	}()

	<-osExitCalled
}

func TestMainAddHealthzCheckError(t *testing.T) {
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

	isOpenShift = func() (bool, error) { return true, nil }
	getKubeAPIServerVersion = func() (*version.Info, error) { return &version.Info{Major: "1", Minor: "31"}, nil }
	getConfigDir = func() string { return "testdata" }
	getk8sPathFn = func(log *zap.SugaredLogger, kubeVersion string, currentVersion, minVersion, maxVersion float64) string {
		return "/default.yaml"
	}
	getSetupWithManagerFn = func(r *controllers.ContainerStorageModuleReconciler) func(mgr ctrl.Manager, limiter workqueue.TypedRateLimiter[reconcile.Request], maxReconcilers int) error {
		return func(mgr ctrl.Manager, limiter workqueue.TypedRateLimiter[reconcile.Request], maxReconcilers int) error {
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
	osExit = func(code int) {
		osExitCalled <- struct{}{}
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

func TestMainAddReadyzCheckError(t *testing.T) {
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

	isOpenShift = func() (bool, error) { return true, nil }
	getKubeAPIServerVersion = func() (*version.Info, error) { return &version.Info{Major: "1", Minor: "31"}, nil }
	getConfigDir = func() string { return "testdata" }
	getk8sPathFn = func(log *zap.SugaredLogger, kubeVersion string, currentVersion, minVersion, maxVersion float64) string {
		return "/default.yaml"
	}
	getSetupWithManagerFn = func(r *controllers.ContainerStorageModuleReconciler) func(mgr ctrl.Manager, limiter workqueue.TypedRateLimiter[reconcile.Request], maxReconcilers int) error {
		return func(mgr ctrl.Manager, limiter workqueue.TypedRateLimiter[reconcile.Request], maxReconcilers int) error {
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
	osExit = func(code int) {
		osExitCalled <- struct{}{}
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
			addReadyzCheckFn:  func(name string, check healthz.Checker) error { return errors.New("error") },
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

func TestMainStartError(t *testing.T) {
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

	isOpenShift = func() (bool, error) { return true, nil }
	getKubeAPIServerVersion = func() (*version.Info, error) { return &version.Info{Major: "1", Minor: "31"}, nil }
	getConfigDir = func() string { return "testdata" }
	getk8sPathFn = func(log *zap.SugaredLogger, kubeVersion string, currentVersion, minVersion, maxVersion float64) string {
		return "/default.yaml"
	}
	getSetupWithManagerFn = func(r *controllers.ContainerStorageModuleReconciler) func(mgr ctrl.Manager, limiter workqueue.TypedRateLimiter[reconcile.Request], maxReconcilers int) error {
		return func(mgr ctrl.Manager, limiter workqueue.TypedRateLimiter[reconcile.Request], maxReconcilers int) error {
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
	osExit = func(code int) {
		osExitCalled <- struct{}{}
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
			addReadyzCheckFn:  func(name string, check healthz.Checker) error { return nil },
			startFn:           func(ctx context.Context) error { return errors.New("error") },
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

type mockManager struct {
	cluster.Cluster
	startFn           func(ctx context.Context) error
	addHealthzCheckFn func(name string, check healthz.Checker) error
	addReadyzCheckFn  func(name string, check healthz.Checker) error
}

func (m *mockManager) Add(r manager.Runnable) error {
	return nil
}

func (m *mockManager) Elected() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (m *mockManager) AddMetricsServerExtraHandler(path string, handler http.Handler) error {
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

func (m *mockCluster) GetEventRecorderFor(name string) record.EventRecorder {
	return nil
}

func (m *mockCluster) GetRESTMapper() meta.RESTMapper {
	return nil
}

func (m *mockCluster) GetAPIReader() client.Reader {
	return nil
}

func (m *mockCluster) Start(ctx context.Context) error {
	return nil
}

/*type MockServer struct{}

func (m *MockServer) NeedLeaderElection() bool {
	return false
}

func (m *MockServer) Register(path string, hook http.Handler) {
}

func (m *MockServer) Start(ctx context.Context) error {
	return nil
}

func (m *MockServer) StartedChecker() healthz.Checker {
	return healthz.Ping
}

func (m *MockServer) WebhookMux() *http.ServeMux {
	return http.NewServeMux()
}
*/
