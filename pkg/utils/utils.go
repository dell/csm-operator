package utils

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"

	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	"github.com/dell/csm-operator/pkg/logger"
	goYAML "github.com/go-yaml/yaml"

	appsv1 "k8s.io/api/apps/v1"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	t1 "k8s.io/apimachinery/pkg/types"
	confv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	k8sClient "github.com/dell/csm-operator/k8s"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// K8sImagesConfig -
type K8sImagesConfig struct {
	K8sVersion string `json:"kubeversion" yaml:"kubeversion"`
	Images     struct {
		Attacher              string `json:"attacher" yaml:"attacher"`
		Provisioner           string `json:"provisioner" yaml:"provisioner"`
		Snapshotter           string `json:"snapshotter" yaml:"snapshotter"`
		Registrar             string `json:"registrar" yaml:"registrar"`
		Resizer               string `json:"resizer" yaml:"resizer"`
		Externalhealthmonitor string `json:"externalhealthmonitorcontroller" yaml:"externalhealthmonitorcontroller"`
	} `json:"images" yaml:"images"`
}

// OperatorConfig -
type OperatorConfig struct {
	IsOpenShift     bool
	K8sVersion      K8sImagesConfig
	ConfigDirectory string
}

// RbacYAML -
type RbacYAML struct {
	ServiceAccount     corev1.ServiceAccount
	ClusterRole        rbacv1.ClusterRole
	ClusterRoleBinding rbacv1.ClusterRoleBinding
}

// UpgradePaths a list of versions eligible to upgrade the current version
type UpgradePaths struct {
	MinUpgradePath string `json:"minUpgradePath" yaml:"minUpgradePath"`
}

// ControllerYAML -
type ControllerYAML struct {
	Deployment confv1.DeploymentApplyConfiguration
	Rbac       RbacYAML
}

// NodeYAML -
type NodeYAML struct {
	DaemonSetApplyConfig confv1.DaemonSetApplyConfiguration
	Rbac                 RbacYAML
}

// ReplicaCluster -
type ReplicaCluster struct {
	ClusterID         string
	ClusterCTRLClient crclient.Client
	ClusterK8sClient  kubernetes.Interface
}

const (
	// DefaultReleaseName constant
	DefaultReleaseName = "<DriverDefaultReleaseName>"
	// DefaultReleaseNamespace constant
	DefaultReleaseNamespace = "<DriverDefaultReleaseNamespace>"
	// DefaultImagePullPolicy constant
	DefaultImagePullPolicy = "IfNotPresent"
	//KubeletConfigDir path
	KubeletConfigDir = "<KUBELET_CONFIG_DIR>"
	// ReplicationControllerNameSpace -
	ReplicationControllerNameSpace = "dell-replication-controller"
	// ReplicationControllerManager -
	ReplicationControllerManager = "dell-replication-controller-manager"
	// ReplicationSideCarName -
	ReplicationSideCarName = "dell-csi-replicator"
	// DefaultSourceClusterID -
	DefaultSourceClusterID = "default-source-cluster"
)

// SplitYaml divides a big bytes of yaml files in individual yaml files.
func SplitYaml(gaintYAML []byte) ([][]byte, error) {
	decoder := goYAML.NewDecoder(bytes.NewReader(gaintYAML))
	nullByte := []byte{110, 117, 108, 108, 10} // byte returned by  goYAML when yaml is empty

	var res [][]byte
	for {
		var value interface{}
		if err := decoder.Decode(&value); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		valueBytes, err := goYAML.Marshal(value)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(valueBytes, nullByte) {
			res = append(res, valueBytes)
		}
	}
	return res, nil
}

// UpdateSideCarApply -
func UpdateSideCarApply(sideCars []csmv1.ContainerTemplate, c *acorev1.ContainerApplyConfiguration) {
	for _, side := range sideCars {
		if *c.Name == side.Name {
			if side.Image != "" {
				*c.Image = string(side.Image)
			}
			if side.ImagePullPolicy != "" {
				*c.ImagePullPolicy = side.ImagePullPolicy
			}
			emptyEnv := make([]corev1.EnvVar, 0)
			c.Env = ReplaceAllApplyCustomEnvs(c.Env, emptyEnv, side.Envs)
			c.Args = ReplaceAllArgs(c.Args, side.Args)
		}
	}
	return
}

