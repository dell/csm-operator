package utils

import (
	"bytes"
	"encoding/json"
	"hash/fnv"
	"io"
	"io/ioutil"

	"fmt"
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/go-logr/logr"
	goYAML "github.com/go-yaml/yaml"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	hashutil "k8s.io/kubernetes/pkg/util/hash"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"
)

// K8sImagesConfig -
type K8sImagesConfig struct {
	K8sVersion string `json:"kubeversion" yaml:"kubeversion"`
	Images     struct {
		Attacher    string `json:"attacher" yaml:"attacher"`
		Provisioner string `json:"provisioner" yaml:"provisioner"`
		Snapshotter string `json:"snapshotter" yaml:"snapshotter"`
		Registrar   string `json:"registrar" yaml:"registrar"`
		Resizer     string `json:"resizer" yaml:"resizer"`
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

// ControllerYAML -
type ControllerYAML struct {
	Deployment appsv1.Deployment
	Rbac       RbacYAML
}

// NodeYAML -
type NodeYAML struct {
	DaemonSet appsv1.DaemonSet
	Rbac      RbacYAML
}

const (
	// DefaultReleaseName constant
	DefaultReleaseName = "<DriverDefaultReleaseName>"
	// DefaultReleaseNamespace constant
	DefaultReleaseNamespace = "<DriverDefaultReleaseNamespace>"
	// DefaultImagePullPolicy constant
	DefaultImagePullPolicy = "IfNotPresent"
)

// SplitYAML divides a big bytes of yaml files in individual yaml files.
func SplitYAML(gaintYAML []byte) ([][]byte, error) {
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

// UpdateSideCar -
func UpdateSideCar(sideCars []csmv1.ContainerTemplate, c corev1.Container) corev1.Container {
	for _, side := range sideCars {
		if c.Name == side.Name {
			if side.Image != "" {
				c.Image = string(side.Image)
			}
			if side.ImagePullPolicy != "" {
				c.Image = string(side.ImagePullPolicy)
			}

			c.Env = ReplaceAllEnvs(c.Env, side.Envs)
			c.Args = ReplaceAllArgs(c.Args, side.Args)
		}
	}
	return c
}

// ReplaceALLContainerImage -
func ReplaceALLContainerImage(img K8sImagesConfig, c corev1.Container) corev1.Container {
	switch c.Name {
	case csmv1.Provisioner:
		c.Image = img.Images.Provisioner
	case csmv1.Attacher:
		c.Image = img.Images.Attacher
	case csmv1.Snapshotter:
		c.Image = img.Images.Snapshotter
	case csmv1.Registrar:
		c.Image = img.Images.Registrar
	case csmv1.Resizer:
		c.Image = img.Images.Resizer
	}
	return c
}

// ReplaceAllEnvs -
func ReplaceAllEnvs(defaultEnv, crEnv []corev1.EnvVar) []corev1.EnvVar {
	merge := []corev1.EnvVar{}
	for _, old := range crEnv {
		found := false
		for i, new := range defaultEnv {
			if old.Name == new.Name {
				defaultEnv[i].Value = old.Value
				found = true
			}
		}
		if !found {
			merge = append(merge, old)
		}
	}

	defaultEnv = append(defaultEnv, merge...)
	return defaultEnv
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
	return YamlString
}

// GetDriverYAML -
func GetDriverYAML(YamlString, kind string) (interface{}, error) {
	bufs, err := SplitYAML([]byte(YamlString))
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
		var dp appsv1.Deployment
		err := yaml.Unmarshal(podBuf, &dp)
		if err != nil {
			return nil, err
		}
		return ControllerYAML{
			Deployment: dp,
			Rbac:       rbac,
		}, nil
	} else if kind == "DaemonSet" {
		var ds appsv1.DaemonSet
		err := yaml.Unmarshal(podBuf, &ds)
		if err != nil {
			return nil, err
		}
		return NodeYAML{
			DaemonSet: ds,
			Rbac:      rbac,
		}, nil
	}

	return nil, fmt.Errorf("unsupported kind %s", kind)
}

// LogBannerAndReturn -
func LogBannerAndReturn(result reconcile.Result, err error, reqLogger logr.Logger) (reconcile.Result, error) {
	reqLogger.Info("################End Reconcile##############")
	return result, err
}

// HashContainerStorageModule returns the hash of the CSM specification
// This is used to detect if the CSM spec has changed and any updates are required
func HashContainerStorageModule(instance *csmv1.ContainerStorageModule) uint64 {
	hash := fnv.New32a()
	driverJSON, _ := json.Marshal(instance.GetContainerStorageModuleSpec())
	hashutil.DeepHashObject(hash, driverJSON)
	return uint64(hash.Sum32())
}

// CSMHashChanged returns hash diff bool
func CSMHashChanged(instance *csmv1.ContainerStorageModule) (uint64, uint64, bool) {
	expectedHash := HashContainerStorageModule(instance)
	return expectedHash, instance.GetCSMStatus().ContainerStorageModuleHash, instance.GetCSMStatus().ContainerStorageModuleHash != expectedHash
}

// GetModuleDefaultVersion -
func GetModuleDefaultVersion(driverConfigVersion string, driverType csmv1.DriverType, moduleType csmv1.ModuleType, path string) (string, error) {
	/* TODO(Michal): review with Team */
	configMapPath := fmt.Sprintf("%s/moduleconfig/common/version-values.yaml", path)
	buf, err := ioutil.ReadFile(configMapPath)
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
