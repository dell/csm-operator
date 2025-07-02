//  Copyright © 2022-2025 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package steps

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/constants"
	"github.com/dell/csm-operator/pkg/modules"
	operatorutils "github.com/dell/csm-operator/pkg/operatorutils"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/kubectl"
	fpod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	roleName           = "CSIGold"
	tenantName         = "PancakeGroup"
	certManagerVersion = "v1.11.0"
)

var (
	authString        = "karavi-authorization-proxy"
	operatorNamespace = "dell-csm-operator"
	quotaLimit        = "100000000"
	pflexSecretMap    = map[string]string{
		"REPLACE_USER": "PFLEX_USER", "REPLACE_PASS": "PFLEX_PASS", "REPLACE_SYSTEMID": "PFLEX_SYSTEMID", "REPLACE_ENDPOINT": "PFLEX_ENDPOINT", "REPLACE_MDM": "PFLEX_MDM", "REPLACE_POOL": "PFLEX_POOL", "REPLACE_NAS": "PFLEX_NAS", "REPLACE_SFTP_REPO_ADDRESS": "PFLEX_SFTP_REPO_ADDRESS", "REPLACE_SFTP_REPO_USER": "PFLEX_SFTP_REPO_USER",
		"REPLACE_ZONING_USER": "PFLEX_ZONING_USER", "REPLACE_ZONING_PASS": "PFLEX_ZONING_PASS", "REPLACE_ZONING_SYSTEMID": "PFLEX_ZONING_SYSTEMID", "REPLACE_ZONING_ENDPOINT": "PFLEX_ZONING_ENDPOINT", "REPLACE_ZONING_MDM": "PFLEX_ZONING_MDM", "REPLACE_ZONING_POOL": "PFLEX_ZONING_POOL", "REPLACE_ZONING_NAS": "PFLEX_ZONING_NAS",
	}
	pflexAuthSecretMap       = map[string]string{"REPLACE_USER": "PFLEX_USER", "REPLACE_PASS": "PFLEX_PASS", "REPLACE_SYSTEMID": "PFLEX_SYSTEMID", "REPLACE_ENDPOINT": "PFLEX_AUTH_ENDPOINT", "REPLACE_MDM": "PFLEX_MDM"}
	pscaleSecretMap          = map[string]string{"REPLACE_CLUSTERNAME": "PSCALE_CLUSTER", "REPLACE_USER": "PSCALE_USER", "REPLACE_PASS": "PSCALE_PASS", "REPLACE_ENDPOINT": "PSCALE_ENDPOINT", "REPLACE_PORT": "PSCALE_PORT", "REPLACE_MULTI_CLUSTERNAME": "PSCALE_MULTI_CLUSTER", "REPLACE_MULTI_USER": "PSCALE_MULTI_USER", "REPLACE_MULTI_PASS": "PSCALE_MULTI_PASS", "REPLACE_MULTI_ENDPOINT": "PSCALE_MULTI_ENDPOINT", "REPLACE_MULTI_PORT": "PSCALE_MULTI_PORT", "REPLACE_MULTI_AUTH_ENDPOINT": "PSCALE_MULTI_AUTH_ENDPOINT", "REPLACE_MULTI_AUTH_PORT": "PSCALE_MULTI_AUTH_PORT"}
	pscaleAuthSecretMap      = map[string]string{"REPLACE_CLUSTERNAME": "PSCALE_CLUSTER", "REPLACE_USER": "PSCALE_USER", "REPLACE_PASS": "PSCALE_PASS", "REPLACE_AUTH_ENDPOINT": "PSCALE_AUTH_ENDPOINT", "REPLACE_AUTH_PORT": "PSCALE_AUTH_PORT", "REPLACE_ENDPOINT": "PSCALE_ENDPOINT", "REPLACE_PORT": "PSCALE_PORT"}
	pscaleAuthSidecarMap     = map[string]string{"REPLACE_CLUSTERNAME": "PSCALE_CLUSTER", "REPLACE_ENDPOINT": "PSCALE_ENDPOINT", "REPLACE_AUTH_ENDPOINT": "PSCALE_AUTH_ENDPOINT", "REPLACE_AUTH_PORT": "PSCALE_AUTH_PORT", "REPLACE_PORT": "PSCALE_PORT"}
	pscaleEphemeralVolumeMap = map[string]string{"REPLACE_CLUSTERNAME": "PSCALE_CLUSTER", "REPLACE_ENDPOINT": "PSCALE_ENDPOINT"}
	pflexEphemeralVolumeMap  = map[string]string{"REPLACE_SYSTEMID": "PFLEX_SYSTEMID", "REPLACE_POOL": "PFLEX_POOL", "REPLACE_VOLUME": "PFLEX_VOLUME"}
	pflexAuthSidecarMap      = map[string]string{"REPLACE_USER": "PFLEX_USER", "REPLACE_PASS": "PFLEX_PASS", "REPLACE_SYSTEMID": "PFLEX_SYSTEMID", "REPLACE_ENDPOINT": "PFLEX_ENDPOINT", "REPLACE_AUTH_ENDPOINT": "PFLEX_AUTH_ENDPOINT"}
	pmaxCredMap              = map[string]string{"REPLACE_USER": "PMAX_USER_ENCODED", "REPLACE_PASS": "PMAX_PASS_ENCODED"}
	pmaxSecretMap            = map[string]string{
		"REPLACE_USERNAME": "PMAX_USER", "REPLACE_PASSWORD": "PMAX_PASS", "REPLACE_SYSTEMID": "PMAX_SYSTEMID", "REPLACE_ENDPOINT": "PMAX_ENDPOINT",
		"REPLACE_ZONING_USERNAME": "PMAX_ZONING_USER", "REPLACE_ZONING_PASSWORD": "PMAX_ZONING_PASS", "REPLACE_ZONING_SYSTEMID": "PMAX_ZONING_SYSTEMID", "REPLACE_ZONING_ENDPOINT": "PMAX_ZONING_ENDPOINT",
	}
	pmaxAuthSidecarMap     = map[string]string{"REPLACE_SYSTEMID": "PMAX_SYSTEMID", "REPLACE_ENDPOINT": "PMAX_ENDPOINT", "REPLACE_AUTH_ENDPOINT": "PMAX_AUTH_ENDPOINT"}
	pmaxStorageMap         = map[string]string{"REPLACE_SYSTEMID": "PMAX_SYSTEMID", "REPLACE_RESOURCE_POOL": "PMAX_POOL_V1", "REPLACE_SERVICE_LEVEL": "PMAX_SERVICE_LEVEL"}
	pmaxReverseProxyMap    = map[string]string{"REPLACE_SYSTEMID": "PMAX_SYSTEMID", "REPLACE_AUTH_ENDPOINT": "PMAX_AUTH_ENDPOINT"}
	authSidecarRootCertMap = map[string]string{}
	amConfigMap            = map[string]string{"REPLACE_ALT_BUCKET_NAME": "ALT_BUCKET_NAME", "REPLACE_BUCKET_NAME": "BUCKET_NAME", "REPLACE_S3URL": "BACKEND_STORAGE_URL", "REPLACE_CONTROLLER_IMAGE": "AM_CONTROLLER_IMAGE", "REPLACE_PLUGIN_IMAGE": "AM_PLUGIN_IMAGE"}
	pmaxArrayConfigMap     = map[string]string{"REPLACE_PORTGROUPS": "PMAX_PORTGROUPS", "REPLACE_PROTOCOL": "PMAX_PROTOCOL", "REPLACE_ARRAYS": "PMAX_ARRAYS", "REPLACE_ENDPOINT": "PMAX_ENDPOINT"}
	pmaxAuthArrayConfigMap = map[string]string{"REPLACE_PORTGROUPS": "PMAX_PORTGROUPS", "REPLACE_PROTOCOL": "PMAX_PROTOCOL", "REPLACE_ARRAYS": "PMAX_ARRAYS", "REPLACE_ENDPOINT": "PMAX_AUTH_ENDPOINT"}
	// Auth V2
	pflexCrMap  = map[string]string{"REPLACE_STORAGE_NAME": "PFLEX_STORAGE", "REPLACE_STORAGE_TYPE": "PFLEX_STORAGE", "REPLACE_ENDPOINT": "PFLEX_ENDPOINT", "REPLACE_SYSTEM_ID": "PFLEX_SYSTEMID", "REPLACE_VAULT_STORAGE_PATH": "PFLEX_VAULT_STORAGE_PATH", "REPLACE_ROLE_NAME": "PFLEX_ROLE", "REPLACE_QUOTA": "PFLEX_QUOTA", "REPLACE_STORAGE_POOL_PATH": "PFLEX_POOL", "REPLACE_TENANT_NAME": "PFLEX_TENANT", "REPLACE_TENANT_ROLES": "PFLEX_ROLE", "REPLACE_TENANT_VOLUME_PREFIX": "PFLEX_TENANT_PREFIX"}
	pscaleCrMap = map[string]string{"REPLACE_STORAGE_NAME": "PSCALE_STORAGE", "REPLACE_STORAGE_TYPE": "PSCALE_STORAGE", "REPLACE_ENDPOINT": "PSCALE_ENDPOINT", "REPLACE_SYSTEM_ID": "PSCALE_CLUSTER", "REPLACE_VAULT_STORAGE_PATH": "PSCALE_VAULT_STORAGE_PATH", "REPLACE_ROLE_NAME": "PSCALE_ROLE", "REPLACE_QUOTA": "PSCALE_QUOTA", "REPLACE_STORAGE_POOL_PATH": "PSCALE_POOL_V2", "REPLACE_TENANT_NAME": "PSCALE_TENANT", "REPLACE_TENANT_ROLES": "PSCALE_ROLE", "REPLACE_TENANT_VOLUME_PREFIX": "PSCALE_TENANT_PREFIX"}
	pmaxCrMap   = map[string]string{"REPLACE_STORAGE_NAME": "PMAX_STORAGE", "REPLACE_STORAGE_TYPE": "PMAX_STORAGE", "REPLACE_ENDPOINT": "PMAX_ENDPOINT", "REPLACE_SYSTEM_ID": "PMAX_SYSTEMID", "REPLACE_VAULT_STORAGE_PATH": "PMAX_VAULT_STORAGE_PATH", "REPLACE_ROLE_NAME": "PMAX_ROLE", "REPLACE_QUOTA": "PMAX_QUOTA", "REPLACE_STORAGE_POOL_PATH": "PMAX_POOL_V2", "REPLACE_TENANT_NAME": "PMAX_TENANT", "REPLACE_TENANT_ROLES": "PMAX_ROLE", "REPLACE_TENANT_VOLUME_PREFIX": "PMAX_TENANT_PREFIX"}

	pstoreSecretMap          = map[string]string{"REPLACE_USER": "PSTORE_USER", "REPLACE_PASS": "PSTORE_PASS", "REPLACE_GLOBALID": "PSTORE_GLOBALID", "REPLACE_ENDPOINT": "PSTORE_ENDPOINT", "REPLACE_PROTOCOL": "PSTORE_PROTOCOL"}
	pstoreEphemeralVolumeMap = map[string]string{"REPLACE_GLOBALID": "PSTORE_GLOBALID"}
	unitySecretMap           = map[string]string{"REPLACE_USER": "UNITY_USER", "REPLACE_PASS": "UNITY_PASS", "REPLACE_ARRAYID": "UNITY_ARRAYID", "REPLACE_ENDPOINT": "UNITY_ENDPOINT", "REPLACE_POOL": "UNITY_POOL", "REPLACE_NAS": "UNITY_NAS"}
	unityEphemeralVolumeMap  = map[string]string{"REPLACE_ARRAYID": "UNITY_ARRAYID", "REPLACE_POOL": "UNITY_POOL", "REPLACE_NAS": "UNITY_NAS"}
)

