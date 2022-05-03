package steps

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	"github.com/dell/csm-operator/pkg/constants"
	"github.com/dell/csm-operator/pkg/modules"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	confv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	fpod "k8s.io/kubernetes/test/e2e/framework/pod"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// CustomTest -
type CustomTest struct {
	Name string `json:"name" yaml:"name"`
	Run  string `json:"run" yaml:"run"`
}

// Scenario -
type Scenario struct {
	Scenario   string     `json:"scenario" yaml:"scenario"`
	Path       string     `json:"path" yaml:"path"`
	Steps      []string   `json:"steps" yaml:"steps"`
	CustomTest CustomTest `json:"customTest,omitempty" yaml:"customTest"`
}

// Resource -
type Resource struct {
	Scenario       Scenario
	CustomResource csmv1.ContainerStorageModule
}

// Step -
type Step struct {
	ctrlClient client.Client
	clientSet  *kubernetes.Clientset
}

var (
	authString        = "karavi-authorization-proxy"
	operatorNamespace = "dell-csm-operator"
)

// GetTestResources -- parse values file
func GetTestResources(valuesFilePath string) ([]Resource, error) {
	b, err := ioutil.ReadFile(valuesFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read values file: %v", err)
	}

	scenarios := []Scenario{}
	err = yaml.Unmarshal(b, &scenarios)
	if err != nil {
		return nil, fmt.Errorf("failed to read unmarshal values file: %v", err)
	}

	recourses := []Resource{}
	for _, scene := range scenarios {
		b, err := ioutil.ReadFile(scene.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to read testdata: %v", err)
		}

		customResource := csmv1.ContainerStorageModule{}
		err = yaml.Unmarshal(b, &customResource)
		if err != nil {
			return nil, fmt.Errorf("failed to read unmarshal CSM custom resource: %v", err)
		}

		recourses = append(recourses, Resource{
			Scenario:       scene,
			CustomResource: customResource,
		})
	}

	return recourses, nil
}

func (step *Step) applyCustomResource(res Resource) error {
	cr := res.CustomResource
	crBuff, err := ioutil.ReadFile(res.Scenario.Path)
	if err != nil {
		return fmt.Errorf("failed to read testdata: %v", err)
	}

	if _, err := framework.RunKubectlInput(cr.Namespace, string(crBuff), "apply", "--validate=true", "-f", "-"); err != nil {
		return fmt.Errorf("failed to apply CR %s in namespace %s: %v", cr.Name, cr.Namespace, err)
	}

	return nil

}

func (step *Step) deleteCustomResource(res Resource) error {
	cr := res.CustomResource
	found := new(csmv1.ContainerStorageModule)
	err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name}, found,
	)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return step.ctrlClient.Delete(context.TODO(), &cr)
}

func (step *Step) validateCustomResourceStatus(res Resource) error {
	cr := res.CustomResource
	found := new(csmv1.ContainerStorageModule)
	err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name}, found)
	if err != nil {
		return err
	}
	if found.Status.State != constants.Succeeded {
		return fmt.Errorf("expected custom resource status to be %s. Got: %s", constants.Succeeded, found.Status.State)
	}

	return nil
}

func (step *Step) validateDriverInstalled(res Resource, driverType string) error {
	cr := res.CustomResource
	notReadyMessage := ""
	allReady := true
	pods, err := fpod.GetPodsInNamespace(step.clientSet, cr.Namespace, map[string]string{})
	if err != nil {
		return err
	}
	if len(pods) == 0 {
		return fmt.Errorf("no driver pod was found")
	}
	for _, pod := range pods {
		if pod.Status.Phase == corev1.PodRunning {
			for _, cntStat := range pod.Status.ContainerStatuses {
				if cntStat.State.Running == nil {
					allReady = false
					notReadyMessage += fmt.Sprintf("\nThe container(%s) in pod(%s) is %s", cntStat.Name, pod.Name, cntStat.State)
					break
				}
			}
		} else {
			allReady = false
			notReadyMessage += fmt.Sprintf("\nThe pod(%s) is %s", pod.Name, pod.Status.Phase)
		}
	}

	if !allReady {
		return fmt.Errorf(notReadyMessage)
	}
	return nil
}

func (step *Step) validateDriverNotInstalled(res Resource, driverType string) error {
	cr := res.CustomResource
	//ginkgo.By(fmt.Sprintf("validating %s driver is not installed", driverType))
	pods, err := fpod.GetPodsInNamespace(step.clientSet, cr.Namespace, map[string]string{})
	if err != nil {
		return err
	}

	podsFound := ""
	for _, pod := range pods {
		podsFound += (pod.Name + ",")
	}
	if len(pods) != 0 {
		return fmt.Errorf("found the following pods: %s", podsFound)
	}

	return nil
}

func (step *Step) validateModuleInstalled(res Resource, module string) error {
	cr := res.CustomResource
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name}, found,
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
				return step.validateAuthorizationInstalled(res)
			}
		}
	}
	return fmt.Errorf("%s module is not not found", module)
}

