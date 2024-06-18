//  Copyright © 2022-2023 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	csmv1 "github.com/dell/csm-operator/api/v1"

	"github.com/dell/csm-operator/pkg/constants"
	"github.com/dell/csm-operator/pkg/modules"
	"github.com/dell/csm-operator/pkg/utils"
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
	authString             = "karavi-authorization-proxy"
	operatorNamespace      = "dell-csm-operator"
	quotaLimit             = "30000000"
	pflexSecretMap         = map[string]string{"REPLACE_USER": "PFLEX_USER", "REPLACE_PASS": "PFLEX_PASS", "REPLACE_SYSTEMID": "PFLEX_SYSTEMID", "REPLACE_ENDPOINT": "PFLEX_ENDPOINT", "REPLACE_MDM": "PFLEX_MDM", "REPLACE_POOL": "PFLEX_POOL"}
	pflexAuthSecretMap     = map[string]string{"REPLACE_USER": "PFLEX_USER", "REPLACE_SYSTEMID": "PFLEX_SYSTEMID", "REPLACE_ENDPOINT": "PFLEX_AUTH_ENDPOINT", "REPLACE_MDM": "PFLEX_MDM"}
	pscaleSecretMap        = map[string]string{"REPLACE_CLUSTERNAME": "PSCALE_CLUSTER", "REPLACE_USER": "PSCALE_USER", "REPLACE_PASS": "PSCALE_PASS", "REPLACE_ENDPOINT": "PSCALE_ENDPOINT"}
	pscaleAuthSecretMap    = map[string]string{"REPLACE_CLUSTERNAME": "PSCALE_CLUSTER", "REPLACE_USER": "PSCALE_USER", "REPLACE_PASS": "PSCALE_PASS", "REPLACE_AUTH_ENDPOINT": "PSCALE_AUTH_ENDPOINT", "REPLACE_PORT": "PSCALE_AUTH_PORT", "REPLACE_ENDPOINT": "PSCALE_ENDPOINT"}
	pscaleAuthSidecarMap   = map[string]string{"REPLACE_CLUSTERNAME": "PSCALE_CLUSTER", "REPLACE_ENDPOINT": "PSCALE_ENDPOINT", "REPLACE_AUTH_ENDPOINT": "PSCALE_AUTH_ENDPOINT", "REPLACE_PORT": "PSCALE_AUTH_PORT"}
	pflexAuthSidecarMap    = map[string]string{"REPLACE_USER": "PFLEX_USER", "REPLACE_PASS": "PFLEX_PASS", "REPLACE_SYSTEMID": "PFLEX_SYSTEMID", "REPLACE_ENDPOINT": "PFLEX_ENDPOINT", "REPLACE_AUTH_ENDPOINT": "PFLEX_AUTH_ENDPOINT"}
	authSidecarRootCertMap = map[string]string{}
	amConfigMap            = map[string]string{"REPLACE_ALT_BUCKET_NAME": "ALT_BUCKET_NAME", "REPLACE_BUCKET_NAME": "BUCKET_NAME", "REPLACE_S3URL": "BACKEND_STORAGE_URL", "REPLACE_CONTROLLER_IMAGE": "AM_CONTROLLER_IMAGE", "REPLACE_PLUGIN_IMAGE": "AM_PLUGIN_IMAGE"}
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
	b, err := os.ReadFile(valuesFilePath)
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
		customResources := []csmv1.ContainerStorageModule{}
		for _, path := range scene.Paths {
			b, err := os.ReadFile(path)
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
	cr := res.CustomResource[crNum-1]
	crBuff, err := os.ReadFile(res.Scenario.Paths[crNum-1])
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
	oldCr := res.CustomResource[oldCrNum-1]

	newCrNum, _ := strconv.Atoi(newCrNumStr)
	newCr := res.CustomResource[newCrNum-1]

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

func (step *Step) installThirdPartyModule(res Resource, thirdPartyModule string) error {
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
			return fmt.Errorf("Installation of velero %v failed", err1)
		}

		cmd2 := exec.Command("helm", "install", "velero", "vmware-tanzu/velero", "--namespace=velero", "--create-namespace", "-f", "testfiles/application-mobility-templates/velero-values.yaml", "--version=3.0.0")
		err2 := cmd2.Run()
		if err2 != nil {
			return fmt.Errorf("Installation of velero %v failed", err2)
		}
	} else if thirdPartyModule == "wordpress" {

		cmd := exec.Command("kubectl", "get", "ns", "wordpress")
		err := cmd.Run()
		if err != nil {
			cmd = exec.Command("kubectl", "create", "ns", "wordpress")
			err = cmd.Run()
			if err != nil {
				return err
			}
		}

		// create wordpress APP for AM testing, requires pflex driver installed and op-e2e-vxflexos SC created
		cmd2 := exec.Command("kubectl", "apply", "-n", "wordpress", "-k", "testfiles/sample-application")
		err = cmd2.Run()
		if err != nil {
			return err
		}

		// give wp time to setup before we create backup/restores
		fmt.Println("Sleeping 60 seconds to allow WP time to create")
		time.Sleep(60 * time.Second)

	}

	return nil
}

