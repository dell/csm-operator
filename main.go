//  Copyright © 2021 - 2025 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"flag"
	"fmt"
	"os"
	"path/filepath"
	osruntime "runtime"
	"strconv"
	"strings"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/version"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	crzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/yaml"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/controllers"
	"github.com/dell/csm-operator/core"
	k8sClient "github.com/dell/csm-operator/k8s"
	"github.com/dell/csm-operator/pkg/logger"
	utils "github.com/dell/csm-operator/pkg/utils"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	//+kubebuilder:scaffold:imports
)

const (
	// ConfigDir path to driver deployment files
	ConfigDir = "/etc/config/dell-csm-operator"
	// Operatorconfig sub folder for deployment files
	Operatorconfig = "operatorconfig"
	// K8sMinimumSupportedVersion is the minimum supported version for k8s
	K8sMinimumSupportedVersion = "1.28"
	// K8sMaximumSupportedVersion is the maximum supported version for k8s
	K8sMaximumSupportedVersion = "1.30"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(csmv1.AddToScheme(scheme))

	utilruntime.Must(velerov1.AddToScheme(scheme))

	utilruntime.Must(apiextv1.AddToScheme(scheme))

	utilruntime.Must(certmanagerv1.AddToScheme(scheme))

	//+kubebuilder:scaffold:scheme
}

func printVersion(log *zap.SugaredLogger) {
	log.Debugw("Operator Version", "Version", core.SemVer, "Commit ID", core.CommitSha32, "Commit SHA", string(core.CommitTime.Format(time.RFC1123)))
	log.Debugf("Go Version: %s", osruntime.Version())
	log.Debugf("Go OS/Arch: %s/%s", osruntime.GOOS, osruntime.GOARCH)
}

var (
	isOpenShift = func() (bool, error) {
		return k8sClient.IsOpenShift()
	}

	getKubeAPIServerVersion = func() (*version.Info, error) {
		return k8sClient.GetKubeAPIServerVersion()
	}

	getConfigDir = func() string {
		return ConfigDir
	}

	getk8sPathFn = func(log *zap.SugaredLogger, kubeVersion string, currentVersion, minVersion, maxVersion float64) string {
		return getk8sPath(log, kubeVersion, currentVersion, minVersion, maxVersion)
	}

	getK8sMinimumSupportedVersion = func() string {
		return K8sMaximumSupportedVersion
	}

	getK8sMaximumSupportedVersion = func() string {
		return K8sMaximumSupportedVersion
	}

	yamlUnmarshal = func(data []byte, v interface{}, opts ...yaml.JSONOpt) error {
		return yaml.Unmarshal(data, v, opts...)
	}
)

func getOperatorConfig(log *zap.SugaredLogger) (utils.OperatorConfig, error) {
	cfg := utils.OperatorConfig{}

	isOpenShift, err := isOpenShift()
	if err != nil {
		log.Info(fmt.Sprintf("isOpenShift err %t", isOpenShift))
	}
	cfg.IsOpenShift = isOpenShift
	if isOpenShift {
		log.Infof("Openshift environment")
	} else {
		log.Infof("Kubernetes environment")
	}
	kubeAPIServerVersion, err := getKubeAPIServerVersion()
	if err != nil {
		log.Info(fmt.Sprintf("kubeVersion err %s", kubeAPIServerVersion))
	}
	// format the required k8s version
	majorVersion := kubeAPIServerVersion.Major
	minorVersion := strings.TrimSuffix(kubeAPIServerVersion.Minor, "+")
	kubeVersion := fmt.Sprintf("%s.%s", majorVersion, minorVersion)

	minVersion := 0.0
	maxVersion := 0.0

	minVersion, err = strconv.ParseFloat(getK8sMinimumSupportedVersion(), 64)
	if err != nil {
		log.Info(fmt.Sprintf("minVersion %s", getK8sMinimumSupportedVersion()))
	}
	maxVersion, err = strconv.ParseFloat(getK8sMaximumSupportedVersion(), 64)
	if err != nil {
		log.Info(fmt.Sprintf("maxVersion %s", getK8sMaximumSupportedVersion()))
	}

	currentVersion, err := strconv.ParseFloat(kubeVersion, 64)
	if err != nil {
		log.Infof("currentVersion is %s", kubeVersion)
	}

	k8sPath := getk8sPathFn(log, kubeVersion, currentVersion, minVersion, maxVersion)

	_, err = os.ReadDir(filepath.Clean(getConfigDir()))
	if err != nil {
		log.Errorw(err.Error(), "cannot find driver config path", getConfigDir())
		cfg.ConfigDirectory = Operatorconfig
		log.Infof("Use ConfigDirectory %s", cfg.ConfigDirectory)
		k8sPath = fmt.Sprintf("%s%s", Operatorconfig, k8sPath)
	} else {
		cfg.ConfigDirectory = filepath.Clean(getConfigDir())
		log.Infof("Use ConfigDirectory %s", cfg.ConfigDirectory)
		k8sPath = fmt.Sprintf("%s%s", getConfigDir(), k8sPath)
	}

	buf, err := os.ReadFile(filepath.Clean(k8sPath))
	if err != nil {
		log.Info(fmt.Sprintf("reading file, %s, from the configmap mount: %v", k8sPath, err))
	}

	var imageConfig utils.K8sImagesConfig
	err = yamlUnmarshal(buf, &imageConfig)
	if err != nil {
		return cfg, fmt.Errorf("unmarshalling: %v", err)
	}

	cfg.K8sVersion = imageConfig

	return cfg, nil
}