func (step *Step) validateModuleNotInstalled(res Resource, module string) error {
	cr := res.CustomResource
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name}, found,
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
				return step.validateAuthorizationNotInstalled(res)
			}
		}
	}

	return nil
}

func (step *Step) validateAuthorizationInstalled(res Resource) error {
	cr := res.CustomResource
	correctlyInjected := func(annotations map[string]string, vols []acorev1.VolumeApplyConfiguration, cnt []acorev1.ContainerApplyConfiguration) error {
		err := modules.CheckAnnotationAuth(annotations)
		if err != nil {
			return err
		}

		err = modules.CheckApplyVolumesAuth(vols)
		if err != nil {
			return err
		}
		err = modules.CheckApplyContainersAuth(cnt, string(cr.Spec.Driver.CSIDriverType))
		if err != nil {
			return err
		}
		return nil
	}

	dp := &appsv1.Deployment{}
	err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      fmt.Sprintf("%s-controller", cr.Name)}, dp)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %v", err)
	}

	podBuf, err := yaml.Marshal(dp)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %v", err)
	}

	var dpApply confv1.DeploymentApplyConfiguration
	err = yaml.Unmarshal(podBuf, &dpApply)
	if err != nil {
		return err
	}

	if err := correctlyInjected(dp.Annotations, dpApply.Spec.Template.Spec.Volumes, dpApply.Spec.Template.Spec.Containers); err != nil {
		return err
	}

	ds := &appsv1.DaemonSet{}
	err = step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      fmt.Sprintf("%s-node", cr.Name)}, ds)
	if err != nil {
		return fmt.Errorf("failed to get daemonset: %v", err)
	}

	podBuf, err = yaml.Marshal(ds)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %v", err)
	}

	var dsApply confv1.DaemonSetApplyConfiguration
	err = yaml.Unmarshal(podBuf, &dsApply)
	if err != nil {
		return err
	}

	return correctlyInjected(ds.Annotations, dsApply.Spec.Template.Spec.Volumes, dsApply.Spec.Template.Spec.Containers)
}

func (step *Step) validateAuthorizationNotInstalled(res Resource) error {
	cr := res.CustomResource
	dp := &appsv1.Deployment{}
	err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      fmt.Sprintf("%s-controller", cr.Name)}, dp)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %v", err)
	}

	for _, cnt := range dp.Spec.Template.Spec.Containers {
		if cnt.Name == authString {
			return fmt.Errorf("found authorization in deployment: %v", err)
		}

	}

	ds := &appsv1.DaemonSet{}
	err = step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      fmt.Sprintf("%s-node", cr.Name)}, ds)
	if err != nil {
		return fmt.Errorf("failed to get daemonset: %v", err)
	}

	for _, cnt := range ds.Spec.Template.Spec.Containers {
		if cnt.Name == authString {
			return fmt.Errorf("found authorization in deployment: %v", err)
		}

	}

	return nil
}

func (step *Step) runCustomTest(res Resource) error {
	var (
		stdout string
		stderr string
		err    error
	)

	args := strings.Split(res.Scenario.CustomTest.Run, " ")
	if len(args) == 1 {
		stdout, stderr, err = framework.RunCmd(args[0])

	} else {
		stdout, stderr, err = framework.RunCmd(args[0], args[1:]...)
	}

	if err != nil {
		return fmt.Errorf("error running customs test. Error: %v \n stdout: %s \n stderr: %s", err, stdout, stderr)
	}
	return nil
}

func (step *Step) enableModule(res Resource, module string) error {
	cr := res.CustomResource
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name}, found,
	); err != nil {
		return err
	}

	for i, m := range found.Spec.Modules {
		if !m.Enabled && m.Name == csmv1.ModuleType(module) {
			found.Spec.Modules[i].Enabled = true
		}
	}

	return step.ctrlClient.Update(context.TODO(), found)
}

func (step *Step) setDriverSecret(res Resource, driverSecretName string) error {
	cr := res.CustomResource
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name}, found,
	); err != nil {
		return err
	}
	found.Spec.Driver.AuthSecret = driverSecretName
	return step.ctrlClient.Update(context.TODO(), found)
}

func (step *Step) disableModule(res Resource, module string) error {
	cr := res.CustomResource
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name}, found,
	); err != nil {
		return err
	}

	for i, m := range found.Spec.Modules {
		if m.Enabled && m.Name == csmv1.ModuleType(module) {
			found.Spec.Modules[i].Enabled = false
		}
	}

	return step.ctrlClient.Update(context.TODO(), found)
}

func (step *Step) enableForceRemoveDriver(res Resource) error {
	cr := res.CustomResource
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name}, found,
	); err != nil {
		return err
	}

	found.Spec.Driver.ForceRemoveDriver = true
	return step.ctrlClient.Update(context.TODO(), found)
}

func (step *Step) validateTestEnvironment(_ Resource) error {
	if os.Getenv("OPERATOR_NAMESPACE") != "" {
		operatorNamespace = os.Getenv("OPERATOR_NAMESPACE")
	}

	pods, err := fpod.GetPodsInNamespace(step.clientSet, operatorNamespace, map[string]string{})
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