func (step *Step) uninstallThirdPartyModule(res Resource, thirdPartyModule string) error {
	if thirdPartyModule == "cert-manager" {
		cmd := exec.Command("kubectl", "delete", "-f", "testfiles/cert-manager-crds.yaml")
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("cert-manager uninstall failed: %v", err)
		}
	} else if thirdPartyModule == "velero" {
		cmd := exec.Command("helm", "delete", "velero", "--namespace=velero")
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("Uninstallation of velero %v failed", err)
		}
	} else if thirdPartyModule == "wordpress" {
		cmd := exec.Command("kubectl", "delete", "-n", "wordpress", "-k", "testfiles/sample-application")
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("Uninstallation of wordpress %v failed", err)
		}

	}
	return nil
}

func (step *Step) deleteCustomResource(res Resource, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1]
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
	cr := res.CustomResource[crNum-1]
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

func (step *Step) validateDriverInstalled(res Resource, driverName string, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	return checkAllRunningPods(context.TODO(), res.CustomResource[crNum-1].Namespace, step.clientSet)
}

func (step *Step) validateDriverNotInstalled(res Resource, driverName string, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	return checkNoRunningPods(context.TODO(), res.CustomResource[crNum-1].Namespace, step.clientSet)
}

func (step *Step) setNodeLabel(res Resource, label string) error {
	if label == "control-plane" {
		setNodeLabel(label, "node-role.kubernetes.io/control-plane", "")
	} else {
		return fmt.Errorf("Adding node label %s not supported, feel free to add support", label)
	}

	return nil
}

func (step *Step) removeNodeLabel(res Resource, label string) error {
	if label == "control-plane" {
		removeNodeLabel(label, "node-role.kubernetes.io/control-plane")
	} else {
		return fmt.Errorf("Removing node label %s not supported, feel free to add support", label)
	}

	return nil
}

func (step *Step) validateModuleInstalled(res Resource, module string, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1]
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
	cr := res.CustomResource[crNum-1]
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
	fakeReconcile := utils.FakeReconcileCSM{
		Client:    step.ctrlClient,
		K8sClient: step.clientSet,
	}

	_, clusterClients, err := utils.GetDefaultClusters(context.TODO(), cr, &fakeReconcile)
	if err != nil {
		return err
	}
	for _, cluster := range clusterClients {
		// check observability in all clusters
		if err := checkObservabilityRunningPods(context.TODO(), utils.ObservabilityNamespace, cluster.ClusterK8sClient); err != nil {
			return fmt.Errorf("failed to check for observability installation in %s: %v", cluster.ClusterID, err)
		}

		// check observability's authorization
		driverType := cr.Spec.Driver.CSIDriverType
		dpApply, err := getApplyObservabilityDeployment(utils.ObservabilityNamespace, driverType, cluster.ClusterCTRLClient)
		if err != nil {
			return err
		}
		if authorizationEnabled, _ := utils.IsModuleEnabled(context.TODO(), *instance, csmv1.Authorization); authorizationEnabled {
			if err := correctlyAuthInjected(cr, dpApply.Annotations, dpApply.Spec.Template.Spec.Volumes, dpApply.Spec.Template.Spec.Containers); err != nil {
				return fmt.Errorf("failed to check for observability authorization installation in %s: %v", cluster.ClusterID, err)
			}
		} else {
			for _, cnt := range dpApply.Spec.Template.Spec.Containers {
				if *cnt.Name == authString {
					return fmt.Errorf("found observability authorization in deployment: %v, err:%v", dpApply.Name, err)
				}
			}
		}
	}

	return nil
}

