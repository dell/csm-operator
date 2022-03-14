/*

Copyright (c) 2021 Dell Inc., or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package main

import (
	"flag"
	"fmt"
	"os"
	osruntime "runtime"
	"strconv"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	crzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	"github.com/dell/csm-operator/controllers"
	"github.com/dell/csm-operator/core"
	k8sClient "github.com/dell/csm-operator/k8s"
	"github.com/dell/csm-operator/pkg/logger"
	utils "github.com/dell/csm-operator/pkg/utils"
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
	K8sMinimumSupportedVersion = "1.21"
	// K8sMaximumSupportedVersion is the maximum supported version for k8s
	K8sMaximumSupportedVersion = "1.23"
	// OpenshiftMinimumSupportedVersion is the minimum supported version for openshift
	OpenshiftMinimumSupportedVersion = "4.8"
	// OpenshiftMaximumSupportedVersion is the maximum supported version for openshift
	OpenshiftMaximumSupportedVersion = "4.9"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(csmv1.AddToScheme(scheme))

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
		log.Info(fmt.Sprintf("isOpenShift err %t", isOpenShift))
	}
	cfg.IsOpenShift = isOpenShift

	kubeVersion, err := k8sClient.GetVersion()
	if err != nil {
		return cfg, err
	}

	log.Info(fmt.Sprintf("kubeVersion %s", kubeVersion))

	minVersion := 0.0
	maxVersion := 0.0
	if !isOpenShift {
		minVersion, err = strconv.ParseFloat(K8sMinimumSupportedVersion, 64)
		if err != nil {
			log.Info(fmt.Sprintf("minVersion %s", K8sMinimumSupportedVersion))
		}
		maxVersion, err = strconv.ParseFloat(K8sMaximumSupportedVersion, 64)
		if err != nil {
			log.Info(fmt.Sprintf("maxVersion %s", K8sMaximumSupportedVersion))
		}
	} else {
		minVersion, err = strconv.ParseFloat(OpenshiftMinimumSupportedVersion, 64)
		if err != nil {
			log.Info(fmt.Sprintf("minVersion %s", OpenshiftMinimumSupportedVersion))
		}
		maxVersion, err = strconv.ParseFloat(OpenshiftMaximumSupportedVersion, 64)
		if err != nil {
			log.Info(fmt.Sprintf("maxVersion  %s", OpenshiftMaximumSupportedVersion))
		}
	}
	currentVersion, err := strconv.ParseFloat(kubeVersion, 64)
	if err != nil {
		log.Infof("currentVersion is %s", kubeVersion)
	}
	// intialise variable
	k8sPath := ""
	if currentVersion < minVersion {
		log.Infof("Installed k8s version %s is less than the minimum supported k8s version %s , hence using the default configurations", kubeVersion, K8sMinimumSupportedVersion)
		k8sPath = "default.yaml"
	} else if currentVersion > maxVersion {
		log.Infof("Installed k8s version %s is greater than the maximum supported k8s version %s , hence using the latest available configurations", kubeVersion, K8sMaximumSupportedVersion)
		k8sPath = fmt.Sprintf("k8s-%s-values.yaml", K8sMaximumSupportedVersion)
	} else {
		k8sPath = fmt.Sprintf("k8s-%s-values.yaml", kubeVersion)
		log.Infof("Current kubernetes version is %s which is a supported version ", kubeVersion)
	}

	var imagecfg utils.K8sImagesConfig
	imagecfg.K8sVersion = k8sPath
	cfg.K8sSidecars = imagecfg

	log.Infof("kubernetes k8sPath %+v", cfg)
	return cfg, nil
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8082", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
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

	restConfig := ctrl.GetConfigOrDie()
	k8sClient := kubernetes.NewForConfigOrDie(restConfig)

	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "090cae6a.dell.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	printVersion(log)
	operatorConfig, err := getOperatorConfig(log)
	if err != nil {
		setupLog.Error(err, "unable to start manager pre-requisite configmap processing failed")
		os.Exit(1)
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: k8sClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(clientgoscheme.Scheme, corev1.EventSource{Component: "csm"})

	expRateLimiter := workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 120*time.Second)
	if err = (&controllers.ContainerStorageModuleReconciler{
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
	defer close(controllers.StopWatch)
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
