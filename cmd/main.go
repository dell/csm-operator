//  Copyright © 2021 - 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"crypto/tls"
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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	crzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/yaml"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/core"
	"github.com/dell/csm-operator/internal/controller"
	k8sClient "github.com/dell/csm-operator/k8s"
	"github.com/dell/csm-operator/pkg/logger"
	utils "github.com/dell/csm-operator/pkg/utils"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
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

func getOperatorConfig(log *zap.SugaredLogger) (utils.OperatorConfig, error) {
	cfg := utils.OperatorConfig{}

	isOpenShift, err := k8sClient.IsOpenShift()
	if err != nil {
		return cfg, fmt.Errorf("isOpenShift: %v", err)
	}
	cfg.IsOpenShift = isOpenShift
	if isOpenShift {
		log.Infof("Openshift environment")
	} else {
		log.Infof("Kubernetes environment")
	}
	kubeAPIServerVersion, err := k8sClient.GetKubeAPIServerVersion()
	if err != nil {
		return cfg, fmt.Errorf("kubeVersion: %v", err)
	}
	// format the required k8s version
	majorVersion := kubeAPIServerVersion.Major
	minorVersion := strings.TrimSuffix(kubeAPIServerVersion.Minor, "+")
	kubeVersion := fmt.Sprintf("%s.%s", majorVersion, minorVersion)

	minVersion := 0.0
	maxVersion := 0.0

	minVersion, err = strconv.ParseFloat(K8sMinimumSupportedVersion, 64)
	if err != nil {
		return cfg, fmt.Errorf("malformed minVersion: %v", err)
	}
	maxVersion, err = strconv.ParseFloat(K8sMaximumSupportedVersion, 64)
	if err != nil {
		return cfg, fmt.Errorf("malformed maxVersion: %v", err)
	}

	currentVersion, err := strconv.ParseFloat(kubeVersion, 64)
	if err != nil {
		return cfg, fmt.Errorf("malformed currentVersion: %s", err)
	}
	// intialise variable
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

	_, err = os.ReadDir(filepath.Clean(ConfigDir))
	if err != nil {
		log.Errorf("Read driver config dir %s: %v", ConfigDir, err)
		cfg.ConfigDirectory = Operatorconfig
		log.Infof("Use ConfigDirectory %s", cfg.ConfigDirectory)
		k8sPath = fmt.Sprintf("%s%s", Operatorconfig, k8sPath)
	} else {
		cfg.ConfigDirectory = filepath.Clean(ConfigDir)
		log.Infof("Use ConfigDirectory %s", cfg.ConfigDirectory)
		k8sPath = fmt.Sprintf("%s%s", ConfigDir, k8sPath)
	}

	buf, err := os.ReadFile(filepath.Clean(k8sPath))
	if err != nil {
		return cfg, fmt.Errorf("reading file %s, from the configmap mount: %v", k8sPath, err)
	}

	var imageConfig utils.K8sImagesConfig
	err = yaml.Unmarshal(buf, &imageConfig)
	if err != nil {
		return cfg, fmt.Errorf("unmarshalling imageConfig: %v", err)
	}

	cfg.K8sVersion = imageConfig

	return cfg, nil
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8082", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", false,
		"If set the metrics endpoint is served securely")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	opts := crzap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(crzap.New(crzap.UseFlagOptions(&opts)))

	logType := logger.DevelopmentLogLevel
	logger.SetLoggerLevel(logType)
	_, log := logger.GetNewContextWithLogger("main")

	ctrl.SetLogger(crzap.New(crzap.UseFlagOptions(&opts)))

	printVersion(log)
	operatorConfig, err := getOperatorConfig(log)
	if err != nil {
		setupLog.Error(err, "unable to get operator config")
		os.Exit(1)
	}
	restConfig := ctrl.GetConfigOrDie()

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	var tlsOpts []func(*tls.Config)
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: tlsOpts,
		Port:    9443,
	})

	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:   metricsAddr,
			SecureServing: secureMetrics,
			TLSOpts:       tlsOpts,
		},
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "090cae6a.dell.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Infof)
	k8sClient := kubernetes.NewForConfigOrDie(restConfig)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: k8sClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(clientgoscheme.Scheme, corev1.EventSource{Component: "csm"})

	expRateLimiter := workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](5*time.Millisecond, 120*time.Second)
	if err = (&controller.ContainerStorageModuleReconciler{
		Client:        mgr.GetClient(),
		K8sClient:     k8sClient,
		Log:           log,
		Scheme:        mgr.GetScheme(),
		EventRecorder: recorder,
		Config:        operatorConfig,
	}).SetupWithManager(mgr, expRateLimiter, 1); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ContainerStorageModule")
		os.Exit(1)
	}
	if err = (&controller.ApexConnectivityClientReconciler{
		Client:        mgr.GetClient(),
		K8sClient:     k8sClient,
		Log:           log,
		Scheme:        mgr.GetScheme(),
		EventRecorder: recorder,
		Config:        operatorConfig,
	}).SetupWithManager(mgr, expRateLimiter, 1); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ApexConnectivityClient")
		os.Exit(1)
	}
	defer close(controller.StopWatch)
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}