func (step *Step) validateObservabilityNotInstalled(cr csmv1.ContainerStorageModule) error {
	// check installation for all replicas
	fakeReconcile := utils.FakeReconcileCSM{
		Client:    step.ctrlClient,
		K8sClient: step.clientSet,
	}

	_, clusterClients, err := utils.GetDefaultClusters(context.TODO(), cr, &fakeReconcile)
	if err != nil {
		return err
	}
	for _, cluster := range clusterClients {
		// check observability is not installed
		if err := checkObservabilityNoRunningPods(context.TODO(), utils.ObservabilityNamespace, cluster.ClusterK8sClient); err != nil {
			return fmt.Errorf("failed observability installation check %s: %v", cluster.ClusterID, err)
		}
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
	fakeReconcile := utils.FakeReconcileCSM{
		Client:    step.ctrlClient,
		K8sClient: step.clientSet,
	}

	_, clusterClients, err := utils.GetDefaultClusters(context.TODO(), cr, &fakeReconcile)
	if err != nil {
		return err
	}
	for _, cluster := range clusterClients {
		// check replication controllers in all clusters
		if err := checkAllRunningPods(context.TODO(), utils.ReplicationControllerNameSpace, cluster.ClusterK8sClient); err != nil {
			return fmt.Errorf("failed to check for  replication controllers installation in %s: %v", cluster.ClusterID, err)
		}

		// check driver deployment in all clusters
		if err := checkAllRunningPods(context.TODO(), cr.Namespace, cluster.ClusterK8sClient); err != nil {
			return fmt.Errorf("failed while check for driver installation in %s: %v", cluster.ClusterID, err)
		}
	}

	return nil
}

func (step *Step) validateReplicationNotInstalled(cr csmv1.ContainerStorageModule) error {
	// check installation for all replicas
	fakeReconcile := utils.FakeReconcileCSM{
		Client:    step.ctrlClient,
		K8sClient: step.clientSet,
	}

	_, clusterClients, err := utils.GetDefaultClusters(context.TODO(), cr, &fakeReconcile)
	if err != nil {
		return err
	}
	for _, cluster := range clusterClients {
		// check replication  controller is not installed
		if err := checkNoRunningPods(context.TODO(), utils.ReplicationControllerNameSpace, cluster.ClusterK8sClient); err != nil {
			return fmt.Errorf("failed replica installation check %s: %v", cluster.ClusterID, err)
		}

		// check no driver is not installed in target clusters
		if cluster.ClusterID != utils.DefaultSourceClusterID {
			if err := checkNoRunningPods(context.TODO(), cr.Namespace, cluster.ClusterK8sClient); err != nil {
				return fmt.Errorf("failed replica installation check %s: %v", cluster.ClusterID, err)
			}
		}

	}

	// check that replication sidecar is not in source cluster
	dp, err := getDriverDeployment(cr, step.ctrlClient)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %v", err)
	}
	for _, cnt := range dp.Spec.Template.Spec.Containers {
		if cnt.Name == utils.ReplicationSideCarName {
			return fmt.Errorf("found %s: %v", utils.ReplicationSideCarName, err)
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

func (step *Step) setUpStorageClass(res Resource, scName, templateFile, crType string) error {
	// find which map to use for secret values
	mapValues, err := determineMap(crType)
	if err != nil {
		return err
	}

	for key := range mapValues {
		err := replaceInFile(key, os.Getenv(mapValues[key]), templateFile)
		if err != nil {
			return err
		}
	}

	cmd := exec.Command("kubectl", "get", "sc", scName)
	err = cmd.Run()
	if err == nil {
		cmd = exec.Command("kubectl", "delete", "sc", scName)
		err = cmd.Run()
		if err != nil {
			return err
		}
	}
	cmd = exec.Command("kubectl", "create", "-f", templateFile)
	err = cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func (step *Step) setupSecretFromFile(res Resource, file, namespace string) error {
	crBuff, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read secret data: %v", err)
	}

	if _, err := kubectl.RunKubectlInput(namespace, string(crBuff), "apply", "--validate=true", "-f", "-"); err != nil {
		return fmt.Errorf("failed to apply secret from file %s in namespace %s: %v", file, namespace, err)
	}

	return nil
}

func (step *Step) setUpSecret(res Resource, templateFile, name, namespace, crType string) error {
	// find which map to use for secret values
	mapValues, err := determineMap(crType)
	if err != nil {
		return err
	}

	for key := range mapValues {
		err := replaceInFile(key, os.Getenv(mapValues[key]), templateFile)
		if err != nil {
			return err
		}
	}

	// if secret exists- delete it
	if secretExists(namespace, name) {
		cmd := exec.Command("kubectl", "delete", "secret", "-n", namespace, name)
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("failed to delete secret: %s", err.Error())
		}
	}

	// create new secret
	fileArg := "--from-file=config=" + templateFile
	cmd := exec.Command("kubectl", "create", "secret", "generic", "-n", namespace, name, fileArg)
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to create secret with template file: %s:  %s", templateFile, err.Error())
	}

	return nil
}

func (step *Step) restoreTemplate(res Resource, templateFile, crType string) error {
	mapValues, err := determineMap(crType)
	if err != nil {
		return err
	}

	for key := range mapValues {
		err := replaceInFile(os.Getenv(mapValues[key]), key, templateFile)
		if err != nil {
			return err
		}
	}
	return nil
}

func determineMap(crType string) (map[string]string, error) {
	mapValues := map[string]string{}
	if crType == "pflex" {
		mapValues = pflexSecretMap
	} else if crType == "pflexAuth" {
		mapValues = pflexAuthSecretMap
	} else if crType == "pscale" {
		mapValues = pscaleSecretMap
	} else if crType == "pscaleAuth" {
		mapValues = pscaleAuthSecretMap
	} else if crType == "pscaleAuthSidecar" {
		mapValues = pscaleAuthSidecarMap
	} else if crType == "pflexAuthSidecar" {
		mapValues = pflexAuthSidecarMap
	} else if crType == "authSidecarCert" {
		mapValues = authSidecarRootCertMap
	} else if crType == "application-mobility" {
		mapValues = amConfigMap
	} else {
		return mapValues, fmt.Errorf("type: %s is not supported", crType)
	}

	return mapValues, nil
}

func secretExists(namespace, name string) bool {
	cmd := exec.Command("kubectl", "get", "secret", "-n", namespace, name)
	err := cmd.Run()
	if err != nil {
		return false
	}
	return true
}

func replaceInFile(old, new, templateFile string) error {
	cmdString := "s/" + old + "/" + new + "/g"
	cmd := exec.Command("sed", "-i", cmdString, templateFile)
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

	for testNum, customTest := range res.Scenario.CustomTest.Run {
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

func (step *Step) enableModule(res Resource, module string, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1]
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
	cr := res.CustomResource[crNum-1]
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
	cr := res.CustomResource[crNum-1]
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
	cr := res.CustomResource[crNum-1]
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found,
	); err != nil {
		return err
	}

	found.Spec.Driver.ForceRemoveDriver = true
	return step.ctrlClient.Update(context.TODO(), found)
}

func (step *Step) enableForceRemoveModule(res Resource, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1]
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
		return fmt.Errorf("no pod was found")
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
		return fmt.Errorf(notReadyMessage)
	}

	return nil
}

