//  Copyright © 2021-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"crypto/tls"
	_ "embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	osruntime "runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	operatorutils "eos2git.cec.lab.emc.com/CSM/csm-operator/pkg/operatorutils"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	crzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	filters "sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/yaml"

	csmv1 "eos2git.cec.lab.emc.com/CSM/csm-operator/api/v1"
	"eos2git.cec.lab.emc.com/CSM/csm-operator/controllers"
	"eos2git.cec.lab.emc.com/CSM/csm-operator/core"
	k8sClient "eos2git.cec.lab.emc.com/CSM/csm-operator/k8s"
	"eos2git.cec.lab.emc.com/CSM/csm-operator/pkg/logger"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	//+kubebuilder:scaffold:imports
)

//go:embed samples/v2.17.0/storage_csm_powerflex_v2170.yaml
var powerflexSample []byte

//go:embed samples/v2.17.0/storage_csm_powermax_v2170.yaml
var powermaxSample []byte

//go:embed samples/v2.17.0/storage_csm_powerscale_v2170.yaml
var powerscaleSample []byte

//go:embed samples/v2.17.0/storage_csm_powerstore_v2170.yaml
var powerstoreSample []byte

//go:embed samples/v2.17.0/storage_csm_unity_v2170.yaml
var unitySample []byte

const (
	// ConfigDir path to driver deployment files
	ConfigDir = "/etc/config/dell-csm-operator"
	// Operatorconfig subfolder for deployment files
	Operatorconfig = "operatorconfig"
	// K8sMinimumSupportedVersion is the minimum supported version for k8s
	K8sMinimumSupportedVersion = "1.33"
	// K8sMaximumSupportedVersion is the maximum supported version for k8s
	K8sMaximumSupportedVersion = "1.35"
)

var (
	scheme         = runtime.NewScheme()
	setupLog       = ctrl.Log.WithName("setup")
	ManifestSemver = "dev"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(csmv1.AddToScheme(scheme))

	utilruntime.Must(apiextv1.AddToScheme(scheme))

	utilruntime.Must(certmanagerv1.AddToScheme(scheme))

	utilruntime.Must(gatewayv1.Install(scheme))

	//+kubebuilder:scaffold:scheme
}

func printVersion(log *zap.SugaredLogger) {
	log.Debugf("Operator Version: %s, Build Time: %s", ManifestSemver, core.CommitTime.Format(time.RFC1123))
	log.Debugf("Go Version: %s", osruntime.Version())
	log.Debugf("Go OS/Arch: %s/%s", osruntime.GOOS, osruntime.GOARCH)
}

var (
	isOpenShift = func(log *zap.SugaredLogger) (bool, error) {
		return k8sClient.IsOpenShift(log)
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
		return K8sMinimumSupportedVersion
	}

	getK8sMaximumSupportedVersion = func() string {
		return K8sMaximumSupportedVersion
	}

	yamlUnmarshal = func(data []byte, v interface{}, opts ...yaml.JSONOpt) error {
		return yaml.Unmarshal(data, v, opts...)
	}
)