var correctlyAuthInjected = func(cr csmv1.ContainerStorageModule, annotations map[string]string, vols []acorev1.VolumeApplyConfiguration, cnt []acorev1.ContainerApplyConfiguration) error {
	err := modules.CheckAnnotationAuth(annotations)
	if err != nil {
		return err
	}

	err = modules.CheckApplyVolumesAuth(vols)
	if err != nil {
		return err
	}
	err = modules.CheckApplyContainersAuth(cnt, string(cr.Spec.Driver.CSIDriverType), true)
	if err != nil {
		return err
	}
	return nil
}

// GetTestResources -- parse values file
func GetTestResources(valuesFilePath string) ([]Resource, error) {
	b, err := os.ReadFile(valuesFilePath) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("failed to read values file: %v", err)
	}

	scenarios := []Scenario{}
	err = yaml.Unmarshal(b, &scenarios)
	if err != nil {
		return nil, fmt.Errorf("failed to read unmarshal values file: %v", err)
	}

	resources := []Resource{}
	for _, scene := range scenarios {
		var customResources []interface{}
		for _, path := range scene.Paths {
			b, err := os.ReadFile(path) // #nosec G304
			if err != nil {
				return nil, fmt.Errorf("failed to read testdata: %v", err)
			}

			customResource := csmv1.ContainerStorageModule{}
			err = yaml.Unmarshal(b, &customResource)
			if err != nil {
				return nil, fmt.Errorf("failed to read unmarshal CSM custom resource: %v", err)
			}
			customResources = append(customResources, customResource)
		}
		resources = append(resources, Resource{
			Scenario:       scene,
			CustomResource: customResources,
		})
	}

	return resources, nil
}

func (step *Step) applyCustomResource(res Resource, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	crFilePath := res.Scenario.Paths[crNum-1]

	// If the specified file is a template, assume it was rendered to a temporary file earlier.
	// Attempt to read the rendered file first. If it doesn't exist, assume the specified file
	// is not a template and should be applied as-is.

	tempFilePath := getRenderedFilePath(crFilePath)
	crBuff, err := os.ReadFile(tempFilePath) // #nosec G304
	if os.IsNotExist(err) {
		// There is no corresponding rendered file, use crFilePath
		crBuff, err = os.ReadFile(crFilePath) // #nosec G304
	}
	if err != nil {
		return fmt.Errorf("failed to read testdata: %v", err)
	}

	if _, err := kubectl.RunKubectlInput(cr.Namespace, string(crBuff), "apply", "--validate=true", "-f", "-"); err != nil {
		return fmt.Errorf("failed to apply CR %s in namespace %s: %v", cr.Name, cr.Namespace, err)
	}

	return nil
}

func (step *Step) upgradeCustomResource(res Resource, oldCrNumStr, newCrNumStr string) error {
	oldCrNum, _ := strconv.Atoi(oldCrNumStr)
	oldCr := res.CustomResource[oldCrNum-1].(csmv1.ContainerStorageModule)

	newCrNum, _ := strconv.Atoi(newCrNumStr)
	newCr := res.CustomResource[newCrNum-1].(csmv1.ContainerStorageModule)

	time.Sleep(60 * time.Second)

	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: oldCr.Namespace,
		Name:      oldCr.Name,
	}, found); err != nil {
		return err
	}

	// Update old CR with the spec of new CR
	found.Spec = newCr.Spec
	return step.ctrlClient.Update(context.TODO(), found)
}

func (step *Step) installThirdPartyModule(_ Resource, thirdPartyModule string) error {
	if thirdPartyModule == "cert-manager" {
		cmd := exec.Command("kubectl", "apply", "-f", "testfiles/cert-manager-crds.yaml")
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("cert-manager install failed: %v", err)
		}
	} else if thirdPartyModule == "velero" {
		cmd1 := exec.Command("helm", "repo", "add", "vmware-tanzu", "https://vmware-tanzu.github.io/helm-charts")
		err1 := cmd1.Run()
		if err1 != nil {
			return fmt.Errorf("installation of velero %v failed", err1)
		}

		amNamespace := os.Getenv("AM_NS")
		if amNamespace == "" {
			amNamespace = "test-vxflexos"
		}

		// Cleanup backupstoragelocations and volumesnapshotlocation before installing velero
		cmd := exec.Command("kubectl", "get", "backupstoragelocations.velero.io", "default", "-n", amNamespace) // #nosec G204
		err := cmd.Run()
		if err == nil {
			cmd1 = exec.Command("kubectl", "delete", "backupstoragelocations.velero.io", "default", "-n", amNamespace) // #nosec G204
			err1 = cmd1.Run()
			if err1 != nil {
				return fmt.Errorf("installation of velero failed: %v", err1)
			}
		}

		cmd = exec.Command("kubectl", "get", "volumesnapshotlocations.velero.io", "default", "-n", amNamespace) // #nosec G204
		err = cmd.Run()
		if err == nil {
			cmd1 = exec.Command("kubectl", "delete", "volumesnapshotlocations.velero.io", "default", "-n", amNamespace) // #nosec G204
			err1 = cmd1.Run()
			if err1 != nil {
				return fmt.Errorf("installation of velero failed: %v", err1)
			}
		}

		cmd2 := exec.Command("helm", "install", "velero", "vmware-tanzu/velero", "--namespace="+amNamespace, "--create-namespace",
			"-f", getRenderedFilePath("testfiles/application-mobility-templates/velero-values.yaml")) // #nosec G204
		err2 := cmd2.Run()
		if err2 != nil {
			return fmt.Errorf("installation of velero failed: %v", err2)
		}
	} else if thirdPartyModule == "sample-app" {

		cmd := exec.Command("kubectl", "get", "ns", "ns1") // #nosec G204
		err := cmd.Run()
		if err != nil {
			cmd = exec.Command("kubectl", "create", "ns", "ns1") // #nosec G204
			err = cmd.Run()
			if err != nil {
				return err
			}
		}

		// create a stateful set with one pod and one volume for AM testing, requires pflex driver installed and op-e2e-vxflexos SC created
		cmd2 := exec.Command("kubectl", "apply", "-n", "ns1", "-f", "testfiles/sample-application/test-sts.yaml")
		err = cmd2.Run()
		if err != nil {
			return err
		}

		// give wp time to setup before we create backup/restores
		fmt.Println("Sleeping 20 seconds to allow stateful set time to create")
		time.Sleep(20 * time.Second)

	}

	return nil
}