func (step *Step) createPrereqs(res Resource, module string, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1]

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
	fakeReconcile := utils.FakeReconcileCSM{
		Client:    step.ctrlClient,
		K8sClient: step.clientSet,
	}

	_, clusterClients, err := utils.GetDefaultClusters(context.TODO(), cr, &fakeReconcile)
	if err != nil {
		return err
	}
	for _, cluster := range clusterClients {
		// check AuthorizationProxyServer in all clusters
		if err := checkAuthorizationProxyServerPods(context.TODO(), cr.Namespace, cluster.ClusterK8sClient); err != nil {
			return fmt.Errorf("failed to check for AuthorizationProxyServer installation in %s: %v", cluster.ClusterID, err)
		}
	}

	// provide few seconds for cluster to settle down
	time.Sleep(20 * time.Second)
	return nil
}

func (step *Step) validateAuthorizationProxyServerNotInstalled(cr csmv1.ContainerStorageModule) error {
	// check installation for all AuthorizationProxyServer
	fakeReconcile := utils.FakeReconcileCSM{
		Client:    step.ctrlClient,
		K8sClient: step.clientSet,
	}

	_, clusterClients, err := utils.GetDefaultClusters(context.TODO(), cr, &fakeReconcile)
	if err != nil {
		return err
	}
	for _, cluster := range clusterClients {
		// check AuthorizationProxyServer is not installed
		if err := checkAuthorizationProxyServerNoRunningPods(context.TODO(), cr.Namespace, cluster.ClusterK8sClient); err != nil {
			return fmt.Errorf("failed AuthorizationProxyServer installation check %s: %v", cluster.ClusterID, err)
		}
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

	fakeReconcile := utils.FakeReconcileCSM{
		Client:    step.ctrlClient,
		K8sClient: step.clientSet,
	}

	_, clusterClients, err := utils.GetDefaultClusters(context.TODO(), cr, &fakeReconcile)
	if err != nil {
		return err
	}
	for _, cluster := range clusterClients {
		if err := checkApplicationMobilityPods(context.TODO(), cr.Namespace, cluster.ClusterK8sClient); err != nil {
			return fmt.Errorf("failed to check for App-mob installation in %s: %v", cluster.ClusterID, err)
		}
	}

	// provide few seconds for cluster to settle down
	time.Sleep(10 * time.Second)
	return nil
}

func (step *Step) authProxyServerPrereqs(cr csmv1.ContainerStorageModule) error {
	fmt.Println("=== Creating Authorization Proxy Server Prerequisites ===")

	cmd := exec.Command("kubectl", "get", "ns", cr.Namespace)
	err := cmd.Run()
	if err == nil {
		cmd = exec.Command("kubectl", "delete", "ns", cr.Namespace)
		b, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to delete authorization namespace: %v\nErrMessage:\n%s", err, string(b))
		}
	}

	cmd = exec.Command("kubectl", "create",
		"ns", cr.Namespace,
	)
	b, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create authorization namespace: %v\nErrMessage:\n%s", err, string(b))
	}

	cmd = exec.Command("kubectl", "apply",
		"--validate=false", "-f",
		fmt.Sprintf("https://github.com/jetstack/cert-manager/releases/download/%s/cert-manager.crds.yaml",
			certManagerVersion),
	)
	b, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to apply cert-manager CRDs: %v\nErrMessage:\n%s", err, string(b))
	}

	cmd = exec.Command("kubectl", "create",
		"secret", "generic",
		"karavi-config-secret",
		"-n", cr.Namespace,
		"--from-file=config.yaml=testfiles/authorization-templates/csm_authorization_config.yaml",
	)
	b, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create config secret for JWT: %v\nErrMessage:\n%s", err, string(b))
	}

	cmd = exec.Command("kubectl", "create", "-n", cr.Namespace,
		"-f", "testfiles/authorization-templates/csm_authorization_storage_secret.yaml",
	)
	b, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create storage secret: %v\nErrMessage:\n%s", err, string(b))
	}

	cmd = exec.Command("kubectl", "get", "sc", "local-storage")
	err = cmd.Run()
	if err == nil {
		cmd = exec.Command("kubectl", "delete", "-f", "testfiles/authorization-templates/csm_authorization_local_storage.yaml")
		b, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to delete local storage: %v\nErrMessage:\n%s", err, string(b))
		}
	}

	cmd = exec.Command("kubectl", "create",
		"-f", "testfiles/authorization-templates/csm_authorization_local_storage.yaml",
	)
	b, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create local storage for redis: %v\nErrMessage:\n%s", err, string(b))
	}

	cmd = exec.Command("kubectl", "create", "-n", cr.Namespace,
		"-f", "testfiles/authorization-templates/csm_authorization_certificate.yaml",
	)
	b, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create self-signed cert: %v\nErrMessage:\n%s", err, string(b))
	}

	return nil
}