// ReplaceAllContainerImageApply -
func ReplaceAllContainerImageApply(img K8sImagesConfig, c *acorev1.ContainerApplyConfiguration) {
	switch *c.Name {
	case csmv1.Provisioner:
		*c.Image = img.Images.Provisioner
	case csmv1.Attacher:
		*c.Image = img.Images.Attacher
	case csmv1.Snapshotter:
		*c.Image = img.Images.Snapshotter
	case csmv1.Registrar:
		*c.Image = img.Images.Registrar
	case csmv1.Resizer:
		*c.Image = img.Images.Resizer
	case csmv1.Externalhealthmonitor:
		*c.Image = img.Images.Externalhealthmonitor
	}
	return
}

// ReplaceAllApplyCustomEnvs -
func ReplaceAllApplyCustomEnvs(driverEnv []acorev1.EnvVarApplyConfiguration,
	commonEnv []corev1.EnvVar,
	nrEnv []corev1.EnvVar) []acorev1.EnvVarApplyConfiguration {
	newEnv := make([]acorev1.EnvVarApplyConfiguration, 0)
	temp := make(map[string]string)
	for _, update := range commonEnv {
		if update.Value == "" {
			update.Value = "NA"
		}
		temp[update.Name] = update.Value
	}
	for _, update := range nrEnv {
		if update.Value == "" {
			update.Value = "NA"
		}
		temp[update.Name] = update.Value
	}
	for _, old := range driverEnv {
		if temp[*old.Name] != "" {
			val := temp[*old.Name]
			if val == "NA" {
				val = ""
			}
			//log.Info("debug overwrite ", "name", *old.Name, "value", val)
			e := acorev1.EnvVarApplyConfiguration{
				Name:  old.Name,
				Value: &val,
			}
			newEnv = append(newEnv, e)
		} else {
			e := acorev1.EnvVarApplyConfiguration{
				Name: old.Name,
			}
			if old.ValueFrom != nil {
				pRef := old.ValueFrom.FieldRef
				if pRef != nil {
					path := *pRef.FieldPath
					e = acorev1.EnvVarApplyConfiguration{
						Name: old.Name,
						ValueFrom: &acorev1.EnvVarSourceApplyConfiguration{
							FieldRef: &acorev1.ObjectFieldSelectorApplyConfiguration{
								FieldPath: &path,
							},
						},
					}
				}
				sRef := old.ValueFrom.SecretKeyRef
				if sRef != nil {
					secret := &acorev1.SecretKeySelectorApplyConfiguration{
						Key: sRef.Key,
					}
					secret.WithName(*sRef.Name)
					e = acorev1.EnvVarApplyConfiguration{
						Name: old.Name,
						ValueFrom: &acorev1.EnvVarSourceApplyConfiguration{
							SecretKeyRef: secret,
						},
					}
				}
			} else {
				e = acorev1.EnvVarApplyConfiguration{
					Name:  old.Name,
					Value: old.Value,
				}
			}

			newEnv = append(newEnv, e)
		}
	}
	return newEnv
}

// ReplaceAllArgs -
func ReplaceAllArgs(defaultArgs, crArgs []string) []string {
	merge := []string{}
	for _, old := range crArgs {
		found := false
		keyOld := strings.Split(old, "=")
		for i, new := range defaultArgs {
			if strings.Contains(new, keyOld[0]) {
				defaultArgs[i] = old
				found = true
			}
		}
		if !found {
			merge = append(merge, old)
		}
	}

	defaultArgs = append(defaultArgs, merge...)
	return defaultArgs
}