func getk8sPath(log *zap.SugaredLogger, kubeVersion string, currentVersion, minVersion, maxVersion float64) string {
	k8sPath := ""
	if currentVersion < minVersion {
		log.Infof("Installed k8s version %s is less than the minimum supported k8s version %s , hence using the default configurations", kubeVersion, K8sMinimumSupportedVersion)
		k8sPath = "/driverconfig/common/default.yaml"
	} else if currentVersion > maxVersion {
		log.Infof("Installed k8s version %s is greater than the maximum supported k8s version %s , hence using the latest available configurations", kubeVersion, K8sMaximumSupportedVersion)
		k8sPath = fmt.Sprintf("/driverconfig/common/k8s-%s-values.yaml", K8sMaximumSupportedVersion)
	} else {
		k8sPath = fmt.Sprintf("/driverconfig/common/k8s-%s-values.yaml", kubeVersion)
		log.Infof("Current kubernetes version is %s which is a supported version ", kubeVersion)
	}
	return k8sPath
}

var (
	getConfigOrDie = func() *rest.Config {
		return ctrl.GetConfigOrDie()
	}

	newManager = func(config *rest.Config, options manager.Options) (manager.Manager, error) {
		return ctrl.NewManager(config, options)
	}

	newConfigOrDie = func(c *rest.Config) *kubernetes.Clientset {
		return kubernetes.NewForConfigOrDie(c)
	}

	getSetupWithManagerFn = func(r *controllers.ContainerStorageModuleReconciler) func(mgr ctrl.Manager, limiter workqueue.TypedRateLimiter[reconcile.Request], maxReconcilers int) error {
		return r.SetupWithManager
	}

	osExit = func(code int) {
		os.Exit(code)
	}

	initFlags = func() crzap.Options {
		flags.metricsBindAddress = flag.String("metrics-bind-address", ":8082", "The address the metric endpoint binds to.")
		flags.healthProbeBindAddress = flag.String("health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
		flags.leaderElect = flag.Bool("leader-elect", false,
			"Enable leader election for controller manager. "+
				"Enabling this will ensure there is only one active controller manager.")
		opts := initZapFlags()
		flag.Parse()
		return opts
	}

	initZapFlags = func() crzap.Options {
		opts := crzap.Options{
			Development: true,
		}
		opts.BindFlags(flag.CommandLine)
		return opts
	}

	getControllerWatchCh = func() chan (struct{}) {
		return controllers.StopWatch
	}

	setupSignalHandler = func() context.Context {
		return ctrl.SetupSignalHandler()
	}
)

var flags struct {
	metricsBindAddress     *string
	healthProbeBindAddress *string
	leaderElect            *bool
	zapOpts                crzap.Options
}

func main() {
	opts := initFlags()

	_, log := logger.GetNewContextWithLogger("main")

	ctrl.SetLogger(crzap.New(crzap.UseFlagOptions(&opts)))

	printVersion(log)
	operatorConfig, err := getOperatorConfig(log)
	if err != nil {
		setupLog.Error(err, "unable to get operator config")
		osExit(1)
		return
	}
	restConfig := getConfigOrDie()

	mgr, err := newManager(restConfig, ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: *flags.metricsBindAddress},
		HealthProbeBindAddress: *flags.healthProbeBindAddress,
		LeaderElection:         *flags.leaderElect,
		LeaderElectionID:       "090cae6a.dell.com",
		WebhookServer: webhook.NewServer(webhook.Options{ // Corrected webhook initialization
			Port: 9443,
		}),
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		osExit(1)
		return
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Infof)
	k8sClient := newConfigOrDie(restConfig)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: k8sClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(clientgoscheme.Scheme, corev1.EventSource{Component: "csm"})

	expRateLimiter := workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](5*time.Millisecond, 120*time.Second)

	r := &controllers.ContainerStorageModuleReconciler{
		Client:        mgr.GetClient(),
		K8sClient:     k8sClient,
		Log:           log,
		Scheme:        mgr.GetScheme(),
		EventRecorder: recorder,
		Config:        operatorConfig,
	}

	setupWithManager := getSetupWithManagerFn(r)
	if err := setupWithManager(mgr, expRateLimiter, 1); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ContainerStorageModule")
		osExit(1)
		return
	}
	defer close(getControllerWatchCh())
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		osExit(1)
		return
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		osExit(1)
		return
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(setupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		osExit(1)
	}
}