func (step *Step) configureAuthorizationProxyServer(res Resource, driver string, crNumStr string) error {
	fmt.Println("=== Configuring Authorization Proxy Server ===")

	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1]

	var b []byte
	var err error

	var (
		endpoint        = ""
		sysID           = ""
		user            = ""
		password        = ""
		storageType     = ""
		pool            = ""
		driverNamespace = ""
		proxyHost       = ""
	)

	// by default, use set defined in env file
	endpointvar := "END_POINT"
	systemIdvar := "SYSTEM_ID"
	uservar := "STORAGE_USER"
	passvar := "STORAGE_PASSWORD"
	poolvar := "STORAGE_POOL"

	// if tests are running multiple scenarios that require differently configured auth servers, we will not be able to use one set of vars
	// this section is for powerflex, other drivers can add their sections as required.
	if driver == "powerflex" {
		endpointvar = "PFLEX_ENDPOINT"
		systemIdvar = "PFLEX_SYSTEMID"
		uservar = "PFLEX_USER"
		passvar = "PFLEX_PASS"
		poolvar = "PFLEX_POOL"
		os.Setenv("STORAGE_TYPE", "powerflex")
		os.Setenv("DRIVER_NAMESPACE", "test-vxflexos")
	}

	if driver == "powerscale" {
		os.Setenv("STORAGE_TYPE", "powerscale")
		os.Setenv("DRIVER_NAMESPACE", "isilon")
	}
	// get env variables
	if os.Getenv(endpointvar) != "" {
		endpoint = os.Getenv(endpointvar)
	}
	if os.Getenv(systemIdvar) != "" {
		sysID = os.Getenv(systemIdvar)
	}
	if os.Getenv(uservar) != "" {
		user = os.Getenv(uservar)
	}
	if os.Getenv(passvar) != "" {
		password = os.Getenv(passvar)
	}
	if os.Getenv(poolvar) != "" {
		pool = os.Getenv(poolvar)
	}
	if os.Getenv("STORAGE_TYPE") != "" {
		storageType = os.Getenv("STORAGE_TYPE")
	}
	if os.Getenv("DRIVER_NAMESPACE") != "" {
		driverNamespace = os.Getenv("DRIVER_NAMESPACE")
	}

	proxyHost = os.Getenv("PROXY_HOST")

	port, err := getPortContainerizedAuth(cr.Namespace)
	if err != nil {
		return err
	}

	fmt.Printf("=== Generating Admin Token ===\n")
	adminTkn := exec.Command("karavictl",
		"admin", "token",
		"--name", "Admin",
		"--jwt-signing-secret", "secret",
		"--refresh-token-expiration", fmt.Sprint(30*24*time.Hour),
		"--access-token-expiration", fmt.Sprint(2*time.Hour),
	)
	b, err = adminTkn.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create admin token: %v\nErrMessage:\n%s", err, string(b))
	}

	fmt.Println("=== Writing Admin Token to Tmp File ===\n ")
	err = os.WriteFile("/tmp/adminToken.yaml", b, 0o644)
	if err != nil {
		return fmt.Errorf("failed to write admin token: %v\nErrMessage:\n%s", err, string(b))
	}

	address := proxyHost
	// For v1.9.1 and earlier, use the old address
	configVersion := cr.GetModule(csmv1.AuthorizationServer).ConfigVersion
	isOldVersion, _ := utils.MinVersionCheck(configVersion, "v1.9.1")
	if isOldVersion {
		address = "authorization-ingress-nginx-controller.authorization.svc.cluster.local"
	}

	fmt.Printf("Address: %s\n", address)

	fmt.Println("=== Creating Storage ===\n ")
	cmd := exec.Command("karavictl",
		"--admin-token", "/tmp/adminToken.yaml",
		"storage", "create",
		"--type", storageType,
		"--endpoint", fmt.Sprintf("https://%s", endpoint),
		"--system-id", sysID,
		"--user", user,
		"--password", password,
		"--array-insecure",
		"--insecure", "--addr", fmt.Sprintf("%s:%s", address, port),
	)
	fmt.Println("=== Storage === \n", cmd.String())
	b, err = cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(b), "is already registered") {
		return fmt.Errorf("failed to create storage %s: %v\nErrMessage:\n%s", storageType, err, string(b))
	}

	// Create Tenant
	fmt.Println("=== Creating Tenant ===\n ")
	cmd = exec.Command("karavictl",
		"--admin-token", "/tmp/adminToken.yaml",
		"tenant", "create",
		"-n", tenantName, "--insecure", "--addr", fmt.Sprintf("%s:%s", address, port),
	)
	b, err = cmd.CombinedOutput()
	fmt.Println("=== Tenant === \n", cmd.String())

	if err != nil && !strings.Contains(string(b), "tenant already exists") {
		return fmt.Errorf("failed to create tenant %s: %v\nErrMessage:\n%s", tenantName, err, string(b))
	}

	fmt.Println("=== Creating Role ===\n", cmd.String())
	// Create Role
	if storageType == "powerscale" {
		quotaLimit = "0"
	}
	cmd = exec.Command("karavictl",
		"--admin-token", "/tmp/adminToken.yaml",
		"role", "create",
		fmt.Sprintf("--role=%s=%s=%s=%s=%s",
			roleName, storageType, sysID, pool, quotaLimit),
		"--insecure", "--addr", fmt.Sprintf("%s:%s", address, port),
	)

	fmt.Println("=== Role === \n", cmd.String())
	b, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create role %s: %v\nErrMessage:\n%s", roleName, err, string(b))
	}

	// role creation take few seconds
	time.Sleep(5 * time.Second)

	// Bind role
	cmd = exec.Command("karavictl",
		"--admin-token", "/tmp/adminToken.yaml",
		"rolebinding", "create",
		"--tenant", tenantName,
		"--role", roleName,
		"--insecure", "--addr", fmt.Sprintf("%s:%s", address, port),
	)
	fmt.Println("=== Binding Role ===\n", cmd.String())
	b, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create rolebinding %s: %v\nErrMessage:\n%s", roleName, err, string(b))
	}

	// Generate token
	fmt.Println("=== Generating token ===\n ")
	cmd = exec.Command("karavictl",
		"--admin-token", "/tmp/adminToken.yaml",
		"generate", "token",
		"--tenant", tenantName,
		"--insecure", "--addr", fmt.Sprintf("%s:%s", address, port),
		"--access-token-expiration", fmt.Sprint(10*time.Minute),
	)
	fmt.Println("=== Token ===\n", cmd.String())
	b, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to generate token for %s: %v\nErrMessage:\n%s", tenantName, err, string(b))
	}

	// Apply token to CSI driver host
	fmt.Println("=== Applying token ===\n ")

	err = os.WriteFile("/tmp/token.yaml", b, 0o644)
	if err != nil {
		return fmt.Errorf("failed to write tenant token: %v\nErrMessage:\n%s", err, string(b))
	}

	cmd = exec.Command("kubectl", "apply",
		"-f", "/tmp/token.yaml",
		"-n", driverNamespace,
	)
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

	// validating args & env presence for powerstore & powerscale
	driverType := cr.Spec.Driver.CSIDriverType

	if driverType == csmv1.PowerScaleName || driverType == csmv1.PowerStore || driverType == csmv1.PowerScale {
		if err := modules.CheckApplyContainersResiliency(dpApply.Spec.Template.Spec.Containers, cr); err != nil {
			return err
		}
		if err := modules.CheckApplyContainersResiliency(dsApply.Spec.Template.Spec.Containers, cr); err != nil {
			return err
		}
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
		if cnt.Name == utils.ResiliencySideCarName {
			return fmt.Errorf("found %s: %v", utils.ResiliencySideCarName, err)
		}
	}

	// check that resiliency sidecar(podmon) is not in cluster: for node
	ds, err := getDriverDaemonset(cr, step.ctrlClient)
	if err != nil {
		return fmt.Errorf("failed to get daemonset: %v", err)
	}
	for _, cnt := range ds.Spec.Template.Spec.Containers {
		if cnt.Name == utils.ResiliencySideCarName {
			return fmt.Errorf("found %s: %v", utils.ResiliencySideCarName, err)
		}
	}
	return nil
}