func (step *Step) uninstallThirdPartyModule(res Resource, thirdPartyModule string) error {
	if thirdPartyModule == "cert-manager" {
		cmd := exec.Command("kubectl", "delete", "-f", "testfiles/cert-manager-crds.yaml") // #nosec G204
		err := cmd.Run()
		if err != nil {
			// Some deployments are not found since they are deleted already.
			cmd = exec.Command("kubectl", "get", "pods", "-n", "cert-manager") // #nosec G204
			err = cmd.Run()
			if err != nil {
				return fmt.Errorf("cert-manager uninstall failed: %v", err)
			}
		}
	} else if thirdPartyModule == "velero" {
		amNamespace := os.Getenv("AM_NS")
		if amNamespace == "" {
			amNamespace = "test-vxflexos"
		}

		cmd := exec.Command("helm", "delete", "velero", "--namespace="+amNamespace) // #nosec G204
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("uninstallation of velero %v failed", err)
		}
	} else if thirdPartyModule == "sample-app" {
		cmd := exec.Command("kubectl", "delete", "-n", "ns1", "-f", "testfiles/sample-application/test-sts.yaml") // #nosec G204
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("uninstallation of stateful set failed:  %v", err)
		}

	}
	return nil
}

func (step *Step) deleteCustomResource(res Resource, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found,
	)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return step.ctrlClient.Delete(context.TODO(), &cr)
}

func (step *Step) validateCustomResourceStatus(res Resource, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	time.Sleep(20 * time.Second)
	found := new(csmv1.ContainerStorageModule)
	err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found)
	if err != nil {
		return err
	}
	if found.Status.State != constants.Succeeded {
		return fmt.Errorf("expected custom resource status to be %s. Got: %s", constants.Succeeded, found.Status.State)
	}

	return nil
}

func (step *Step) validateContainerArg(res Resource, crNumStr string, arg string, container string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	dp, err := getDriverDeployment(cr, step.ctrlClient)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %v", err)
	}
	containerFound := false
	for _, cnt := range dp.Spec.Template.Spec.Containers {
		if cnt.Name == container {
			containerFound = true
			// iterate through args and see if it was found
			for _, argVal := range cnt.Args {
				if argVal == arg {
					return nil
				}
			}
			return fmt.Errorf("container arg %s not found on container %s", arg, container)
		}
	}
	if !containerFound {
		return fmt.Errorf("container %s not found in deployment", container)
	}

	return fmt.Errorf("unknown error validating container arg")
}

func (step *Step) validateDriverInstalled(res Resource, driverName string, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	time.Sleep(20 * time.Second)
	return checkAllRunningPods(context.TODO(), res.CustomResource[crNum-1].(csmv1.ContainerStorageModule).Namespace, step.clientSet)
}

func (step *Step) validateMinimalCSMDriverSpec(res Resource, driverName string, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found)
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("CSM resource '%s' not found in namespace '%s'", cr.Name, cr.Namespace)
		}
		return fmt.Errorf("failed to get CSM resource '%s/%s': %w", cr.Namespace, cr.Name, err)
	}
	driver := found.Spec.Driver
	if driver.ConfigVersion == "" {
		return fmt.Errorf("configVersion is missing")
	}
	if driver.CSIDriverType == "" {
		return fmt.Errorf("csiDriverType is missing")
	}

	// Ensure that the expected number of controller pods are running.
	status := found.Status
	if status.ControllerStatus.Failed > "0" {
		return fmt.Errorf("replicas should have a non-zero value")
	}

	// Ensure all other fields are empty or nil
	if len(driver.SideCars) > 0 ||
		len(driver.InitContainers) > 0 ||
		len(driver.SnapshotClass) > 0 ||
		driver.Controller != nil ||
		driver.CSIDriverSpec != nil ||
		driver.DNSPolicy != "" ||
		driver.AuthSecret != "" ||
		driver.TLSCertSecret != "" {
		return fmt.Errorf("unexpected fields found in Driver spec: %+v", driver)
	}

	return nil
}

func (step *Step) validateDriverNotInstalled(res Resource, driverName string, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	time.Sleep(20 * time.Second)
	return checkNoRunningPods(context.TODO(), res.CustomResource[crNum-1].(csmv1.ContainerStorageModule).Namespace, step.clientSet)
}

func (step *Step) setNodeLabel(res Resource, label string) error {
	if label == "control-plane" {
		_ = setNodeLabel(label, "node-role.kubernetes.io/control-plane", "")
	} else {
		return fmt.Errorf("Adding node label %s not supported, feel free to add support", label)
	}

	return nil
}

func (step *Step) removeNodeLabel(res Resource, label string) error {
	if label == "control-plane" {
		_ = removeNodeLabel(label, "node-role.kubernetes.io/control-plane")
	} else {
		return fmt.Errorf("Removing node label %s not supported, feel free to add support", label)
	}

	return nil
}

func (step *Step) validateModuleInstalled(res Resource, module string, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	time.Sleep(10 * time.Second)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found,
	); err != nil {
		return err
	}

	for _, m := range found.Spec.Modules {
		if m.Name == csmv1.ModuleType(module) {
			if !m.Enabled {
				return fmt.Errorf("%s module is not enabled in CR", m.Name)
			}
			switch m.Name {
			case csmv1.Authorization:
				return step.validateAuthorizationInstalled(cr)

			case csmv1.Replication:
				return step.validateReplicationInstalled(cr)

			case csmv1.Observability:
				return step.validateObservabilityInstalled(cr)

			case csmv1.AuthorizationServer:
				return step.validateAuthorizationProxyServerInstalled(cr)

			case csmv1.Resiliency:
				return step.validateResiliencyInstalled(cr)

			case csmv1.ApplicationMobility:
				return step.validateAppMobInstalled(cr)

			default:
				return fmt.Errorf("%s module is not found", module)
			}
		}
	}
	return nil
}

func (step *Step) validateModuleNotInstalled(res Resource, module string, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	time.Sleep(10 * time.Second)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found,
	); err != nil {
		return err
	}

	for _, m := range found.Spec.Modules {
		if m.Name == csmv1.ModuleType(module) {
			if m.Enabled {
				return fmt.Errorf("%s module is enabled in CR", m.Name)
			}
			switch m.Name {
			case csmv1.Authorization:
				return step.validateAuthorizationNotInstalled(cr)

			case csmv1.Replication:
				return step.validateReplicationNotInstalled(cr)

			case csmv1.Observability:
				return step.validateObservabilityNotInstalled(cr)

			case csmv1.AuthorizationServer:
				return step.validateAuthorizationProxyServerNotInstalled(cr)

			case csmv1.Resiliency:
				return step.validateResiliencyNotInstalled(cr)
			case csmv1.ApplicationMobility:
				return step.validateApplicationMobilityNotInstalled(cr)
			}
		}
	}

	return nil
}

func (step *Step) validateObservabilityInstalled(cr csmv1.ContainerStorageModule) error {
	instance := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, instance,
	); err != nil {
		return err
	}

	// check installation for all replicas
	fakeReconcile := operatorutils.FakeReconcileCSM{
		Client:    step.ctrlClient,
		K8sClient: step.clientSet,
	}

	csmNamespace := cr.Namespace
	clusterClient := operatorutils.GetCluster(context.TODO(), &fakeReconcile)

	// check observability in all clusters
	if err := checkObservabilityRunningPods(context.TODO(), csmNamespace, clusterClient.ClusterK8sClient); err != nil {
		return fmt.Errorf("failed to check for observability installation in %s: %v", clusterClient.ClusterID, err)
	}

	// check observability's authorization
	driverType := cr.Spec.Driver.CSIDriverType
	dpApply, err := getApplyObservabilityDeployment(csmNamespace, driverType, clusterClient.ClusterCTRLClient)
	if err != nil {
		return err
	if authorizationEnabled, _ := operatorutils.IsModuleEnabled(context.TODO(), *instance, csmv1.Authorization); authorizationEnabled {
	} else {
		for _, cnt := range dpApply.Spec.Template.Spec.Containers {
			if *cnt.Name == authString {
				return fmt.Errorf("found observability authorization in deployment: %v, err:%v", dpApply.Name, err)
			}
		}
	}

	return nil
}

func (step *Step) validateObservabilityNotInstalled(cr csmv1.ContainerStorageModule) error {
	// check installation for all replicas
	fakeReconcile := operatorutils.FakeReconcileCSM{
		Client:    step.ctrlClient,
		K8sClient: step.clientSet,
	}

	csmNamespace := cr.Namespace
	clusterClient := operatorutils.GetCluster(context.TODO(), &fakeReconcile)

	// check observability is not installed
	if err := checkObservabilityNoRunningPods(context.TODO(), csmNamespace, clusterClient.ClusterK8sClient); err != nil {
		return fmt.Errorf("failed observability installation check %s: %v", clusterClient.ClusterID, err)
	}

	return nil
}

