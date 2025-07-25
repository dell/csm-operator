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

package operatorutils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/logger"
	goYAML "gopkg.in/yaml.v3"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	admissionregistration "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	t1 "k8s.io/apimachinery/pkg/types"
	confv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"sigs.k8s.io/yaml"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	k8sClient "github.com/dell/csm-operator/k8s"
)

// wrapper for UT to allow more coverage when testing
var yamlUnmarshal = func(data []byte, v interface{}) error {
	return yaml.Unmarshal(data, v)
}

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
		Sdc                   string `json:"sdc" yaml:"sdc"`
		Sdcmonitor            string `json:"sdcmonitor" yaml:"sdcmonitor"`
		Podmon                string `json:"podmon" yaml:"podmon"`
		CSIRevProxy           string `json:"csiReverseProxy" yaml:"csiReverseProxy"`
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
	Role               rbacv1.Role
	RoleBinding        rbacv1.RoleBinding
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

// StatefulControllerYAML -
type StatefulControllerYAML struct {
	StatefulSet confv1.StatefulSetApplyConfiguration
	Rbac        RbacYAML
}

// NodeYAML -
type NodeYAML struct {
	DaemonSetApplyConfig confv1.DaemonSetApplyConfiguration
	Rbac                 RbacYAML
}

// ClusterConfig -
type ClusterConfig struct {
	ClusterID         string
	ClusterCTRLClient crclient.Client
	ClusterK8sClient  kubernetes.Interface
}

// CSMComponentType - type constraint for DriverType and ModuleType
type CSMComponentType interface {
	csmv1.ModuleType | csmv1.DriverType
}

// LatestVersion - used in minimal manifests for CSM
type LatestVersion struct {
	Version string `yaml:"version"`
}

const (
	// DefaultReleaseName constant
	DefaultReleaseName = "<DriverDefaultReleaseName>"
	// DefaultReleaseNamespace constant
	DefaultReleaseNamespace = "<DriverDefaultReleaseNamespace>"
	// DefaultImagePullPolicy constant
	DefaultImagePullPolicy = "IfNotPresent"
	// KubeletConfigDir path
	KubeletConfigDir = "<KUBELET_CONFIG_DIR>"
	// ReplicationControllerNameSpace -
	ReplicationControllerNameSpace = "dell-replication-controller"
	// ReplicationControllerManager -
	ReplicationControllerManager = "dell-replication-controller-manager"
	// ReplicationControllerInit -
	ReplicationControllerInit = "dell-replication-controller-init"
	// ReplicationSideCarName -
	ReplicationSideCarName = "dell-csi-replicator"
	// ResiliencySideCarName -
	ResiliencySideCarName = "podmon"
	// DefaultSourceClusterID -
	DefaultSourceClusterID = "default-source-cluster"
	// AuthorizationNamespace - authorization
	AuthorizationNamespace = "authorization"
	// PodmonControllerComponent - podmon-controller
	PodmonControllerComponent = "podmon-controller"
	// PodmonNodeComponent - podmon-node
	PodmonNodeComponent = "podmon-node"
	// ApplicationMobilityNamespace - application-mobility
	ApplicationMobilityNamespace = "application-mobility"
	// ExistingNamespace - existing namespace
	ExistingNamespace = "<ExistingNameSpace>"
	// ClientNamespace - client namespace
	ClientNamespace = "<ClientNameSpace>"
	// BrownfieldManifest - brownfield-onboard.yaml
	BrownfieldManifest = "brownfield-onboard.yaml"
	// DefaultKubeletConfigDir - default kubelet config directory
	DefaultKubeletConfigDir = "/var/lib/kubelet"
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
	UpdateContainerApply(sideCars, c)
}