// set up AM CR
func (step *Step) configureAMInstall(res Resource, templateFile string) error {
	mapValues, err := determineMap("application-mobility")
	if err != nil {
		return err
	}

	for key := range mapValues {
		if os.Getenv(mapValues[key]) == "" {
			return fmt.Errorf("env variable %s not set, set in env-e2e-test.sh before continuing", mapValues[key])
		}
		err := replaceInFile(key, os.Getenv(mapValues[key]), templateFile)
		if err != nil {
			return err
		}
	}

	return nil
}

// Steps for Connectivity Client

func (step *Step) validateConnectivityClientInstalled(res Resource, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1]
	found := new(csmv1.ApexConnectivityClient)

	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found); err != nil {
		return err
	}

	return checkAllRunningPods(context.TODO(), res.CustomResource[crNum-1].Namespace, step.clientSet)
}

func (step *Step) validateConnectivityClientNotInstalled(res Resource, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1]
	found := new(csmv1.ApexConnectivityClient)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found); err == nil {
		return fmt.Errorf("Found traces of client installation in namespace %s: %v", cr.Namespace, found)
	}

	return checkNoRunningPods(context.TODO(), res.CustomResource[crNum-1].Namespace, step.clientSet)
}

// uninstallConnectivityClient - uninstall the client
func (step *Step) uninstallConnectivityClient(res Resource, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1]

	found := new(csmv1.ApexConnectivityClient)
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

	crBuff, err := os.ReadFile(res.Scenario.Paths[crNum-1])
	if err != nil {
		return fmt.Errorf("failed to read testdata: %v", err)
	}

	if _, err := kubectl.RunKubectlInput(cr.Namespace, string(crBuff), "delete", "--wait=true", "--timeout=30s", "-f", "-"); err != nil {
		return fmt.Errorf("failed to delete CR %s in namespace %s: %v", cr.Name, cr.Namespace, err)
	}

	return nil
}

func (step *Step) validateApplicationMobilityNotInstalled(cr csmv1.ContainerStorageModule) error {
	fakeReconcile := utils.FakeReconcileCSM{
		Client:    step.ctrlClient,
		K8sClient: step.clientSet,
	}

	_, clusterClients, err := utils.GetDefaultClusters(context.TODO(), cr, &fakeReconcile)
	if err != nil {
		return err
	}
	for _, cluster := range clusterClients {
		err := checkApplicationMobilityPods(context.TODO(), cr.Namespace, cluster.ClusterK8sClient)
		if err == nil {
			return fmt.Errorf("AM pods found in namespace: %s", cr.Namespace)
		}

	}
	fmt.Println("All AM pods removed ")
	return nil
}

func (step *Step) validateRbacCreated(_ Resource, namespace string) error {
	fmt.Println("=== validating Rbac created ===")

	cmd := exec.Command("kubectl", "get", "rolebindings", "-n", "namespacename")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run command")
	}

	roles := strings.Split(out.String(), "\n")
	for _, role := range roles {
		if strings.Contains(role, "Role/connectivity-client-docker-k8s") {
			return nil
		}
	}

	return nil
}