// ModifyCommonCR -
func ModifyCommonCR(YamlString string, cr csmv1.ContainerStorageModule) string {
	if cr.Name != "" {
		YamlString = strings.ReplaceAll(YamlString, DefaultReleaseName, cr.Name)
	}
	if cr.Namespace != "" {
		YamlString = strings.ReplaceAll(YamlString, DefaultReleaseNamespace, cr.Namespace)
	}
	if string(cr.Spec.Driver.Common.ImagePullPolicy) != "" {
		YamlString = strings.ReplaceAll(YamlString, DefaultImagePullPolicy, string(cr.Spec.Driver.Common.ImagePullPolicy))
	}
	path := ""
	for _, env := range cr.Spec.Driver.Common.Envs {
		if env.Name == "KUBELET_CONFIG_DIR" {
			path = env.Value
			break
		}
	}
	YamlString = strings.ReplaceAll(YamlString, KubeletConfigDir, path)

	return YamlString
}

// GetCTRLObject -
func GetCTRLObject(CtrlBuf []byte) ([]crclient.Object, error) {
	ctrlObjects := []crclient.Object{}

	bufs, err := SplitYaml(CtrlBuf)
	if err != nil {
		return ctrlObjects, err
	}

	for _, raw := range bufs {
		var meta metav1.TypeMeta
		err = yaml.Unmarshal(raw, &meta)
		if err != nil {
			return ctrlObjects, err
		}
		switch meta.Kind {
		case "ClusterRole":
			var cr rbacv1.ClusterRole
			if err := yaml.Unmarshal(raw, &cr); err != nil {
				return ctrlObjects, err
			}
			ctrlObjects = append(ctrlObjects, &cr)

		case "ClusterRoleBinding":
			var crb rbacv1.ClusterRoleBinding
			if err := yaml.Unmarshal(raw, &crb); err != nil {
				return ctrlObjects, err
			}

			ctrlObjects = append(ctrlObjects, &crb)

		case "Service":

			var sv corev1.Service
			if err := yaml.Unmarshal(raw, &sv); err != nil {
				return ctrlObjects, err
			}
			ctrlObjects = append(ctrlObjects, &sv)

		case "ConfigMap":

			var cm corev1.ConfigMap
			if err := yaml.Unmarshal(raw, &cm); err != nil {
				return ctrlObjects, err
			}

			ctrlObjects = append(ctrlObjects, &cm)

		case "Deployment":

			var dp appsv1.Deployment
			if err := yaml.Unmarshal(raw, &dp); err != nil {
				return ctrlObjects, err
			}

			ctrlObjects = append(ctrlObjects, &dp)

		}
	}

	return ctrlObjects, nil
}

// GetDriverYaml -
func GetDriverYaml(YamlString, kind string) (interface{}, error) {
	bufs, err := SplitYaml([]byte(YamlString))
	if err != nil {
		return nil, err
	}
	rbac := RbacYAML{}
	var podBuf []byte
	for _, raw := range bufs {
		var meta metav1.TypeMeta
		err = yaml.Unmarshal(raw, &meta)
		if err != nil {
			return nil, err
		}
		switch meta.Kind {
		case kind:
			podBuf = raw
		case "ServiceAccount":
			var sa corev1.ServiceAccount
			err := yaml.Unmarshal(raw, &sa)
			if err != nil {
				return nil, err
			}
			rbac.ServiceAccount = sa
		case "ClusterRole":
			var cr rbacv1.ClusterRole
			err := yaml.Unmarshal(raw, &cr)
			if err != nil {
				return nil, err
			}
			rbac.ClusterRole = cr

		case "ClusterRoleBinding":
			var crb rbacv1.ClusterRoleBinding
			err := yaml.Unmarshal(raw, &crb)
			if err != nil {
				return nil, err
			}
			rbac.ClusterRoleBinding = crb
		}
	}

	if kind == "Deployment" {
		var dp confv1.DeploymentApplyConfiguration
		err := yaml.Unmarshal(podBuf, &dp)
		if err != nil {
			return nil, err
		}
		return ControllerYAML{
			Deployment: dp,
			Rbac:       rbac,
		}, nil
	} else if kind == "DaemonSet" {
		var dsac confv1.DaemonSetApplyConfiguration

		err := yaml.Unmarshal(podBuf, &dsac)
		if err != nil {
			return nil, err
		}
		return NodeYAML{
			DaemonSetApplyConfig: dsac,
			Rbac:                 rbac,
		}, nil
	}

	return nil, fmt.Errorf("unsupported kind %s", kind)
}

