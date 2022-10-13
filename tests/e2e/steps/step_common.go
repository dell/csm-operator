package steps

import (
	"context"
	"fmt"
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	confv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	"k8s.io/client-go/kubernetes"
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

func checkAllRunningPods(namespace string, k8sClient kubernetes.Interface) error {
	notReadyMessage := ""
	allReady := true

	pods, err := fpod.GetPodsInNamespace(k8sClient, namespace, map[string]string{})
	if err != nil {
		return err
	}
	if len(pods) == 0 {
		return fmt.Errorf("no pod was found in %s", namespace)
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

func checkObservabilityRunningPods(namespace string, k8sClient kubernetes.Interface) error {
	notReadyMessage := ""
	allReady := true

	pods, err := fpod.GetPodsInNamespace(k8sClient, namespace, map[string]string{})
	if err != nil {
		return err
	}
	if len(pods) == 0 {
		return fmt.Errorf("no pod was found in %s", namespace)
	}
	for _, pod := range pods {
		if strings.Contains(pod.Name, "topology") {
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
	}

	if !allReady {
		return fmt.Errorf(notReadyMessage)
	}
	return nil
}

func checkObservabilityNoRunningPods(namespace string, k8sClient kubernetes.Interface) error {
	pods, err := fpod.GetPodsInNamespace(k8sClient, namespace, map[string]string{})
	if err != nil {
		return err
	}

	podsFound := ""
	n := 0
	for _, pod := range pods {
		if strings.Contains(pod.Name, "topology") {
			podsFound += (pod.Name + ",")
			n++
		}
	}
	if n != 0 {
		return fmt.Errorf("found the following pods: %s", podsFound)
	}

	return nil
}

func checkNoRunningPods(namespace string, k8sClient kubernetes.Interface) error {
	pods, err := fpod.GetPodsInNamespace(k8sClient, namespace, map[string]string{})
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

func getApplyDeploymentDaemonSet(cr csmv1.ContainerStorageModule, ctrlClient client.Client) (confv1.DeploymentApplyConfiguration, confv1.DaemonSetApplyConfiguration, error) {
	//cr := res.CustomResource
	dp, err := getDriverDeployment(cr, ctrlClient)
	if err != nil {
		return confv1.DeploymentApplyConfiguration{}, confv1.DaemonSetApplyConfiguration{}, fmt.Errorf("failed to get deployment: %v", err)
	}
	podBuf, err := yaml.Marshal(dp)
	if err != nil {
		return confv1.DeploymentApplyConfiguration{}, confv1.DaemonSetApplyConfiguration{}, fmt.Errorf("failed to get deployment: %v", err)
	}
	var dpApply confv1.DeploymentApplyConfiguration
	err = yaml.Unmarshal(podBuf, &dpApply)
	if err != nil {
		return confv1.DeploymentApplyConfiguration{}, confv1.DaemonSetApplyConfiguration{}, err
	}

	ds, err := getDriverDaemonset(cr, ctrlClient)
	if err != nil {
		return confv1.DeploymentApplyConfiguration{}, confv1.DaemonSetApplyConfiguration{}, fmt.Errorf("failed to get daemonset: %v", err)
	}
	podBuf, err = yaml.Marshal(ds)
	if err != nil {
		return confv1.DeploymentApplyConfiguration{}, confv1.DaemonSetApplyConfiguration{}, fmt.Errorf("failed to get deployment: %v", err)
	}

	var dsApply confv1.DaemonSetApplyConfiguration
	err = yaml.Unmarshal(podBuf, &dsApply)
	if err != nil {
		return confv1.DeploymentApplyConfiguration{}, confv1.DaemonSetApplyConfiguration{}, err
	}

	return dpApply, dsApply, nil
}

func getDriverDeployment(cr csmv1.ContainerStorageModule, ctrlClient client.Client) (*appsv1.Deployment, error) {
	dp := &appsv1.Deployment{}
	if err := ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      fmt.Sprintf("%s-controller", cr.Name)}, dp); err != nil {
		return nil, err
	}

	return dp, nil
}

func getDriverDaemonset(cr csmv1.ContainerStorageModule, ctrlClient client.Client) (*appsv1.DaemonSet, error) {
	ds := &appsv1.DaemonSet{}
	if err := ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      fmt.Sprintf("%s-node", cr.Name)}, ds); err != nil {
		return nil, err
	}

	return ds, nil
}