func UpdateContainerApply(toBeApplied []csmv1.ContainerTemplate, c *acorev1.ContainerApplyConfiguration) {
	for _, ctr := range toBeApplied {
		if *c.Name == ctr.Name {
			if ctr.Image != "" {
				*c.Image = string(ctr.Image)
			}
			if ctr.ImagePullPolicy != "" {
				*c.ImagePullPolicy = ctr.ImagePullPolicy
			}
			emptyEnv := make([]corev1.EnvVar, 0)
			c.Env = ReplaceAllApplyCustomEnvs(c.Env, emptyEnv, ctr.Envs)
			c.Args = ReplaceAllArgs(c.Args, ctr.Args)
		}
	}
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
	case csmv1.Sdc:
		*c.Image = img.Images.Sdcmonitor // driverconfig-default.yaml has only sdcmonitor entry which points to sdc image.
	case csmv1.Sdcmonitor:
		*c.Image = img.Images.Sdcmonitor
	case string(csmv1.Resiliency):
		*c.Image = img.Images.Podmon
	}
}

// UpdateInitContainerApply -
func UpdateInitContainerApply(initContainers []csmv1.ContainerTemplate, c *acorev1.ContainerApplyConfiguration) {
	UpdateContainerApply(initContainers, c)
}

// ReplaceAllApplyCustomEnvs -
func ReplaceAllApplyCustomEnvs(driverEnv []acorev1.EnvVarApplyConfiguration,
	commonEnv []corev1.EnvVar,
	nrEnv []corev1.EnvVar,
) []acorev1.EnvVarApplyConfiguration {
	newEnv := make([]acorev1.EnvVarApplyConfiguration, 0)
	temp := make(map[string]string)

	// get the name and value of the new env and store it in a map using name as key
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
		// Update the value for an existing name
		if temp[*old.Name] != "" {
			val := temp[*old.Name]
			if val == "NA" {
				val = ""
			}
			e := acorev1.EnvVarApplyConfiguration{
				Name:  old.Name,
				Value: &val,
			}
			newEnv = append(newEnv, e)
		} else {
			// if new config does not have a value for the existing name...
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
						Key:      sRef.Key,
						Optional: sRef.Optional,
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
	path := DefaultKubeletConfigDir
	if cr.Spec.Driver.Common != nil {
		if string(cr.Spec.Driver.Common.ImagePullPolicy) != "" {
			YamlString = strings.ReplaceAll(YamlString, DefaultImagePullPolicy, string(cr.Spec.Driver.Common.ImagePullPolicy))
		}
		for _, env := range cr.Spec.Driver.Common.Envs {
			if env.Name == "KUBELET_CONFIG_DIR" {
				path = env.Value
				break
			}
		}
	}
	YamlString = strings.ReplaceAll(YamlString, KubeletConfigDir, path)
	return YamlString
}

// GetCTRLObject - get controller object
func GetCTRLObject(CtrlBuf []byte) ([]crclient.Object, error) {
	ctrlObjects := []crclient.Object{}

	bufs, err := SplitYaml(CtrlBuf)
	if err != nil {
		return ctrlObjects, err
	}

	for _, raw := range bufs {
		var meta metav1.TypeMeta
		err = yamlUnmarshal(raw, &meta)
		if err != nil {
			return ctrlObjects, err
		}
		switch meta.Kind {
		case "ClusterRole":
			var cr rbacv1.ClusterRole
			if err := yamlUnmarshal(raw, &cr); err != nil {
				return ctrlObjects, err
			}
			ctrlObjects = append(ctrlObjects, &cr)

		case "ClusterRoleBinding":
			var crb rbacv1.ClusterRoleBinding
			if err := yamlUnmarshal(raw, &crb); err != nil {
				return ctrlObjects, err
			}

			ctrlObjects = append(ctrlObjects, &crb)

		case "Service":

			var sv corev1.Service
			if err := yamlUnmarshal(raw, &sv); err != nil {
				return ctrlObjects, err
			}
			ctrlObjects = append(ctrlObjects, &sv)

		case "ConfigMap":

			var cm corev1.ConfigMap
			if err := yamlUnmarshal(raw, &cm); err != nil {
				return ctrlObjects, err
			}

			ctrlObjects = append(ctrlObjects, &cm)

		case "Deployment":

			var dp appsv1.Deployment
			if err := yamlUnmarshal(raw, &dp); err != nil {
				return ctrlObjects, err
			}

			ctrlObjects = append(ctrlObjects, &dp)

		}
	}

	return ctrlObjects, nil
}

// GetModuleComponentObj - get module component object from config yaml string
func GetModuleComponentObj(CtrlBuf []byte) ([]crclient.Object, error) {
	ctrlObjects := []crclient.Object{}

	bufs, err := SplitYaml(CtrlBuf)
	if err != nil {
		return ctrlObjects, err
	}

	for _, raw := range bufs {
		var meta metav1.TypeMeta
		err = yamlUnmarshal(raw, &meta)
		if err != nil {
			return ctrlObjects, err
		}
		switch meta.Kind {

		case "CustomResourceDefinition":
			var crd apiextv1.CustomResourceDefinition
			err := yamlUnmarshal(raw, &crd)
			if err != nil {
				return ctrlObjects, err
			}
			ctrlObjects = append(ctrlObjects, &crd)

		case "ServiceAccount":
			var sa corev1.ServiceAccount
			err := yamlUnmarshal(raw, &sa)
			if err != nil {
				return ctrlObjects, err
			}
			ctrlObjects = append(ctrlObjects, &sa)

		case "ClusterRole":
			var cr rbacv1.ClusterRole
			if err := yamlUnmarshal(raw, &cr); err != nil {
				return ctrlObjects, err
			}
			ctrlObjects = append(ctrlObjects, &cr)

		case "ClusterRoleBinding":
			var crb rbacv1.ClusterRoleBinding
			if err := yamlUnmarshal(raw, &crb); err != nil {
				return ctrlObjects, err
			}

			ctrlObjects = append(ctrlObjects, &crb)

		case "Role":
			var r rbacv1.Role
			if err := yamlUnmarshal(raw, &r); err != nil {
				return ctrlObjects, err
			}
			ctrlObjects = append(ctrlObjects, &r)

		case "RoleBinding":
			var rb rbacv1.RoleBinding
			if err := yamlUnmarshal(raw, &rb); err != nil {
				return ctrlObjects, err
			}

			ctrlObjects = append(ctrlObjects, &rb)

		case "Service":

			var sv corev1.Service
			if err := yamlUnmarshal(raw, &sv); err != nil {
				return ctrlObjects, err
			}
			ctrlObjects = append(ctrlObjects, &sv)

		case "PersistentVolumeClaim":
			var pvc corev1.PersistentVolumeClaim
			err := yamlUnmarshal(raw, &pvc)
			if err != nil {
				return ctrlObjects, err
			}
			ctrlObjects = append(ctrlObjects, &pvc)

		case "Job":
			var j batchv1.Job
			err := yamlUnmarshal(raw, &j)
			if err != nil {
				return ctrlObjects, err
			}
			ctrlObjects = append(ctrlObjects, &j)

		case "IngressClass":
			var ic networking.IngressClass
			err := yamlUnmarshal(raw, &ic)
			if err != nil {
				return ctrlObjects, err
			}
			ctrlObjects = append(ctrlObjects, &ic)

		case "Ingress":
			var i networking.Ingress
			err := yamlUnmarshal(raw, &i)
			if err != nil {
				return ctrlObjects, err
			}
			ctrlObjects = append(ctrlObjects, &i)

		case "ValidatingWebhookConfiguration":
			var vwc admissionregistration.ValidatingWebhookConfiguration
			err := yamlUnmarshal(raw, &vwc)
			if err != nil {
				return ctrlObjects, err
			}
			ctrlObjects = append(ctrlObjects, &vwc)

		case "MutatingWebhookConfiguration":
			var mwc admissionregistration.MutatingWebhookConfiguration
			err := yamlUnmarshal(raw, &mwc)
			if err != nil {
				return ctrlObjects, err
			}
			ctrlObjects = append(ctrlObjects, &mwc)

		case "ConfigMap":
			var cm corev1.ConfigMap
			if err := yamlUnmarshal(raw, &cm); err != nil {
				return ctrlObjects, err
			}

			ctrlObjects = append(ctrlObjects, &cm)

		case "Secret":
			var s corev1.Secret
			if err := yamlUnmarshal(raw, &s); err != nil {
				return ctrlObjects, err
			}

			ctrlObjects = append(ctrlObjects, &s)

		case "Deployment":
			var dp appsv1.Deployment
			if err := yamlUnmarshal(raw, &dp); err != nil {
				return ctrlObjects, err
			}

			ctrlObjects = append(ctrlObjects, &dp)

		case "DaemonSet":
			var ds appsv1.DaemonSet
			if err := yamlUnmarshal(raw, &ds); err != nil {
				return ctrlObjects, err
			}

			ctrlObjects = append(ctrlObjects, &ds)

		case "BackupStorageLocation":
			var bsl velerov1.BackupStorageLocation
			if err := yamlUnmarshal(raw, &bsl); err != nil {
				return ctrlObjects, err
			}

			ctrlObjects = append(ctrlObjects, &bsl)

		case "VolumeSnapshotLocation":
			var vs velerov1.VolumeSnapshotLocation
			if err := yamlUnmarshal(raw, &vs); err != nil {
				return ctrlObjects, err
			}

			ctrlObjects = append(ctrlObjects, &vs)

		case "Issuer":
			var is certmanagerv1.Issuer
			if err := yamlUnmarshal(raw, &is); err != nil {
				return ctrlObjects, err
			}

			ctrlObjects = append(ctrlObjects, &is)

		case "Certificate":
			var ct certmanagerv1.Certificate
			if err := yamlUnmarshal(raw, &ct); err != nil {
				return ctrlObjects, err
			}

			ctrlObjects = append(ctrlObjects, &ct)

		case "StatefulSet":
			var ss appsv1.StatefulSet
			if err := yamlUnmarshal(raw, &ss); err != nil {
				return ctrlObjects, err
			}

			ctrlObjects = append(ctrlObjects, &ss)

		case "StorageClass":
			var sc storagev1.StorageClass
			if err := yamlUnmarshal(raw, &sc); err != nil {
				return ctrlObjects, err
			}

			ctrlObjects = append(ctrlObjects, &sc)

		case "PersistentVolume":
			var pv corev1.PersistentVolume
			if err := yamlUnmarshal(raw, &pv); err != nil {
				return ctrlObjects, err
			}

			ctrlObjects = append(ctrlObjects, &pv)

		case "Namespace":
			var ss corev1.Namespace
			if err := yamlUnmarshal(raw, &ss); err != nil {
				return ctrlObjects, err
			}

			ctrlObjects = append(ctrlObjects, &ss)
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
		err = yamlUnmarshal(raw, &meta)
		if err != nil {
			return nil, err
		}
		switch meta.Kind {
		case kind:
			podBuf = raw
		case "ServiceAccount":
			var sa corev1.ServiceAccount
			err := yamlUnmarshal(raw, &sa)
			if err != nil {
				return nil, err
			}
			rbac.ServiceAccount = sa
		case "ClusterRole":
			var cr rbacv1.ClusterRole
			err := yamlUnmarshal(raw, &cr)
			if err != nil {
				return nil, err
			}
			rbac.ClusterRole = cr

		case "ClusterRoleBinding":
			var crb rbacv1.ClusterRoleBinding
			err := yamlUnmarshal(raw, &crb)
			if err != nil {
				return nil, err
			}
			rbac.ClusterRoleBinding = crb

		case "Role":
			var crole rbacv1.Role
			err := yaml.Unmarshal(raw, &crole)
			if err != nil {
				return nil, err
			}
			rbac.Role = crole

		case "RoleBinding":
			var rb rbacv1.RoleBinding
			err := yaml.Unmarshal(raw, &rb)
			if err != nil {
				return nil, err
			}
			rbac.RoleBinding = rb

		}
	}

	if kind == "Deployment" {
		var dp confv1.DeploymentApplyConfiguration
		err := yamlUnmarshal(podBuf, &dp)
		if err != nil {
			return nil, err
		}
		return ControllerYAML{
			Deployment: dp,
			Rbac:       rbac,
		}, nil
	} else if kind == "DaemonSet" {
		var dsac confv1.DaemonSetApplyConfiguration

		err := yamlUnmarshal(podBuf, &dsac)
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
	}

	log.Infow("Deleting object", "Name:", name, "Kind:", kind)
	err = ctrlClient.Delete(ctx, obj)
	if err != nil && !k8serror.IsNotFound(err) {
		return err
	}
	return nil
}

// ApplyObject -
func ApplyObject(ctx context.Context, obj crclient.Object, ctrlClient crclient.Client) error {
	log := logger.GetLogger(ctx)

	kind := obj.GetObjectKind().GroupVersionKind().Kind
	name := obj.GetName()

	k8sObj := obj.DeepCopyObject().(crclient.Object)
	err := ctrlClient.Get(ctx, t1.NamespacedName{Name: name, Namespace: obj.GetNamespace()}, k8sObj)

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
		// Copy data/changes from obj to k8s object that already exists on the cluster
		if jsonBytes, err := json.Marshal(obj); err == nil {
			if err := json.Unmarshal(jsonBytes, &k8sObj); err == nil {
				obj = k8sObj
			}
		}
		err = ctrlClient.Update(ctx, obj)
		if err != nil && k8serror.IsForbidden(err) || k8serror.IsInvalid(err) {
			log.Warnw("Object update failed", "Warning", err.Error())
		} else if err != nil {
			return err
		}
	}
	return nil
}

// ApplyCTRLObject -
// TODO: Refactor to make use of ApplyObject. There's no need for so much repeated code.
func ApplyCTRLObject(ctx context.Context, obj crclient.Object, ctrlClient crclient.Client) error {
	log := logger.GetLogger(ctx)

	tempObj := obj.DeepCopyObject().(crclient.Object)
	kind := tempObj.GetObjectKind().GroupVersionKind().Kind
	name := tempObj.GetName()

	err := ctrlClient.Get(ctx, t1.NamespacedName{Name: name, Namespace: tempObj.GetNamespace()}, tempObj)

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

// LogEndReconcile - Print the 'ending reconcile' message
func LogEndReconcile() {
	fmt.Println("################End Reconcile##############")
}

// GetModuleDefaultVersion -
func GetModuleDefaultVersion(driverConfigVersion string, driverType csmv1.DriverType, moduleType csmv1.ModuleType, path string) (string, error) {
	configMapPath := fmt.Sprintf("%s/moduleconfig/common/version-values.yaml", path)
	buf, err := os.ReadFile(filepath.Clean(configMapPath))
	if err != nil {
		return "", err
	}

	support := map[csmv1.DriverType]map[string]map[csmv1.ModuleType]string{}
	err = yamlUnmarshal(buf, &support)
	if err != nil {
		return "", err
	}

	dType := driverType
	if driverType == "isilon" {
		dType = "powerscale"
	}

	if driver, ok := support[dType]; ok {
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
	minMajorVersion, minMinorVersion, err := versionParser(minVersion)
	if err != nil {
		return false, err
	}

	majorVersion, minorVersion, err := versionParser(version)
	if err != nil {
		return false, err
	}

	// compare each part according to minimum driver version
	if majorVersion > minMajorVersion {
		return true, nil
	} else if majorVersion == minMajorVersion && minorVersion >= minMinorVersion {
		return true, nil
	}
	return false, nil
}

func getConfigData(ctx context.Context, clusterID string, ctrlClient crclient.Client) ([]byte, error) {
	log := logger.GetLogger(ctx)
	secret := &corev1.Secret{}
	if err := ctrlClient.Get(ctx, t1.NamespacedName{
		Name:      clusterID,
		Namespace: ReplicationControllerNameSpace,
	}, secret); err != nil {
		if k8serror.IsNotFound(err) {
			return []byte("error"), fmt.Errorf("failed to find secret %s in namespace %s", clusterID, ReplicationControllerNameSpace)
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

// IsResiliencyModuleEnabled - check if resiliency module is enabled or not
func IsResiliencyModuleEnabled(_ context.Context, instance csmv1.ContainerStorageModule, _ ReconcileCSM) bool {
	for _, m := range instance.Spec.Modules {
		if m.Name == csmv1.Resiliency && m.Enabled {
			return true
		}
	}
	return false
}

// GetCluster - returns the client and ID of the cluster the operator is running on
func GetCluster(_ context.Context, r ReconcileCSM) ClusterConfig {
	clusterClient := ClusterConfig{
		ClusterCTRLClient: r.GetClient(),
		ClusterK8sClient:  r.GetK8sClient(),
		ClusterID:         DefaultSourceClusterID,
	}
	return clusterClient
}

// GetSecret - check if the secret is present
func GetSecret(ctx context.Context, name, namespace string, ctrlClient crclient.Client) (*corev1.Secret, error) {
	found := &corev1.Secret{}
	err := ctrlClient.Get(ctx, t1.NamespacedName{Name: name, Namespace: namespace}, found)
	if err != nil && k8serror.IsNotFound(err) {
		return nil, fmt.Errorf("no secrets found or error: %v", err)
	}
	return found, nil
}

// GetVolumeSnapshotLocation - check if the Volume Snapshot Location is present
func GetVolumeSnapshotLocation(ctx context.Context, name, namespace string, ctrlClient crclient.Client) (*velerov1.VolumeSnapshotLocation, error) {
	snapshotLocation := &velerov1.VolumeSnapshotLocation{}
	err := ctrlClient.Get(ctx, t1.NamespacedName{Namespace: namespace, Name: name},
		snapshotLocation,
	)
	if err != nil {
		return nil, err
	}
	return snapshotLocation, nil
}

// GetBackupStorageLocation - check if the Backup Storage Location is present
func GetBackupStorageLocation(ctx context.Context, name, namespace string, ctrlClient crclient.Client) (*velerov1.BackupStorageLocation, error) {
	backupStorage := &velerov1.BackupStorageLocation{}
	err := ctrlClient.Get(ctx, t1.NamespacedName{Namespace: namespace, Name: name},
		backupStorage,
	)
	if err != nil {
		return nil, err
	}
	return backupStorage, nil
}

// IsModuleEnabled - check if the module is enabled
func IsModuleEnabled(_ context.Context, instance csmv1.ContainerStorageModule, mod csmv1.ModuleType) (bool, csmv1.Module) {
	for _, m := range instance.Spec.Modules {
		if m.Name == mod && m.Enabled {
			return true, m
		}
	}

	return false, csmv1.Module{}
}

// IsModuleComponentEnabled - check if module components are enabled
func IsModuleComponentEnabled(ctx context.Context, instance csmv1.ContainerStorageModule, mod csmv1.ModuleType, componentType string) bool {
	moduleEnabled, module := IsModuleEnabled(ctx, instance, mod)
	if !moduleEnabled {
		return false
	}

	for _, c := range module.Components {
		if c.Name == componentType && *c.Enabled {
			return true
		}
	}

	return false
}

// HasModuleComponent - check if module component is present
func HasModuleComponent(instance csmv1.ContainerStorageModule, mod csmv1.ModuleType, componentType string) bool {
	module := instance.GetModule(mod)

	for _, c := range module.Components {
		if c.Name == componentType {
			return true
		}
	}
	return false
}

// AddModuleComponent - add a module component in the cr
func AddModuleComponent(instance *csmv1.ContainerStorageModule, mod csmv1.ModuleType, component csmv1.ContainerTemplate) {
	for i := range instance.Spec.Modules {
		if instance.Spec.Modules[i].Name == mod {
			instance.Spec.Modules[i].Components = append(instance.Spec.Modules[i].Components, component)
		}
	}
}

// IsAppMobilityComponentEnabled - check if Application Mobility componenets are enabled
func IsAppMobilityComponentEnabled(ctx context.Context, instance csmv1.ContainerStorageModule, _ ReconcileCSM, mod csmv1.ModuleType, componentType string) bool {
	appMobilityEnabled, appmobility := IsModuleEnabled(ctx, instance, mod)
	if !appMobilityEnabled {
		return false
	}

	for _, c := range appmobility.Components {
		if c.Name == componentType && *c.Enabled {
			return true
		}
	}

	return false
}

// Contains - check if slice contains the specified string
func Contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}

// DetermineUnitTestRun will determine if this reconcile call is part of unit test run
func DetermineUnitTestRun(ctx context.Context) bool {
	log := logger.GetLogger(ctx)
	unitTestRun, boolErr := strconv.ParseBool(os.Getenv("UNIT_TEST"))
	if unitTestRun && boolErr == nil {
		log.Info("Running in unit tests mode")
	} else {
		unitTestRun = false
	}
	return unitTestRun
}

// IsValidUpgrade will check if upgrade of module/driver is allowed
func IsValidUpgrade[T CSMComponentType](ctx context.Context, oldVersion, newVersion string, csmComponentType T, operatorConfig OperatorConfig) (bool, error) {
	log := logger.GetLogger(ctx)

	// if versions are equal, it is a modification
	if oldVersion == newVersion {
		log.Infow("proceeding with modification of driver/module install")
		return true, nil
	}

	var minUpgradePath string
	var minDowngradePath string
	var err error
	var isUpgradeValid bool
	var isDowngradeValid bool

	log.Info("####oldVersion: ", oldVersion, " ###newVersion: ", newVersion)

	isUpgrade, _ := MinVersionCheck(oldVersion, newVersion)

	// if it is an upgrade
	if isUpgrade {
		log.Info("proceeding with valid upgrade of driver/module")
		minUpgradePath, err = getUpgradeInfo(ctx, operatorConfig, csmComponentType, newVersion)
		isUpgradeValid, _ = MinVersionCheck(minUpgradePath, oldVersion)
	} else {
		// if it is a downgrade
		log.Info("proceeding with valid downgrade of driver/module")
		minDowngradePath, err = getUpgradeInfo(ctx, operatorConfig, csmComponentType, oldVersion)
		isDowngradeValid, _ = MinVersionCheck(minDowngradePath, newVersion)
	}

	if err != nil {
		log.Infow("getUpgradeInfo not successful")
		return false, err
	}
	if isUpgradeValid || isDowngradeValid {
		log.Infof("proceeding with valid upgrade/downgrade of %s from version %s to version %s", csmComponentType, oldVersion, newVersion)
		return isUpgradeValid || isDowngradeValid, nil
	}

	log.Infof("not proceeding with invalid driver/module upgrade")
	return isUpgradeValid || isDowngradeValid, fmt.Errorf("upgrade/downgrade of %s from version %s to %s not valid", csmComponentType, oldVersion, newVersion)
}

func getUpgradeInfo[T CSMComponentType](ctx context.Context, operatorConfig OperatorConfig, csmCompType T, oldVersion string) (string, error) {
	log := logger.GetLogger(ctx)

	csmCompConfigDir := ""
	switch any(csmCompType).(type) {
	case csmv1.DriverType:
		csmCompConfigDir = "driverconfig"
	case csmv1.ModuleType:
		csmCompConfigDir = "moduleconfig"
	}

	upgradeInfoPath := fmt.Sprintf("%s/%s/%s/%s/upgrade-path.yaml", operatorConfig.ConfigDirectory, csmCompConfigDir, csmCompType, oldVersion)
	log.Debugw("getUpgradeInfo", "upgradeInfoPath", upgradeInfoPath)

	buf, err := os.ReadFile(filepath.Clean(upgradeInfoPath))
	if err != nil {
		log.Errorw("getUpgradeInfo failed", "Error", err.Error())
		return "", err
	}
	YamlString := string(buf)

	var upgradePath UpgradePaths
	err = yamlUnmarshal([]byte(YamlString), &upgradePath)
	if err != nil {
		log.Errorw("getUpgradeInfo yaml marshall failed", "Error", err.Error())
		return "", err
	}

	// Example return value: "v2.2.0"
	return upgradePath.MinUpgradePath, nil
}

// GetCSMNamespaces returns the list of namespaces in the cluster that currently contain a CSM object
func GetCSMNamespaces(ctx context.Context, ctrlClient crclient.Client) ([]string, error) {
	// Set to store unique namespaces
	namespaceMap := make(map[string]struct{})

	csmList := &csmv1.ContainerStorageModuleList{}

	if err := ctrlClient.List(ctx, csmList); err != nil {
		return nil, fmt.Errorf("error listing csm resources: %w", err)
	}
	for _, csmResource := range csmList.Items {
		namespaceMap[csmResource.Namespace] = struct{}{}
	}

	// Convert set to slice
	var namespaces []string
	for namespace := range namespaceMap {
		namespaces = append(namespaces, namespace)
	}

	return namespaces, nil
}

// LoadDefaultComponents loads the default module components into cr
func LoadDefaultComponents(ctx context.Context, cr *csmv1.ContainerStorageModule, op OperatorConfig) error {
	log := logger.GetLogger(ctx)
	modules := []csmv1.ModuleType{csmv1.Observability}
	for _, module := range modules {
		if !cr.HasModule(module) {
			continue
		}

		defaultComps, err := getDefaultComponents(cr.GetDriverType(), module, op)
		if err != nil {
			log.Errorf("failed to get default components for %s: %v", module, err)
			return fmt.Errorf("failed to get default components for %s: %v", module, err)
		}
		// only load default components if module is enabled
		moduleEnabled, _ := IsModuleEnabled(ctx, *cr, module)
		if moduleEnabled {
			for _, comp := range defaultComps {
				if !HasModuleComponent(*cr, module, comp.Name) {
					log.Infof("Adding default component %s for %s ", comp.Name, module)
					AddModuleComponent(cr, csmv1.Observability, comp)
				}
			}
		}
	}

	return nil
}

func getDefaultComponents(driverType csmv1.DriverType, module csmv1.ModuleType, op OperatorConfig) ([]csmv1.ContainerTemplate, error) {
	file := fmt.Sprintf("%s/moduleconfig/%s/default-components.yaml", op.ConfigDirectory, module)
	buf, err := os.ReadFile(filepath.Clean(file))
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %s", file, err.Error())
	}

	defaultCsm := new(csmv1.ContainerStorageModule)
	err = yamlUnmarshal(buf, &defaultCsm)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal default-components.yaml for %s: %s", module, err.Error())
	}

	defaultComps := defaultCsm.GetModule(module).Components
	if module == csmv1.Observability {
		if driverType == csmv1.PowerScale {
			driverType = csmv1.PowerScaleName
		}
		for i := range defaultComps {
			if strings.HasPrefix(defaultComps[i].Name, "metrics") {
				defaultComps[i].Name = strings.ReplaceAll(defaultComps[i].Name, "<CSI_DRIVER_TYPE>", string(driverType))
			}
		}
	}
	return defaultComps, nil
}

// SetContainerImage loops through objects to find deployment and set the image for container
func SetContainerImage(objects []crclient.Object, deploymentName, containerName, image string) {
	if len(objects) == 0 || len(deploymentName) == 0 || len(containerName) == 0 || len(image) == 0 {
		return
	}
	for _, object := range objects {
		deployment, ok := object.(*appsv1.Deployment)
		if !ok || !strings.EqualFold(deployment.Name, deploymentName) {
			continue
		}
		if len(deployment.Spec.Template.Spec.Containers) == 0 {
			break
		}
		for i := range deployment.Spec.Template.Spec.Containers {
			if strings.EqualFold(deployment.Spec.Template.Spec.Containers[i].Name, containerName) {
				deployment.Spec.Template.Spec.Containers[i].Image = image
			}
		}
	}
}