func (step *Step) validateReplicationInstalled(cr csmv1.ContainerStorageModule) error {
	dpApply, _, err := getApplyDeploymentDaemonSet(cr, step.ctrlClient)
	if err != nil {
		return err
	}
	if err := modules.CheckApplyContainersReplica(dpApply.Spec.Template.Spec.Containers, cr); err != nil {
		return err
	}

	// cluster role
	clusterRole := &rbacv1.ClusterRole{}
	err = step.ctrlClient.Get(context.TODO(), types.NamespacedName{
		Name: fmt.Sprintf("%s-controller", cr.Name),
	}, clusterRole)
	if err != nil {
		return err
	}
	if err := modules.CheckClusterRoleReplica(clusterRole.Rules); err != nil {
		return err
	}

	// check installation for all replicas
	fakeReconcile := operatorutils.FakeReconcileCSM{
		Client:    step.ctrlClient,
		K8sClient: step.clientSet,
	}

	clusterClient := operatorutils.GetCluster(context.TODO(), &fakeReconcile)

	// check replication controllers in cluster
	if err := checkAllRunningPods(context.TODO(), operatorutils.ReplicationControllerNameSpace, clusterClient.ClusterK8sClient); err != nil {
		return fmt.Errorf("failed to check for  replication controllers installation in %s: %v", clusterClient.ClusterID, err)
	}

	// check driver deployment in cluster
	if err := checkAllRunningPods(context.TODO(), cr.Namespace, clusterClient.ClusterK8sClient); err != nil {
		return fmt.Errorf("failed while check for driver installation in %s: %v", clusterClient.ClusterID, err)
	}

	return nil
}

func (step *Step) validateReplicationNotInstalled(cr csmv1.ContainerStorageModule) error {
	// check installation for all replicas
	fakeReconcile := operatorutils.FakeReconcileCSM{
		Client:    step.ctrlClient,
		K8sClient: step.clientSet,
	}

	clusterClient := operatorutils.GetCluster(context.TODO(), &fakeReconcile)

	// check replication  controller is not installed
	if err := checkNoRunningPods(context.TODO(), operatorutils.ReplicationControllerNameSpace, clusterClient.ClusterK8sClient); err != nil {
		return fmt.Errorf("failed replica installation check %s: %v", clusterClient.ClusterID, err)
	}

	// check that replication sidecar is not in source cluster
	dp, err := getDriverDeployment(cr, step.ctrlClient)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %v", err)
	}
	for _, cnt := range dp.Spec.Template.Spec.Containers {
		if cnt.Name == operatorutils.ReplicationSideCarName {
			return fmt.Errorf("found %s: %v", operatorutils.ReplicationSideCarName, err)
		}
	}

	return nil
}

func (step *Step) validateAuthorizationInstalled(cr csmv1.ContainerStorageModule) error {
	dpApply, dsApply, err := getApplyDeploymentDaemonSet(cr, step.ctrlClient)
	if err != nil {
		return err
	}

	if err := correctlyAuthInjected(cr, dpApply.Annotations, dpApply.Spec.Template.Spec.Volumes, dpApply.Spec.Template.Spec.Containers); err != nil {
		return err
	}

	return correctlyAuthInjected(cr, dsApply.Annotations, dsApply.Spec.Template.Spec.Volumes, dsApply.Spec.Template.Spec.Containers)
}

func (step *Step) validateAuthorizationNotInstalled(cr csmv1.ContainerStorageModule) error {
	dpApply, dsApply, err := getApplyDeploymentDaemonSet(cr, step.ctrlClient)
	if err != nil {
		return err
	}

	for _, cnt := range dpApply.Spec.Template.Spec.Containers {
		if *cnt.Name == authString {
			return fmt.Errorf("found authorization in deployment: %v", err)
		}
	}

	for _, cnt := range dsApply.Spec.Template.Spec.Containers {
		if *cnt.Name == authString {
			return fmt.Errorf("found authorization in daemonset: %v", err)
		}
	}

	return nil
}

func (step *Step) validateAuthorizationPodsNotInstalled(res Resource, module string, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	time.Sleep(20 * time.Second)
	return checkNoRunningPods(context.TODO(), res.CustomResource[crNum-1].(csmv1.ContainerStorageModule).Namespace, step.clientSet)
}

func (step *Step) setUpStorageClass(_ Resource, templateFile, crType string) error {

	fileString, err := renderTemplate(crType, templateFile)
	if err != nil {
		return err
	}

	// parse resource name out of the spec
	type NamedResource struct {
		Metadata struct {
			Name string `yaml:"name"`
		} `yaml:"metadata"`
	}
	var res NamedResource

	err = yaml.Unmarshal([]byte(fileString), &res)
	if err != nil {
		return fmt.Errorf("error unmarshalling template file %s: %v", templateFile, err)
	}
	name := res.Metadata.Name

	// if resource exists - delete it
	if storageClassExists(name) {
		err := execCommand("kubectl", "delete", "sc", name)
		if err != nil {
			return fmt.Errorf("failed to delete storage class: %v", err)
		}
	}

	filePath, err := writeRenderedFile(templateFile, fileString)
	if err != nil {
		return err
	}

	// create new storage class
	err = execCommand("kubectl", "create", "-f", filePath)
	if err != nil {
		return fmt.Errorf("failed to create storage class with template file %s: %v", templateFile, err)
	}
	return nil
}

func (step *Step) createResource(_ Resource, templateFile, crType string) error {

	fileString, err := renderTemplate(crType, templateFile)
	if err != nil {
		return err
	}

	filePath, err := writeRenderedFile(templateFile, fileString)
	if err != nil {
		return err
	}

	err = execCommand("kubectl", "apply", "-f", filePath)
	if err != nil {
		return fmt.Errorf("failed to apply resource spec file %s: %v", filePath, err)
	}
	return nil
}

func (step *Step) setUpConfigMap(res Resource, templateFile, name, namespace, crType string) error {
	fileString, err := renderTemplate(crType, templateFile)
	if err != nil {
		return err
	}

	// if resource exists - delete it
	if configMapExists(namespace, name) {
		err := execCommand("kubectl", "delete", "configmap", "-n", namespace, name)
		if err != nil {
			return fmt.Errorf("failed to delete config map: %v", err)
		}
	}

	filePath, err := writeRenderedFile(templateFile, fileString)
	if err != nil {
		return err
	}

	// create new storage class
	fileArg := "--from-file=config.yaml=" + filePath
	err = execCommand("kubectl", "create", "cm", name, "-n", namespace, fileArg)
	if err != nil {
		return fmt.Errorf("failed to create storage class with template file %s: %v", templateFile, err)
	}
	return nil
}

func (step *Step) setUpSecret(_ Resource, templateFile, name, namespace, crType string) error {
	fileString, err := renderTemplate(crType, templateFile)
	if err != nil {
		return err
	}

	// if secret exists - delete it
	if secretExists(namespace, name) {
		err := execCommand("kubectl", "delete", "secret", "-n", namespace, name)
		if err != nil {
			return fmt.Errorf("failed to delete secret: %s", err.Error())
		}
	}

	// create new secret
	fileArg := "--from-literal=config=" + fileString
	err = execCommand("kubectl", "create", "secret", "generic", "-n", namespace, name, fileArg)
	if err != nil {
		return fmt.Errorf("failed to create secret with template file %s: %v", templateFile, err)
	}

	return nil
}

func (step *Step) generateAndCreateSftpSecrets(_ Resource, privateKeyPath, privateSecretName, publicSecretName, namespace, crType string) error {
	tmpDir := filepath.Join("temp", "sftp", fmt.Sprintf("%d", time.Now().UnixNano()))
	defer os.RemoveAll(tmpDir)

	// Load env vars
	repoAddress, repoUser := os.Getenv("PFLEX_SFTP_REPO_ADDRESS"), os.Getenv("PFLEX_SFTP_REPO_USER")
	if repoAddress == "" || repoUser == "" {
		return fmt.Errorf("PFLEX_SFTP_REPO_ADDRESS and PFLEX_SFTP_REPO_USER must be set")
	}
	repoHost := strings.TrimPrefix(repoAddress, "sftp://")
	repoHost = strings.TrimSuffix(repoHost, "/")

	// Prepare temp directories
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	sshDir := filepath.Join(tmpDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	// Copy private key
	privateKeyFile := filepath.Join(tmpDir, "id_rsa")
	privateKeyData, err := os.ReadFile(filepath.Clean(privateKeyPath))
	if err != nil {
		return fmt.Errorf("failed to read private key: %v", err)
	}
	if err := os.WriteFile(privateKeyFile, privateKeyData, 0o600); err != nil {
		return fmt.Errorf("failed to write private key to temp dir: %v", err)
	}

	// Run SFTP session to populate known_hosts
	knownHostsPath := filepath.Join(sshDir, "known_hosts")
	cmd := exec.Command("sftp",
		"-o", "UserKnownHostsFile="+knownHostsPath,
		"-o", "StrictHostKeyChecking=accept-new",
		"-i", privateKeyFile,
		fmt.Sprintf("%s@%s", repoUser, repoHost),
	) // #nosec G204
	cmd.Stdin = strings.NewReader("exit\n")
	cmd.Env = append(os.Environ(), "HOME="+tmpDir)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sftp session failed: %v", err)
	}

	// Extract repo public key from known_hosts
	pubKeyBytes, err := os.ReadFile(filepath.Clean(knownHostsPath))
	if err != nil {
		return fmt.Errorf("failed to read known_hosts: %v", err)
	}
	hostPubKey, err := extractHostPublicKey(string(pubKeyBytes), repoHost)
	if err != nil {
		return err
	}

	// Write key files to disk for secret creation
	privateOut := filepath.Join(tmpDir, "sftp-secret-private.yaml")
	publicOut := filepath.Join(tmpDir, "sftp-secret-public.yaml")
	if err := os.WriteFile(privateOut, privateKeyData, 0o600); err != nil {
		return fmt.Errorf("failed to write private secret file: %v", err)
	}
	if err := os.WriteFile(publicOut, []byte(hostPubKey), 0o600); err != nil {
		return fmt.Errorf("failed to write public secret file: %v", err)
	}

	// Delete and recreate secrets
	if err := deleteSecretIfExists(namespace, privateSecretName); err != nil {
		return fmt.Errorf("failed to delete private secret: %w", err)
	}
	if err := deleteSecretIfExists(namespace, publicSecretName); err != nil {
		return fmt.Errorf("failed to delete public secret: %w", err)
	}
	if err := execCommand("kubectl", "create", "secret", "generic", privateSecretName,
		"-n", namespace,
		"--from-file=user_private_rsa_key="+privateOut); err != nil {
		return fmt.Errorf("failed to create private SFTP secret: %v", err)
	}

	if err := execCommand("kubectl", "create", "secret", "generic", publicSecretName,
		"-n", namespace,
		"--from-file=repo_public_rsa_key="+publicOut); err != nil {
		return fmt.Errorf("failed to create public SFTP secret: %v", err)
	}
	fmt.Println("SFTP secrets created successfully.")

	return nil
}