func getOperatorConfig(log *zap.SugaredLogger) (operatorutils.OperatorConfig, error) {
	cfg := operatorutils.OperatorConfig{}

	isOpenShift, err := isOpenShift(log)
	if err != nil {
		log.Info(fmt.Sprintf("isOpenShift returned %v err %v", isOpenShift, err))
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

	var imageConfig operatorutils.K8sImagesConfig
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
		flags.metricsBindAddress = flag.String("metrics-bind-address", ":8443", "The address the metric endpoint binds to.")
		flags.healthProbeBindAddress = flag.String("health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
		flags.leaderElect = flag.Bool("leader-elect", false,
			"Enable leader election for controller manager. "+
				"Enabling this will ensure there is only one active controller manager.")
		flags.secureMetrics = flag.Bool("metrics-secure", true,
			"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
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

// getDefaultCSMSpecsFromSamples reads embedded sample YAML files and returns a map of
// DriverType to ContainerStorageModuleSpec, with images overridden by RELATED_IMAGE env vars
func getDefaultCSMSpecsFromSamples() (map[csmv1.DriverType]csmv1.ContainerStorageModuleSpec, error) {
	sampleFiles := map[string][]byte{
		"powerflex":  powerflexSample,
		"powermax":   powermaxSample,
		"powerscale": powerscaleSample,
		"powerstore": powerstoreSample,
		"unity":      unitySample,
	}

	driverImageMap := make(map[csmv1.DriverType]csmv1.ContainerStorageModuleSpec)

	for driverName, sampleData := range sampleFiles {
		var csm csmv1.ContainerStorageModule
		if err := yaml.Unmarshal(sampleData, &csm); err != nil {
			return nil, fmt.Errorf("failed to unmarshal sample for %s: %w", driverName, err)
		}

		driverType := csmv1.DriverType(driverName)
		driverImageMap[driverType] = csm.Spec
	}

	// Override images from environment variables
	applyRelatedImages(driverImageMap)

	return driverImageMap, nil
}

// applyRelatedImages overrides images in the specs with values from RELATED_IMAGE env vars
func applyRelatedImages(specs map[csmv1.DriverType]csmv1.ContainerStorageModuleSpec) {
	// Driver image mappings
	driverImageMappings := map[string]csmv1.DriverType{
		"RELATED_IMAGE_csi-vxflexos":   csmv1.PowerFlex,
		"RELATED_IMAGE_csi-powermax":   csmv1.PowerMax,
		"RELATED_IMAGE_csi-isilon":     csmv1.PowerScale,
		"RELATED_IMAGE_csi-unity":      csmv1.Unity,
		"RELATED_IMAGE_csi-powerstore": csmv1.PowerStore,
		"RELATED_IMAGE_cosi":           csmv1.Cosi,
	}

	// Populate driver images from env
	for envName, driverType := range driverImageMappings {
		imageValue := os.Getenv(envName)
		if imageValue != "" && specs[driverType].Driver.Common != nil {
			specs[driverType].Driver.Common.Image = csmv1.ImageType(imageValue)
		}
	}

	// CSI sidecar images (common to all drivers)
	sideCarMappings := map[string]string{
		"RELATED_IMAGE_attacher":                        csmv1.Attacher,
		"RELATED_IMAGE_provisioner":                     csmv1.Provisioner,
		"RELATED_IMAGE_snapshotter":                     csmv1.Snapshotter,
		"RELATED_IMAGE_registrar":                       csmv1.Registrar,
		"RELATED_IMAGE_resizer":                         csmv1.Resizer,
		"RELATED_IMAGE_externalhealthmonitorcontroller": csmv1.Externalhealthmonitor,
	}

	for envName, sideCarName := range sideCarMappings {
		imageValue := os.Getenv(envName)
		if imageValue != "" {
			for driverType := range specs {
				spec := specs[driverType]
				for i := range spec.Driver.SideCars {
					if spec.Driver.SideCars[i].Name == sideCarName {
						spec.Driver.SideCars[i].Image = csmv1.ImageType(imageValue)
					}
				}
				specs[driverType] = spec
			}
		}
	}

	// Driver-specific images
	driverSpecificMappings := map[string]map[csmv1.DriverType]string{
		"RELATED_IMAGE_sdc": {
			csmv1.PowerFlex: csmv1.Sdcmonitor,
		},
		"RELATED_IMAGE_csipowermax-reverseproxy": {
			csmv1.PowerMax: string(csmv1.ReverseProxyServer),
		},
	}

	for envName, driverTypeMap := range driverSpecificMappings {
		imageValue := os.Getenv(envName)
		if imageValue != "" {
			for driverType, componentName := range driverTypeMap {
				if spec, exists := specs[driverType]; exists {
					for i := range spec.Driver.SideCars {
						if spec.Driver.SideCars[i].Name == componentName {
							spec.Driver.SideCars[i].Image = csmv1.ImageType(imageValue)
						}
					}
					specs[driverType] = spec
				}
			}
		}
	}

	// Metrics images (driver-specific)
	metricsMappings := map[string]map[csmv1.DriverType]string{
		"RELATED_IMAGE_metrics-powerscale": {
			csmv1.PowerScale: "metrics",
		},
		"RELATED_IMAGE_metrics-powermax": {
			csmv1.PowerMax: "metrics",
		},
		"RELATED_IMAGE_metrics-powerflex": {
			csmv1.PowerFlex: "metrics",
		},
		"RELATED_IMAGE_metrics-powerstore": {
			csmv1.PowerStore: "metrics",
		},
	}

	for envName, driverTypeMap := range metricsMappings {
		imageValue := os.Getenv(envName)
		if imageValue != "" {
			for driverType, componentName := range driverTypeMap {
				if spec, exists := specs[driverType]; exists {
					// Find and update metrics component in observability module
					for i, module := range spec.Modules {
						if module.Name == csmv1.Observability {
							for j := range module.Components {
								if module.Components[j].Name == componentName {
									spec.Modules[i].Components[j].Image = csmv1.ImageType(imageValue)
								}
							}
						}
					}
					specs[driverType] = spec
				}
			}
		}
	}

	// Replication module images
	repMappings := map[string]string{
		"RELATED_IMAGE_dell-csi-replicator":                 "dell-csi-replicator",
		"RELATED_IMAGE_dell-replication-controller-manager": "dell-replication-controller-manager",
	}

	for envName, componentName := range repMappings {
		imageValue := os.Getenv(envName)
		if imageValue != "" {
			for driverType := range specs {
				spec := specs[driverType]
				for i, module := range spec.Modules {
					if module.Name == csmv1.Replication {
						for j := range module.Components {
							if module.Components[j].Name == componentName {
								spec.Modules[i].Components[j].Image = csmv1.ImageType(imageValue)
							}
						}
					}
				}
				specs[driverType] = spec
			}
		}
	}

	// Podmon module images
	podmonMappings := map[string]string{
		"RELATED_IMAGE_podmon-node": "podmon-node",
	}

	for envName, componentName := range podmonMappings {
		imageValue := os.Getenv(envName)
		if imageValue != "" {
			for driverType := range specs {
				spec := specs[driverType]
				for i, module := range spec.Modules {
					if module.Name == csmv1.PodMon {
						for j := range module.Components {
							if module.Components[j].Name == componentName {
								spec.Modules[i].Components[j].Image = csmv1.ImageType(imageValue)
							}
						}
					}
				}
				specs[driverType] = spec
			}
		}
	}

	// OTEL collector image
	if imageValue := os.Getenv("RELATED_IMAGE_otel-collector"); imageValue != "" {
		for driverType := range specs {
			spec := specs[driverType]
			for i, module := range spec.Modules {
				if module.Name == csmv1.Observability {
					for j := range module.Components {
						if module.Components[j].Name == string(csmv1.OtelCollector) {
							spec.Modules[i].Components[j].Image = csmv1.ImageType(imageValue)
						}
					}
				}
			}
			specs[driverType] = spec
		}
	}

	// Other images (init containers)
	otherMappings := map[string]string{
		"RELATED_IMAGE_metadataretriever":                 "metadataretriever",
		"RELATED_IMAGE_objectstorage-provisioner-sidecar": "objectstorage-provisioner",
	}

	for envName, componentName := range otherMappings {
		imageValue := os.Getenv(envName)
		if imageValue != "" {
			for driverType := range specs {
				spec := specs[driverType]
				for i := range spec.Driver.InitContainers {
					if spec.Driver.InitContainers[i].Name == componentName {
						spec.Driver.InitContainers[i].Image = csmv1.ImageType(imageValue)
					}
				}
				specs[driverType] = spec
			}
		}
	}

	// Authorization module images
	authMappings := map[string]string{
		"RELATED_IMAGE_csm-authorization-proxy":      "karavi-authorization-proxy",
		"RELATED_IMAGE_csm-authorization-tenant":     "karavi-authorization-tenant",
		"RELATED_IMAGE_csm-authorization-role":       "karavi-authorization-role",
		"RELATED_IMAGE_csm-authorization-storage":    "karavi-authorization-storage",
		"RELATED_IMAGE_csm-authorization-controller": "karavi-authorization-controller",
		"RELATED_IMAGE_redis-commander":              "redis-commander",
		"RELATED_IMAGE_opa":                          "opa",
		"RELATED_IMAGE_kube-mgmt":                    "opa-kube-mgmt",
	}

	for envName, componentName := range authMappings {
		imageValue := os.Getenv(envName)
		if imageValue != "" {
			for driverType := range specs {
				spec := specs[driverType]
				for i, module := range spec.Modules {
					if module.Name == csmv1.Authorization {
						for j := range module.Components {
							if module.Components[j].Name == componentName {
								spec.Modules[i].Components[j].Image = csmv1.ImageType(imageValue)
							}
						}
					}
				}
				specs[driverType] = spec
			}
		}
	}

	// NGINX proxy image as env var for authorization module
	if imageValue := os.Getenv("RELATED_IMAGE_nginx"); imageValue != "" {
		for driverType := range specs {
			if specs[driverType].Driver.Common != nil {
				specs[driverType].Driver.Common.Envs = append(specs[driverType].Driver.Common.Envs, corev1.EnvVar{
					Name:  "NGINX_PROXY_IMAGE",
					Value: imageValue,
				})
			}
		}
	}
}

var flags struct {
	metricsBindAddress     *string
	healthProbeBindAddress *string
	leaderElect            *bool
	secureMetrics          *bool
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

	var tlsOpts []func(*tls.Config)
	disableHTTP2 := func(c *tls.Config) {
		c.NextProtos = []string{"http/1.1"}
	}
	tlsOpts = append(tlsOpts, disableHTTP2)

	mgr, err := newManager(restConfig, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:    *flags.metricsBindAddress,
			SecureServing:  *flags.secureMetrics,
			TLSOpts:        tlsOpts,
			FilterProvider: filters.WithAuthenticationAndAuthorization,
		},
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

	defaultCSMSpecs, err := getDefaultCSMSpecsFromSamples()
	if err != nil {
		setupLog.Error(err, "unable to load default CSM specs from samples")
		osExit(1)
		return
	}

	r := &controllers.ContainerStorageModuleReconciler{
		Client:               mgr.GetClient(),
		K8sClient:            k8sClient,
		Log:                  log,
		Scheme:               mgr.GetScheme(),
		EventRecorder:        recorder,
		Config:               operatorConfig,
		ContentWatchChannels: make(map[string]chan struct{}),
		ContentWatchLock:     sync.Mutex{},
		DefaultCRs:           defaultCSMSpecs,
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
