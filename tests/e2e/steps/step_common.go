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
	"context"
	"fmt"
	"os/exec"
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	confv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	fpod "k8s.io/kubernetes/test/e2e/framework/pod"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

var defaultObservabilityDeploymentName = map[csmv1.DriverType]string{
	csmv1.PowerScaleName: "karavi-metrics-powerscale",
	csmv1.PowerScale:     "karavi-metrics-powerscale",
	csmv1.PowerFlexName:  "karavi-metrics-powerflex",
	csmv1.PowerFlex:      "karavi-metrics-powerflex",
}

// CustomTest -
type CustomTest struct {
	Name string   `json:"name" yaml:"name"`
	Run  []string `json:"run" yaml:"run"`
}

// Scenario -
type Scenario struct {
	Scenario   string     `json:"scenario" yaml:"scenario"`
	Paths      []string   `json:"paths" yaml:"paths"`
	Modules    []string   `json:"modules" yaml:"modules"`
	Steps      []string   `json:"steps" yaml:"steps"`
	CustomTest CustomTest `json:"customTest,omitempty" yaml:"customTest"`
}

// Resource -
type Resource struct {
	Scenario       Scenario
	CustomResource []csmv1.ContainerStorageModule
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
		} else if strings.Contains(pod.Name, "metrics") {
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
		} else if strings.Contains(pod.Name, "otel") {
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

func getObservabilityDeployment(namespace string, driverType csmv1.DriverType, ctrlClient client.Client) (*appsv1.Deployment, error) {
	dp := &appsv1.Deployment{}
	dpName := defaultObservabilityDeploymentName[driverType]

	if err := ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: namespace,
		Name:      dpName}, dp); err != nil {
		return nil, err
	}

	return dp, nil
}

func getApplyObservabilityDeployment(namespace string, driverType csmv1.DriverType, ctrlClient client.Client) (confv1.DeploymentApplyConfiguration, error) {
	dp, err := getObservabilityDeployment(namespace, driverType, ctrlClient)
	if err != nil {
		return confv1.DeploymentApplyConfiguration{}, fmt.Errorf("failed to get deployment: %v", err)
	}

	dpBuf, err := yaml.Marshal(dp)
	if err != nil {
		return confv1.DeploymentApplyConfiguration{}, fmt.Errorf("failed to get deployment: %v", err)
	}

	var dpApply confv1.DeploymentApplyConfiguration
	err = yaml.Unmarshal(dpBuf, &dpApply)
	if err != nil {
		return confv1.DeploymentApplyConfiguration{}, err
	}

	return dpApply, nil
}

func checkAuthorizationProxyServerPods(namespace string, k8sClient kubernetes.Interface) error {
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
		errMsg := ""
		if strings.Contains(pod.Name, "cert-manager") {
			errMsg, allReady = arePodsRunning(pod)
			notReadyMessage += errMsg
		} else if strings.Contains(pod.Name, "cert-manager-cainjector") {
			errMsg, allReady = arePodsRunning(pod)
			notReadyMessage += errMsg
		} else if strings.Contains(pod.Name, "cert-manager-webhook") {
			errMsg, allReady = arePodsRunning(pod)
			notReadyMessage += errMsg
		} else if strings.Contains(pod.Name, "ingress-nginx-controller") {
			errMsg, allReady = arePodsRunning(pod)
			notReadyMessage += errMsg
		} else if strings.Contains(pod.Name, "proxy-server") {
			errMsg, allReady = arePodsRunning(pod)
			notReadyMessage += errMsg
		} else if strings.Contains(pod.Name, "redis-commander") {
			errMsg, allReady = arePodsRunning(pod)
			notReadyMessage += errMsg
		} else if strings.Contains(pod.Name, "redis-primary") {
			errMsg, allReady = arePodsRunning(pod)
			notReadyMessage += errMsg
		} else if strings.Contains(pod.Name, "role-service") {
			errMsg, allReady = arePodsRunning(pod)
			notReadyMessage += errMsg
		} else if strings.Contains(pod.Name, "storage-service") {
			errMsg, allReady = arePodsRunning(pod)
			notReadyMessage += errMsg
		} else if strings.Contains(pod.Name, "tenant-service") {
			errMsg, allReady = arePodsRunning(pod)
			notReadyMessage += errMsg
		}
	}

	if !allReady {
		return fmt.Errorf(notReadyMessage)
	}
	return nil
}