// Extract public key line for host from known_hosts
func extractHostPublicKey(knownHostsContent, repoHost string) (string, error) {
	for _, line := range strings.Split(knownHostsContent, "\n") {
		if strings.HasPrefix(line, repoHost+" ") {
			return line, nil
		}
	}
	return "", fmt.Errorf("could not extract %s public key from known_hosts", repoHost)
}

// Delete secret if exists in a namespace
func deleteSecretIfExists(namespace, secretName string) error {
	if secretExists(namespace, secretName) {
		return execCommand("kubectl", "delete", "secret", "-n", namespace, secretName)
	}
	return nil
}

func renderTemplate(crType string, templateFile string) (string, error) {
	// find which map to use for secret values
	mapValues, err := determineMap(crType)
	if err != nil {
		return "", err
	}

	// read the template into memory
	fileContent, err := os.ReadFile(templateFile) // #nosec G304
	if err != nil {
		return "", fmt.Errorf("error reading template file: %v", err)
	}

	// Convert the file content to a string
	fileString := string(fileContent)

	// Replace all fields in temporary (in memory) string
	for key, val := range mapValues {
		fileString = strings.ReplaceAll(fileString, key, os.Getenv(val))
	}
	return fileString, nil
}

func determineMap(crType string) (map[string]string, error) {
	mapValues := map[string]string{}
	if crType == "pflex" {
		mapValues = pflexSecretMap
	} else if crType == "pflexAuth" {
		mapValues = pflexAuthSecretMap
	} else if crType == "pflexEphemeral" {
		mapValues = pflexEphemeralVolumeMap
	} else if crType == "pscale" {
		mapValues = pscaleSecretMap
	} else if crType == "pscaleEphemeral" {
		mapValues = pscaleEphemeralVolumeMap
	} else if crType == "pscaleAuth" {
		mapValues = pscaleAuthSecretMap
	} else if crType == "pscaleAuthSidecar" {
		mapValues = pscaleAuthSidecarMap
	} else if crType == "pflexAuthSidecar" {
		mapValues = pflexAuthSidecarMap
	} else if crType == "pmax" {
		mapValues = pmaxStorageMap
	} else if crType == "pmaxAuthSidecar" {
		mapValues = pmaxAuthSidecarMap
	} else if crType == "pmaxCreds" {
		mapValues = pmaxCredMap
	} else if crType == "pmaxUseSecret" {
		mapValues = pmaxSecretMap
	} else if crType == "pmaxReverseProxy" {
		mapValues = pmaxReverseProxyMap
	} else if crType == "pmaxArrayConfig" {
		mapValues = pmaxArrayConfigMap
	} else if crType == "pmaxAuthArrayConfig" {
		mapValues = pmaxAuthArrayConfigMap
	} else if crType == "authSidecarCert" {
		mapValues = authSidecarRootCertMap
	} else if crType == "application-mobility" {
		mapValues = amConfigMap
	} else if crType == "pflexAuthCRs" {
		mapValues = pflexCrMap
	} else if crType == "pscaleAuthCRs" {
		mapValues = pscaleCrMap
	} else if crType == "pmaxAuthCRs" {
		mapValues = pmaxCrMap
	} else if crType == "pstore" {
		mapValues = pstoreSecretMap
	} else if crType == "pstoreEphemeral" {
		mapValues = pstoreEphemeralVolumeMap
	} else if crType == "unity" {
		mapValues = unitySecretMap
	} else if crType == "unityEphemeral" {
		mapValues = unityEphemeralVolumeMap
	} else {
		return mapValues, fmt.Errorf("type: %s is not supported", crType)
	}

	return mapValues, nil
}

func secretExists(namespace, name string) bool {
	err := exec.Command("kubectl", "get", "secret", "-n", namespace, name).Run() // #nosec G204
	return err == nil
}

func configMapExists(namespace, name string) bool {
	err := exec.Command("kubectl", "get", "configmap", "-n", namespace, name).Run() // #nosec G204
	return err == nil
}

func storageClassExists(name string) bool {
	err := exec.Command("kubectl", "get", "storageclass", name).Run() // #nosec G204
	return err == nil
}

func replaceInFile(old, new, templateFile string) error { // TODO delete
	cmdString := "s|" + old + "|" + new + "|g"
	cmd := exec.Command("sed", "-i", cmdString, templateFile) // #nosec G204
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to substitute %s with %s in file %s: %s", old, new, templateFile, err.Error())
	}
	return nil
}

func (step *Step) runCustomTest(res Resource) error {
	var (
		stdout string
		stderr string
		err    error
	)
	if len(res.Scenario.CustomTest) != 1 {
		return fmt.Errorf("'customTest' must be a single element array")
	}

	for testNum, customTest := range res.Scenario.CustomTest[0].Run {
		args := strings.Split(customTest, " ")
		if len(args) == 1 {
			stdout, stderr, err = framework.RunCmd(args[0])
		} else {
			stdout, stderr, err = framework.RunCmd(args[0], args[1:]...)
		}

		if err != nil {
			return fmt.Errorf("error running custom test #%d. Error: %v \n stdout: %s \n stderr: %s", testNum, err, stdout, stderr)
		}
	}

	return nil
}

func (step *Step) runCustomTestSelector(res Resource, testName string) error {
	var (
		stdout string
		stderr string
		err    error
	)

	// retrieve the appropriate test from the list of tests
	var selectedTest CustomTest
	foundTest := false
	for _, test := range res.Scenario.CustomTest {
		if test.Name == testName {
			selectedTest = test
			foundTest = true
			break
		}
	}

	if !foundTest {
		return fmt.Errorf("custom test '%s' not found", testName)
	}

	for testNum, customTest := range selectedTest.Run {
		args := strings.Split(customTest, " ")
		if len(args) == 1 {
			stdout, stderr, err = framework.RunCmd(args[0])
		} else {
			stdout, stderr, err = framework.RunCmd(args[0], args[1:]...)
		}

		if err != nil {
			return fmt.Errorf("error running custom test #%d. Error: %v \n stdout: %s \n stderr: %s", testNum, err, stdout, stderr)
		}
	}

	return nil
}

func (step *Step) setupEphemeralVolumeProperties(_ Resource, templateFile string, crType string) error {

	if crType == "pflexEphemeral" {
		_ = os.Setenv("PFLEX_VOLUME", fmt.Sprintf("k8s-%s", randomAlphaNumberic(10)))
	}

	fileString, err := renderTemplate(crType, templateFile)
	if err != nil {
		return err
	}

	_, err = writeRenderedFile(templateFile, fileString)
	if err != nil {
		return err
	}

	return nil
}

func randomAlphaNumberic(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyz0123456789"

	var result []byte
	for i := 0; i < length; i++ {
		randomIndex := rand.Intn(len(charset)) // #nosec G404
		result = append(result, charset[randomIndex])
	}

	return string(result)
}

func getRenderedFilePath(templatePath string) string {
	return strings.Replace(templatePath, "testfiles/", "temp/", 1)
}

