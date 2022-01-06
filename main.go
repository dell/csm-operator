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
	"io/ioutil"
	"os"
	osruntime "runtime"
	"strconv"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/yaml"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/controllers"
	"github.com/dell/csm-operator/core"
	k8sClient "github.com/dell/csm-operator/k8s"
	utils "github.com/dell/csm-operator/pkg/utils"
	"github.com/kubernetes-csi/external-snapshotter/client/v3/apis/volumesnapshot/v1beta1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	//+kubebuilder:scaffold:imports
)

const (
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

	utilruntime.Must(v1beta1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

var log = logf.Log.WithName("cmd")

func printVersion() {
	log.Info("Operator Version", "Version", core.SemVer, "Commit ID", core.CommitSha32, "Commit SHA", string(core.CommitTime.Format(time.RFC1123)))
	log.Info(fmt.Sprintf("Go Version: %s", osruntime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", osruntime.GOOS, osruntime.GOARCH))
	//log.Info(fmt.Sprintf("Version of operator-sdk: %v", sdkVersion.Version))
}

func getOperatorConfig() utils.OperatorConfig {
	cfg := utils.OperatorConfig{}

	isOpenShift, err := k8sClient.IsOpenShift()
	if err != nil {
		panic(err.Error())
	}
	cfg.IsOpenShift = isOpenShift

	kubeVersion, err := k8sClient.GetVersion()
	if err != nil {
		panic(err.Error())
	}
	minVersion := 0.0
	maxVersion := 0.0
	if !isOpenShift {
		minVersion, err = strconv.ParseFloat(K8sMinimumSupportedVersion, 64)
		if err != nil {
			panic(err.Error())
		}
		maxVersion, err = strconv.ParseFloat(K8sMaximumSupportedVersion, 64)
		if err != nil {
			panic(err.Error())
		}
	} else {
		minVersion, err = strconv.ParseFloat(OpenshiftMinimumSupportedVersion, 64)
		if err != nil {
			panic(err.Error())
		}
		maxVersion, err = strconv.ParseFloat(OpenshiftMaximumSupportedVersion, 64)
		if err != nil {
			panic(err.Error())
		}
	}
	currentVersion, err := strconv.ParseFloat(kubeVersion, 64)
	if err != nil {
		panic(err.Error())
	}
	if currentVersion < minVersion {
		panic(fmt.Sprintf("version %s is less than minimum supported version of %f", kubeVersion, minVersion))
	}
	if currentVersion > maxVersion {
		panic(fmt.Sprintf("version %s is greater than maximum supported version of %f", kubeVersion, maxVersion))
	}

	// Get the environment variSable config dir
	configDir := os.Getenv("X_CSM_OPERATOR_CONFIG_DIR")
	if configDir == "" {
		// Set the config dir to the folder pkg/config
		configDir = "operatorconfig/"
	} else {
		k8sPath := fmt.Sprintf("%s/driverconfig/common/k8s-%s-values.yaml", configDir, kubeVersion)
		_, err := ioutil.ReadFile(k8sPath)
		if err != nil {
			// This means that the configmap is not mounted
			// fall back to the local copy
			log.Error(err, "Error reading file from the configmap mount")
			log.Info("Falling back to local copy of config files")

			configDir = "/etc/config/local/csm-operator"
		}
	}
	cfg.ConfigDirectory = configDir

	k8sPath := fmt.Sprintf("%s/driverconfig/common/k8s-%s-values.yaml", cfg.ConfigDirectory, kubeVersion)
	buf, err := ioutil.ReadFile(k8sPath)
	if err != nil {
		panic(fmt.Sprintf("reading file, %s, from the configmap mount: %v", k8sPath, err))
	}

	var imageConfig utils.K8sImagesConfig
	err = yaml.Unmarshal(buf, &imageConfig)
	if err != nil {
		panic(fmt.Sprintf("unmarshalling: %v", err))
	}

	cfg.K8sVersion = imageConfig

	return cfg
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	printVersion()
	operatorConfig := getOperatorConfig()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
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

	if err = (&controllers.ContainerStorageModuleReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("ContainerStorageModule"),
		Scheme: mgr.GetScheme(),
		Config: operatorConfig,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ContainerStorageModule")
		os.Exit(1)
	}
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