func arePodsRunning(pod *corev1.Pod) (string, bool) {
	notReadyMsg := ""
	allReady := true

	if pod.Status.Phase == corev1.PodRunning {
		for _, cntStat := range pod.Status.ContainerStatuses {
			if cntStat.State.Running == nil {
				allReady = false
				notReadyMsg += fmt.Sprintf("\nThe container(%s) in pod(%s) is %s", cntStat.Name, pod.Name, cntStat.State)
				break
			}
		}
	} else {
		allReady = false
		notReadyMsg += fmt.Sprintf("\nThe pod(%s) is %s", pod.Name, pod.Status.Phase)
	}
	return notReadyMsg, allReady
}

// removeNodelabel clears a node label set by setNodeLabel
func removeNodeLabel(testName, labelName string) error {
	config, err := clientcmd.BuildConfigFromFlags("", "/etc/kubernetes/admin.conf")
	if err != nil {
		return fmt.Errorf("kube config creation failed with %s", err)
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("Clientset creation failed with %s", err)
	}

	// Need empty UpdateOptions for node Update() call
	updateOpts := metav1.UpdateOptions{}

	// Go through all nodes labeled as modified by e2e test and remove both labels to restore nodes to before-test state
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: "e2e-added-" + testName})
	for _, node := range nodes.Items {
		delete(node.ObjectMeta.Labels, labelName)
		delete(node.ObjectMeta.Labels, testName)
		_, err := clientset.CoreV1().Nodes().Update(context.TODO(), &node, updateOpts)
		if err != nil {
			return fmt.Errorf("%s label removal failed with the following error: %s", testName, err)
		}
	}

	return nil
}

// setNodeLabel adds a label to all nodes without it and marks them as modified so they can be reset at the end of the test
func setNodeLabel(testName, labelName, labelValue string) error {
	// Get K8s config
	config, err := clientcmd.BuildConfigFromFlags("", "/etc/kubernetes/admin.conf")
	if err != nil {
		return fmt.Errorf("kube config creation failed with %s", err)
	}

	// create the clientset from K8s config
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("Clientset creation failed with %s", err)
	}

	// Need empty UpdateOptions for node Update() call
	updateOpts := metav1.UpdateOptions{}

	// Get only the nodes that do not already have the label
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: "!" + labelName})
	for _, node := range nodes.Items {
		// Add both the label and a label indicating this node was modified by the e2e test
		node.ObjectMeta.Labels[labelName] = labelValue
		node.ObjectMeta.Labels["e2e-added-"+testName] = ""

		_, err := clientset.CoreV1().Nodes().Update(context.TODO(), &node, updateOpts)
		if err != nil {
			return fmt.Errorf("label update failed with the following error: %s", err)
		}
	}

	return nil
}

func checkAuthorizationProxyServerNoRunningPods(namespace string, k8sClient kubernetes.Interface) error {
	pods, err := fpod.GetPodsInNamespace(k8sClient, namespace, map[string]string{})
	if err != nil {
		return err
	}

	podsFound := ""
	n := 0
	for _, pod := range pods {
		if strings.Contains(pod.Name, "cert-manager") {
			podsFound += (pod.Name + ",")
			n++
		} else if strings.Contains(pod.Name, "cert-manager-cainjector") {
			podsFound += (pod.Name + ",")
			n++
		} else if strings.Contains(pod.Name, "cert-manager-webhook") {
			podsFound += (pod.Name + ",")
			n++
		} else if strings.Contains(pod.Name, "ingress-nginx-controller") {
			podsFound += (pod.Name + ",")
			n++
		} else if strings.Contains(pod.Name, "proxy-server") {
			podsFound += (pod.Name + ",")
			n++
		} else if strings.Contains(pod.Name, "redis-commander") {
			podsFound += (pod.Name + ",")
			n++
		} else if strings.Contains(pod.Name, "redis-primary") {
			podsFound += (pod.Name + ",")
			n++
		} else if strings.Contains(pod.Name, "role-service") {
			podsFound += (pod.Name + ",")
			n++
		} else if strings.Contains(pod.Name, "storage-service") {
			podsFound += (pod.Name + ",")
			n++
		} else if strings.Contains(pod.Name, "tenant-service") {
			podsFound += (pod.Name + ",")
			n++
		}
	}
	if n != 0 {
		return fmt.Errorf("found the following pods: %s", podsFound)
	}

	return nil
}

func getPortContainerizedAuth() (string, error) {
	port := ""
	b, err := exec.Command(
		"kubectl", "get",
		"service", "authorization-ingress-nginx-controller",
		"-n", "authorization",
		"-o", `jsonpath="{.spec.ports[1].nodePort}"`,
	).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get authorization-ingress-nginx-controller port: %s", b)
	}
	port = strings.Replace(string(b), `"`, "", -1)
	return port, nil
}