// To not contaminate the source tree with rendered template files,
// we write all rendered files under the same temp directory, but
// preserve the subdirectories structure. For example, for templatePath
// "testfiles/powerscale-templates/ephemeral.properties" the rendered file
// will be written to "temp/powerscale-templates/ephemeral.properties".
func writeRenderedFile(templatePath, content string) (newPath string, err error) {

	newPath = getRenderedFilePath(templatePath)

	// make sure the base path exist
	err = os.MkdirAll(filepath.Dir(newPath), 0o700)
	if err != nil {
		return "", fmt.Errorf("error creating temp directory %s: %v", filepath.Dir(newPath), err)
	}

	err = os.WriteFile(newPath, []byte(content), 0o644) // #nosec G306 -- this is a test automation tool
	if err != nil {
		return "", fmt.Errorf("error creating temp file: %v", err)
	}

	return newPath, nil
}

func (step *Step) enableModule(res Resource, module string, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	time.Sleep(15 * time.Second)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found,
	); err != nil {
		return err
	}

	for i, m := range found.Spec.Modules {
		if !m.Enabled && m.Name == csmv1.ModuleType(module) {
			found.Spec.Modules[i].Enabled = true
			// for observability, enable all components
			if m.Name == csmv1.Observability {
				for j := range m.Components {
					found.Spec.Modules[i].Components[j].Enabled = pointer.Bool(true)
				}
			}
		}
	}

	return step.ctrlClient.Update(context.TODO(), found)
}

func (step *Step) setDriverSecret(res Resource, crNumStr string, driverSecretName string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found,
	); err != nil {
		return err
	}
	found.Spec.Driver.AuthSecret = driverSecretName
	return step.ctrlClient.Update(context.TODO(), found)
}

func (step *Step) disableModule(res Resource, module string, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found,
	); err != nil {
		return err
	}

	for i, m := range found.Spec.Modules {
		if m.Enabled && m.Name == csmv1.ModuleType(module) {
			found.Spec.Modules[i].Enabled = false

			if m.Name == csmv1.Observability {
				for j := range m.Components {
					found.Spec.Modules[i].Components[j].Enabled = pointer.Bool(false)
				}
			}
		}
	}

	return step.ctrlClient.Update(context.TODO(), found)
}

func (step *Step) enableForceRemoveDriver(res Resource, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found,
	); err != nil {
		return err
	}

	truebool := true
	found.Spec.Driver.ForceRemoveDriver = &truebool
	return step.ctrlClient.Update(context.TODO(), found)
}

func (step *Step) validateForceRemoveDriverEnabled(res Resource, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found,
	); err != nil {
		return err
	}

	if found.Spec.Driver.ForceRemoveDriver != nil && *found.Spec.Driver.ForceRemoveDriver {
		return nil
	}
	return fmt.Errorf("forceRemoveDriver is not set to true")
}

func (step *Step) validateForceRemoveDriverDisabled(res Resource, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found,
	); err != nil {
		return err
	}

	if found.Spec.Driver.ForceRemoveDriver != nil && !*found.Spec.Driver.ForceRemoveDriver {
		return nil
	}
	return fmt.Errorf("forceRemoveDriver is not set to false")
}

func (step *Step) enableForceRemoveModule(res Resource, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found,
	); err != nil {
		return err
	}
	for _, module := range found.Spec.Modules {
		module.ForceRemoveModule = true
	}
	return step.ctrlClient.Update(context.TODO(), found)
}

func (step *Step) validateTestEnvironment(_ Resource) error {
	if os.Getenv("OPERATOR_NAMESPACE") != "" {
		operatorNamespace = os.Getenv("OPERATOR_NAMESPACE")
	}

	pods, err := fpod.GetPodsInNamespace(context.TODO(), step.clientSet, operatorNamespace, map[string]string{})
	if err != nil {
		return err
	}
	if len(pods) == 0 {
		return fmt.Errorf("operator is not installed in namespace [%s]", operatorNamespace)
	}

	notReadyMessage := ""
	allReady := true
	for _, pod := range pods {
		if pod.Status.Phase != corev1.PodRunning {
			allReady = false
			notReadyMessage += fmt.Sprintf("\nThe pod(%s) is %s", pod.Name, pod.Status.Phase)
		}
	}

	if !allReady {
		return fmt.Errorf("Bad Operator state:%s", notReadyMessage)
	}

	return nil
}

func (step *Step) createPrereqs(res Resource, module string, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.ModuleType(module) {
			switch m.Name {
			case csmv1.AuthorizationServer:
				return step.authProxyServerPrereqs(cr)

			default:
				return fmt.Errorf("%s module is not found", module)
			}
		}
	}

	return nil
}

func (step *Step) validateAuthorizationProxyServerInstalled(cr csmv1.ContainerStorageModule) error {
	instance := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, instance,
	); err != nil {
		return err
	}

	// check installation for all AuthorizationProxyServer
	fakeReconcile := operatorutils.FakeReconcileCSM{
		Client:    step.ctrlClient,
		K8sClient: step.clientSet,
	}

	clusterClient := operatorutils.GetCluster(context.TODO(), &fakeReconcile)

	// check AuthorizationProxyServer in all clusters
	if err := checkAuthorizationProxyServerPods(context.TODO(), cr.Namespace, clusterClient.ClusterK8sClient); err != nil {
		return fmt.Errorf("failed to check for AuthorizationProxyServer installation in %s: %v", clusterClient.ClusterID, err)
	}

	// provide few seconds for cluster to settle down
	time.Sleep(20 * time.Second)
	return nil
}

func (step *Step) validateAuthorizationProxyServerNotInstalled(cr csmv1.ContainerStorageModule) error {
	// check installation for all AuthorizationProxyServer
	fakeReconcile := operatorutils.FakeReconcileCSM{
		Client:    step.ctrlClient,
		K8sClient: step.clientSet,
	}

	clusterClient := operatorutils.GetCluster(context.TODO(), &fakeReconcile)

	// check AuthorizationProxyServer is not installed
	if err := checkAuthorizationProxyServerNoRunningPods(context.TODO(), cr.Namespace, clusterClient.ClusterK8sClient); err != nil {
		return fmt.Errorf("failed AuthorizationProxyServer installation check %s: %v", clusterClient.ClusterID, err)
	}

	return nil
}

func (step *Step) validateAppMobInstalled(cr csmv1.ContainerStorageModule) error {
	// providing additional time to get appmob pods up to running
	time.Sleep(10 * time.Second)
	instance := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, instance,
	); err != nil {
		return err
	}

	fakeReconcile := operatorutils.FakeReconcileCSM{
		Client:    step.ctrlClient,
		K8sClient: step.clientSet,
	}

	clusterClient := operatorutils.GetCluster(context.TODO(), &fakeReconcile)

	if err := checkApplicationMobilityPods(context.TODO(), cr.Namespace, clusterClient.ClusterK8sClient); err != nil {
		return fmt.Errorf("failed to check for App-mob installation in %s: %v", clusterClient.ClusterID, err)
	}

	// provide few seconds for cluster to settle down
	time.Sleep(10 * time.Second)
	return nil
}

func (step *Step) authProxyServerPrereqs(cr csmv1.ContainerStorageModule) error {
	fmt.Println("=== Creating Authorization Proxy Server Prerequisites ===")

	cmd := exec.Command("kubectl", "get", "ns", cr.Namespace) // #nosec G204
	err := cmd.Run()
	if err == nil {

		fmt.Printf("\nDeleting all CSM from namespace: %s \n", cr.Namespace)
		cmd = exec.Command("kubectl", "delete", "csm", "-n", cr.Namespace, "--all") // #nosec G204
		b, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to delete all CSM from namespace: %v\nErrMessage:\n%s", err, string(b))
		}

		cmd = exec.Command("kubectl", "delete", "ns", cr.Namespace) // #nosec G204
		b, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to delete authorization namespace: %v\nErrMessage:\n%s", err, string(b))
		}
	}

	cmd = exec.Command("kubectl", "create",
		"ns", cr.Namespace,
	) // #nosec G204
	b, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create authorization namespace: %v\nErrMessage:\n%s", err, string(b))
	}

	cmd = exec.Command("kubectl", "apply",
		"--validate=false", "-f",
		fmt.Sprintf("https://github.com/jetstack/cert-manager/releases/download/%s/cert-manager.crds.yaml",
			certManagerVersion),
	) // #nosec G204
	b, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to apply cert-manager CRDs: %v\nErrMessage:\n%s", err, string(b))
	}

	cmd = exec.Command("kubectl", "create",
		"secret", "generic",
		"karavi-config-secret",
		"-n", cr.Namespace,
		"--from-file=config.yaml=testfiles/authorization-templates/storage_csm_authorization_config.yaml",
	) // #nosec G204
	b, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create config secret for JWT: %v\nErrMessage:\n%s", err, string(b))
	}

	cmd = exec.Command("kubectl", "create", "-n", cr.Namespace,
		"-f", "testfiles/authorization-templates/storage_csm_authorization_storage_secret.yaml",
	) // #nosec G204
	b, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create storage secret: %v\nErrMessage:\n%s", err, string(b))
	}

	cmd = exec.Command("kubectl", "get", "sc", "local-storage") // #nosec G204
	err = cmd.Run()
	if err == nil {
		cmd = exec.Command("kubectl", "delete", "-f", "testfiles/authorization-templates/storage_csm_authorization_local_storage.yaml") // #nosec G204
		b, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to delete local storage: %v\nErrMessage:\n%s", err, string(b))
		}
	}

	cmd = exec.Command("kubectl", "create",
		"-f", "testfiles/authorization-templates/storage_csm_authorization_local_storage.yaml",
	) // #nosec G204
	b, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create local storage for redis: %v\nErrMessage:\n%s", err, string(b))
	}

	return nil
}