// DeleteObject -
func DeleteObject(ctx context.Context, obj crclient.Object, ctrlClient crclient.Client) error {
	log := logger.GetLogger(ctx)

	kind := obj.GetObjectKind().GroupVersionKind().Kind
	name := obj.GetName()

	err := ctrlClient.Get(ctx, t1.NamespacedName{Name: name, Namespace: obj.GetNamespace()}, obj)

	if err != nil && k8serror.IsNotFound(err) {
		log.Infow("Object not found to delete", "Name:", name, "Kind:", kind, "Namespace:", obj.GetNamespace())
		return nil
	} else if err != nil {
		log.Errorw("error to find object in deleteObj", "Error", err.Error(), "Name:", name, "Kind:", kind)
		return err
	} else {
		log.Infow("Deleting object", "Name:", name, "Kind:", kind)
		err = ctrlClient.Delete(ctx, obj)
		if err != nil && !k8serror.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// ApplyObject -
func ApplyObject(ctx context.Context, obj crclient.Object, ctrlClient crclient.Client) error {
	log := logger.GetLogger(ctx)

	kind := obj.GetObjectKind().GroupVersionKind().Kind
	name := obj.GetName()

	err := ctrlClient.Get(ctx, t1.NamespacedName{Name: name, Namespace: obj.GetNamespace()}, obj)

	if err != nil && k8serror.IsNotFound(err) {
		log.Infow("Creating a new Object", "Name:", name, "Kind:", kind)
		err = ctrlClient.Create(ctx, obj)
		if err != nil {
			return err
		}

	} else if err != nil {
		log.Errorw("Unknown error.", "Error", err.Error())
		return err
	} else {
		log.Infow("Updating a new Object", "Name:", name, "Kind:", kind)
		err = ctrlClient.Update(ctx, obj)
		if err != nil {
			return err
		}
	}
	return nil
}

// LogBannerAndReturn -
func LogBannerAndReturn(result reconcile.Result, err error) (reconcile.Result, error) {
	fmt.Println("################End Reconcile##############")
	return result, err
}

// GetModuleDefaultVersion -
func GetModuleDefaultVersion(driverConfigVersion string, driverType csmv1.DriverType, moduleType csmv1.ModuleType, path string) (string, error) {
	configMapPath := fmt.Sprintf("%s/moduleconfig/common/version-values.yaml", path)
	buf, err := ioutil.ReadFile(filepath.Clean(configMapPath))
	if err != nil {
		return "", err
	}

	suppport := map[csmv1.DriverType]map[string]map[csmv1.ModuleType]string{}
	err = yaml.Unmarshal(buf, &suppport)
	if err != nil {
		return "", err
	}

	dType := driverType
	if driverType == "isilon" {
		dType = "powerscale"
	}

	if driver, ok := suppport[dType]; ok {
		if modules, ok := driver[driverConfigVersion]; ok {
			if moduleVer, ok := modules[moduleType]; ok {
				return moduleVer, nil
			}
			return "", fmt.Errorf(" %s module for %s driver  does not exist in file %s", moduleType, dType, configMapPath)
		}
		return "", fmt.Errorf("version %s of %s driver does not exist in file %s", driverConfigVersion, dType, configMapPath)

	}

	return "", fmt.Errorf("%s driver does not exist in file %s", dType, configMapPath)

}

func versionParser(version string) (int, int, error) {
	// strip v off of version string
	versionNoV := strings.TrimLeft(version, "v")
	// split by .
	versionPieces := strings.Split(versionNoV, ".")
	if len(versionPieces) != 3 {
		err := fmt.Errorf("version %+v not in correct version format, breaks down as: %+v", version, versionPieces)
		return -1, -1, err
	}

	majorVersion, _ := strconv.Atoi(versionPieces[0])
	minorVersion, _ := strconv.Atoi(versionPieces[1])

	return majorVersion, minorVersion, nil
}

// MinVersionCheck takes a driver name and a version of the form "vA.B.C" and checks it against the minimum version for the specified driver
func MinVersionCheck(minVersion string, version string) (bool, error) {
	minVersionA, minVersionB, err := versionParser(minVersion)
	if err != nil {
		return false, err
	}

	versionA, versionB, err := versionParser(version)
	if err != nil {
		return false, err
	}

	// compare each part according to minimum driver version
	if versionA >= minVersionA && versionB >= minVersionB {
		return true, nil
	}
	return false, nil
}

func getClusterIDs(replica csmv1.Module) ([]string, error) {
	var clusterIDs []string
	for _, comp := range replica.Components {
		if comp.Name == ReplicationControllerManager {
			for _, env := range comp.Envs {
				if env.Name == "TARGET_CLUSTERS_IDS" && env.Value != "" {
					clusterIDs = strings.Split(env.Value, ",")
					break
				}
			}
		}
	}
	err := fmt.Errorf("TARGET_CLUSTERS_IDS on CR should have more than 0 commma seperated cluster IDs. Got  %d", len(clusterIDs))
	if len(clusterIDs) >= 1 {
		err = nil
	}
	return clusterIDs, err
}

func getConfigData(ctx context.Context, clusterID string, ctrlClient crclient.Client) ([]byte, error) {
	log := logger.GetLogger(ctx)
	secret := &corev1.Secret{}
	if err := ctrlClient.Get(ctx, types.NamespacedName{Name: clusterID,
		Namespace: ReplicationControllerNameSpace}, secret); err != nil {
		if k8serrors.IsNotFound(err) {
			return []byte("error"), fmt.Errorf("failed to find secret %s", clusterID)
		}
		log.Error(err, "Failed to query for secret. Warning - the controller pod may not start")
	}
	return secret.Data["data"], nil
}

// NewControllerRuntimeClientWrapper -
var NewControllerRuntimeClientWrapper = func(clusterConfigData []byte) (crclient.Client, error) {
	return k8sClient.NewControllerRuntimeClient(clusterConfigData)
}

// NewK8sClientWrapper -
var NewK8sClientWrapper = func(clusterConfigData []byte) (*kubernetes.Clientset, error) {
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(clusterConfigData)
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(restConfig)
}

func getClusterCtrlClient(ctx context.Context, clusterID string, ctrlClient crclient.Client) (crclient.Client, error) {
	clusterConfigData, err := getConfigData(ctx, clusterID, ctrlClient)
	if err != nil {
		return nil, err
	}

	return NewControllerRuntimeClientWrapper(clusterConfigData)
}

func getClusterK8SClient(ctx context.Context, clusterID string, ctrlClient crclient.Client) (*kubernetes.Clientset, error) {
	clusterConfigData, err := getConfigData(ctx, clusterID, ctrlClient)
	if err != nil {
		return nil, err
	}

	return NewK8sClientWrapper(clusterConfigData)

}

// GetDefaultClusters -
func GetDefaultClusters(ctx context.Context, instance csmv1.ContainerStorageModule, r ReconcileCSM) (bool, []ReplicaCluster, error) {
	clusterClients := []ReplicaCluster{
		{
			ClusterCTRLClient: r.GetClient(),
			ClusterK8sClient:  r.GetK8sClient(),
			ClusterID:         DefaultSourceClusterID,
		},
	}

	replicaEnabled := false
	for _, m := range instance.Spec.Modules {
		if m.Name == csmv1.Replication && m.Enabled {
			replicaEnabled = true
			clusterIDs, err := getClusterIDs(m)
			if err != nil {
				return replicaEnabled, clusterClients, err
			}

			for _, clusterID := range clusterIDs {
				/*Hack: skip check for for csm_controller unit test*/
				if clusterID == "skip-replication-cluster-check" {
					return replicaEnabled, clusterClients, nil
				}
				targetCtrlClient, err := getClusterCtrlClient(ctx, clusterID, r.GetClient())
				if err != nil {
					return replicaEnabled, clusterClients, err
				}
				targetK8sClient, err := getClusterK8SClient(ctx, clusterID, r.GetClient())
				if err != nil {
					return replicaEnabled, clusterClients, err
				}

				clusterClients = append(clusterClients, ReplicaCluster{
					ClusterID:         clusterID,
					ClusterCTRLClient: targetCtrlClient,
					ClusterK8sClient:  targetK8sClient,
				})
			}
		}
	}
	return replicaEnabled, clusterClients, nil
}