func (step *Step) configureAuthorizationProxyServer(res Resource, driver string, crNumStr string) error {
	fmt.Println("=== Configuring Authorization Proxy Server ===")

	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	var err error
	var (
		storageType     = ""
		driverNamespace = ""
		proxyHost       = ""
		csmTenantName   = ""
	)

	// if tests are running multiple scenarios that require differently configured auth servers, we will not be able to use one set of vars
	// this section is for powerflex, other drivers can add their sections as required.
	if driver == "powerflex" {
		_ = os.Setenv("PFLEX_STORAGE", "powerflex")
		_ = os.Setenv("DRIVER_NAMESPACE", "test-vxflexos")
		storageType = os.Getenv("PFLEX_STORAGE")
		csmTenantName = os.Getenv("PFLEX_TENANT")
	}

	if driver == "powerscale" {
		_ = os.Setenv("PSCALE_STORAGE", "powerscale")
		_ = os.Setenv("DRIVER_NAMESPACE", "isilon")
		storageType = os.Getenv("PSCALE_STORAGE")
		csmTenantName = os.Getenv("PSCALE_TENANT")
	}

	if driver == "powermax" {
		_ = os.Setenv("PMAX_STORAGE", "powermax")
		_ = os.Setenv("DRIVER_NAMESPACE", "powermax")
		storageType = os.Getenv("PMAX_STORAGE")
		csmTenantName = os.Getenv("PMAX_TENANT")
	}

	proxyHost = os.Getenv("PROXY_HOST")
	driverNamespace = os.Getenv("DRIVER_NAMESPACE")

	port, err := getPortContainerizedAuth(cr.Namespace)
	if err != nil {
		return err
	}

	address := proxyHost
	fmt.Printf("Address: %s\n", address)

	return step.AuthorizationV2Resources(storageType, driver, driverNamespace, address, port, csmTenantName)
}

// AuthorizationV2Resources creates resources using CRs and dellctl for V2 versions of Authorization Proxy Server
func (step *Step) AuthorizationV2Resources(storageType, driver, driverNamespace, proxyHost, port, csmTenantName string) error {
	var (
		crMap               = ""
		templateFile        = "testfiles/authorization-templates/storage_csm_authorization_v2_template.yaml"
		updatedTemplateFile = ""
	)

	if driver == "powerflex" {
		crMap = "pflexAuthCRs"
		updatedTemplateFile = "temp/authorization-templates/storage_csm_authorization_crs_powerflex.yaml"
	} else if driver == "powerscale" {
		crMap = "pscaleAuthCRs"
		updatedTemplateFile = "temp/authorization-templates/storage_csm_authorization_crs_powerscale.yaml"
	} else if driver == "powermax" {
		crMap = "pmaxAuthCRs"
		updatedTemplateFile = "temp/authorization-templates/storage_csm_authorization_crs_powermax.yaml"
	}

	err := execShell(fmt.Sprintf("mkdir -p temp/authorization-templates && cp %s %s", templateFile, updatedTemplateFile))
	if err != nil {
		return fmt.Errorf("failed to copy template file %s to %s: %v", templateFile, updatedTemplateFile, err)
	}

	// Create Admin Token
	fmt.Printf("=== Generating Admin Token ===\n")
	adminTkn := exec.Command("dellctl",
		"admin", "token",
		"--name", "Admin",
		"--jwt-signing-secret", "secret",
		"--refresh-token-expiration", fmt.Sprint(30*24*time.Hour),
		"--access-token-expiration", fmt.Sprint(2*time.Hour),
	) // #nosec G204
	b, err := adminTkn.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create admin token: %v\nErrMessage:\n%s", err, string(b))
	}

	fmt.Println("=== Writing Admin Token to Tmp File ===\n ")
	err = os.WriteFile("temp/adminToken.yaml", b, 0o644) // #nosec G303, G306
	if err != nil {
		return fmt.Errorf("failed to write admin token: %v\nErrMessage:\n%s", err, string(b))
	}

	// Create Resources
	fmt.Println("=== Creating Storage, Role, and Tenant ===\n ")
	mapValues, err := determineMap(crMap)
	if err != nil {
		return err
	}

	for key := range mapValues {
		val := os.Getenv(mapValues[key])
		if driver == "powerscale" && key == "REPLACE_ENDPOINT" {
			fmt.Println("Replacing PowerScale Endpoint and adding port...")

			port := os.Getenv(mapValues["REPLACE_PORT"])
			if port == "" {
				port = "8080"
			}

			val = val + ":" + port
		}

		err := replaceInFile(key, val, updatedTemplateFile)
		if err != nil {
			return err
		}
	}
	cmd := exec.Command("kubectl", "apply",
		"-f", updatedTemplateFile,
	)
	fmt.Println("=== Storage, Role, and Tenant === \n", cmd.String())
	b, err = cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(b), "is already registered") {
		return fmt.Errorf("failed to create resources for %s: %v\nErrMessage:\n%s", storageType, err, string(b))
	}

	fmt.Println("Waiting 5 seconds before generating token.")
	time.Sleep(5 * time.Second)

	// Generate tenant token
	fmt.Println("=== Generating token ===\n ")
	cmd = exec.Command("dellctl",
		"generate", "token",
		"--admin-token", "temp/adminToken.yaml",
		"--access-token-expiration", fmt.Sprint(10*time.Minute),
		"--refresh-token-expiration", "48h",
		"--tenant", csmTenantName,
		"--insecure", "--addr", fmt.Sprintf("%s:%s", proxyHost, port),
	) // #nosec G204
	fmt.Println("=== Token ===\n", cmd.String())
	b, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to generate token for %s: %v\nErrMessage:\n%s", csmTenantName, err, string(b))
	}

	// Apply token to CSI driver host
	fmt.Println("=== Applying token ===\n ")

	err = os.WriteFile("temp/token.yaml", b, 0o644) // #nosec G303, G306
	if err != nil {
		return fmt.Errorf("failed to write tenant token: %v\nErrMessage:\n%s", err, string(b))
	}

	cmd = exec.Command("kubectl", "apply",
		"-f", "temp/token.yaml",
		"-n", driverNamespace,
	) // #nosec G204
	b, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to apply token: %v\nErrMessage:\n%s", err, string(b))
	}
	fmt.Println("=== Token Applied ===\n ")

	return nil
}

func (step *Step) validateResiliencyInstalled(cr csmv1.ContainerStorageModule) error {
	dpApply, dsApply, err := getApplyDeploymentDaemonSet(cr, step.ctrlClient)
	if err != nil {
		return err
	}

	var presentInNode, presentInController bool
	// check whether podmon container is present in cluster or not: for controller
	for _, cnt := range dpApply.Spec.Template.Spec.Containers {
		if *cnt.Name == "podmon" {
			presentInController = true
			break
		}
	}

	// check whether podmon container is present in cluster or not: for node
	for _, cnt := range dsApply.Spec.Template.Spec.Containers {
		if *cnt.Name == "podmon" {
			presentInNode = true
			break
		}
	}

	if !presentInNode || !presentInController {
		return fmt.Errorf("podmon container not found either in controller or node pod")
	}

	return nil
}

func (step *Step) validateResiliencyNotInstalled(cr csmv1.ContainerStorageModule) error {
	// check that resiliency sidecar(podmon) is not in cluster: for controller
	dp, err := getDriverDeployment(cr, step.ctrlClient)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %v", err)
	}
	for _, cnt := range dp.Spec.Template.Spec.Containers {
		if cnt.Name == operatorutils.ResiliencySideCarName {
			return fmt.Errorf("found %s: %v", operatorutils.ResiliencySideCarName, err)
		}
	}

	// check that resiliency sidecar(podmon) is not in cluster: for node
	ds, err := getDriverDaemonset(cr, step.ctrlClient)
	if err != nil {
		return fmt.Errorf("failed to get daemonset: %v", err)
	}
	for _, cnt := range ds.Spec.Template.Spec.Containers {
		if cnt.Name == operatorutils.ResiliencySideCarName {
			return fmt.Errorf("found %s: %v", operatorutils.ResiliencySideCarName, err)
		}
	}
	return nil
}

// Render the AM CR template into a temporary file with the same name
func (step *Step) configureAMInstall(_ Resource, templateFile string) error {
	fileString, err := renderTemplate("application-mobility", templateFile)
	if err != nil {
		return err
	}

	filePath, err := writeRenderedFile(templateFile, fileString)
	if err != nil {
		return err
	}
	fmt.Printf("Rendered template %s into %s\n", templateFile, filePath)

	// Setup RH registry authentication secret. Calling it here,
	// since configureAMInstall is used to setup each AM test.
	err = setupAMImagePullSecret()
	if err != nil {
		return fmt.Errorf("failed to setup RH registry authentication for App Mobility: %v", err)
	}

	return nil
}

// Render the Powerflex SFTP CR template into a temporary file with the same name
func (step *Step) configurePowerflexSftpInstall(res Resource, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	crFilePath := res.Scenario.Paths[crNum-1]
	fileString, err := renderTemplate("pflex", crFilePath)
	if err != nil {
		return err
	}

	filePath, err := writeRenderedFile(crFilePath, fileString)
	if err != nil {
		return err
	}
	fmt.Printf("Rendered template %s into %s\n", crFilePath, filePath)

	return nil
}

// For authentication to registry.redhat.io, create an image pull secret and
// associate it with the service account vxflexos-app-mobility-controller,
// that is used by the AM controller manager.
// Normally, this service account is created by Operator, but we create it here
// in advance to set imagePullSecrets.
func setupAMImagePullSecret() error {
	if os.Getenv("RH_REGISTRY_USERNAME") == "" || os.Getenv("RH_REGISTRY_PASSWORD") == "" {
		return fmt.Errorf("env variable RH_REGISTRY_USERNAME or RH_REGISTRY_PASSWORD is not set," +
			" set it in array-info.env before continuing")
	}

	// Create or update the image pull secret
	err := execShell(`kubectl -n test-vxflexos create secret docker-registry rhregcred \
--docker-server="https://registry.redhat.io" --docker-username="$RH_REGISTRY_USERNAME" \
--docker-password="$RH_REGISTRY_PASSWORD" --dry-run=client -o yaml --save-config | kubectl apply -f -`)
	if err != nil {
		return fmt.Errorf("failed to create rh image pull secret: %v", err)
	}

	// Create or update the service account and associate it with the image pull secret
	err = execShell(`kubectl create --dry-run=client -o yaml --save-config \
-f testfiles/application-mobility-templates/controller-manager-sa.yaml | kubectl apply -f -`)
	if err != nil {
		return fmt.Errorf("failed to create am controller manager service account: %v", err)
	}

	return nil
}

func (step *Step) validateApplicationMobilityNotInstalled(cr csmv1.ContainerStorageModule) error {
	fakeReconcile := operatorutils.FakeReconcileCSM{
		Client:    step.ctrlClient,
		K8sClient: step.clientSet,
	}

	clusterClient := operatorutils.GetCluster(context.TODO(), &fakeReconcile)

	err := checkApplicationMobilityPods(context.TODO(), cr.Namespace, clusterClient.ClusterK8sClient)
	if err == nil {
		return fmt.Errorf("AM pods found in namespace: %s", cr.Namespace)
	}

	fmt.Println("All AM pods removed ")
	return nil
}

func (step *Step) createCustomResourceDefinition(res Resource, crdNumStr string) error {
	crdNum, _ := strconv.Atoi(crdNumStr)
	cmd := exec.Command("kubectl", "apply", "-f", res.Scenario.Paths[crdNum-1]) // #nosec G204
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("csm authorization crds install failed: %v", err)
	}

	return nil
}

func (step *Step) validateCustomResourceDefinition(res Resource, crdName string) error {
	cmd := exec.Command("kubectl", "get", "crd", fmt.Sprintf("%s.csm-authorization.storage.dell.com", crdName)) // #nosec G204
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to validate csm authorization crd [%s]: %v", crdName, err)
	}

	return nil
}

// deleteAuthorizationCRs will delete storage, role, and tenant objects
func (step *Step) deleteAuthorizationCRs(_ Resource, driver string) error {
	updatedTemplateFile := ""
	if driver == "powerflex" {
		updatedTemplateFile = "temp/authorization-templates/storage_csm_authorization_crs_powerflex.yaml"
	} else if driver == "powerscale" {
		updatedTemplateFile = "temp/authorization-templates/storage_csm_authorization_crs_powerscale.yaml"
	} else if driver == "powermax" {
		updatedTemplateFile = "temp/authorization-templates/storage_csm_authorization_crs_powermax.yaml"
	}

	cmd := exec.Command("kubectl", "delete", "-f", updatedTemplateFile)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to delete csm authorization CRs: %v", err)
	}
	return nil
}

func (step *Step) deleteCustomResourceDefinition(res Resource, crdNumStr string) error {
	crdNum, _ := strconv.Atoi(crdNumStr)
	cmd := exec.Command("kubectl", "delete", "-f", res.Scenario.Paths[crdNum-1]) // #nosec G204
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("csm authorization crds uninstall failed: %v", err)
	}
	return nil
}

func (step *Step) setUpReverseProxy(_ Resource, namespace string) error {
	// Check if the revproxy-certs secret exists
	revproxyExists := false
	cmd := exec.Command("kubectl", "get", "secret", "revproxy-certs", "-n", namespace) // #nosec G204
	err := cmd.Run()
	if err == nil {
		fmt.Println("revproxy-certs secret already exists, skipping creation.")
		revproxyExists = true
	}

	// Check if the csirevproxy-tls-secret exists
	csirevproxyExists := false
	cmd = exec.Command("kubectl", "get", "secret", "csirevproxy-tls-secret", "-n", namespace) // #nosec G204
	err = cmd.Run()
	if err == nil {
		fmt.Println("csirevproxy-tls-secret already exists, skipping creation.")
		csirevproxyExists = true
	}

	// If both secrets exist, no need to generate TLS key and certificate
	if revproxyExists && csirevproxyExists {
		return nil
	}

	// Paths for the key and certificate files
	keyPath := "temp/tls.key"
	crtPath := "temp/tls.crt"

	// Generate TLS key
	cmd = exec.Command("openssl", "genrsa", "-out", keyPath, "2048") // #nosec G204
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to generate TLS key: %v", err)
	}

	// Generate TLS certificate
	cmd = exec.Command("openssl", "req", "-new", "-x509", "-sha256", "-key", keyPath, "-out", crtPath, "-days", "3650", "-subj", "/CN=US") // #nosec G204
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to generate TLS certificate: %v", err)
	}

	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return fmt.Errorf("key file does not exist: %s", keyPath)
	}
	if _, err := os.Stat(crtPath); os.IsNotExist(err) {
		return fmt.Errorf("cert file does not exist: %s", crtPath)
	}

	// Create Kubernetes secret for revproxy-certs if it does not exist
	if !revproxyExists {
		cmd = exec.Command("kubectl", "create", "secret", "-n", namespace, "tls", "revproxy-certs", "--cert="+crtPath, "--key="+keyPath) // #nosec G204
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("failed to create revproxy-certs secret: %v", err)
		}
	}

	// Create Kubernetes secret for csirevproxy-tls-secret if it does not exist
	if !csirevproxyExists {
		cmd = exec.Command("kubectl", "create", "secret", "-n", namespace, "tls", "csirevproxy-tls-secret", "--cert="+crtPath, "--key="+keyPath) // #nosec G204
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("failed to create csirevproxy-tls-secret: %v", err)
		}
	}

	return nil
}

func (step *Step) setUpTLSSecretWithSAN(res Resource, namespace string) error {

	// Paths for the key, CSR, and certificate files
	keyPath := "temp/tls.key"
	csrPath := "temp/tls.csr"
	crtPath := "temp/tls.crt"
	sanConfigPath := "testfiles/powermax-templates/san.cnf"

	// Generate TLS key
	cmd := exec.Command("openssl", "genrsa", "-out", keyPath, "2048") // #nosec G204
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to generate TLS key: %v", err)
	}

	// Generate CSR
	cmd = exec.Command("openssl", "req", "-new", "-key", keyPath, "-out", csrPath, "-config", sanConfigPath) // #nosec G204
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to generate CSR: %v", err)
	}

	// Generate TLS certificate
	cmd = exec.Command("openssl", "x509", "-req", "-in", csrPath, "-signkey", keyPath, "-out", crtPath, "-days", "3650", "-extensions", "v3_req", "-extfile", sanConfigPath) // #nosec G204
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to generate TLS certificate: %v", err)
	}

	// Create or update Kubernetes secret for revproxy-certs
	cmd = exec.Command("kubectl", "create", "secret", "tls", "revproxy-certs", "--cert="+crtPath, "--key="+keyPath, "-n", namespace, "-o", "yaml", "--dry-run=client") // #nosec G204
	cmdOut, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to prepare revproxy-certs secret: %v", err)
	}

	cmd = exec.Command("kubectl", "apply", "-f", "-") // #nosec G204
	cmd.Stdin = bytes.NewReader(cmdOut)
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to apply revproxy-certs secret: %v", err)
	}

	// Create or update Kubernetes secret for csirevproxy-tls-secret
	cmd = exec.Command("kubectl", "create", "secret", "tls", "csirevproxy-tls-secret", "--cert="+crtPath, "--key="+keyPath, "-n", namespace, "-o", "yaml", "--dry-run=client") // #nosec G204
	cmdOut, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to prepare csirevproxy-tls-secret: %v", err)
	}

	cmd = exec.Command("kubectl", "apply", "-f", "-") // #nosec G204
	cmd.Stdin = bytes.NewReader(cmdOut)
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to apply csirevproxy-tls-secret: %v", err)
	}

	return nil
}
