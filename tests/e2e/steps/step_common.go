//  Copyright © 2022 Dell Inc. or its subsidiaries. All Rights Reserved.
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

var defaultObservabilityDeploymentName = map[csmv1.DriverType]string{
	csmv1.PowerScaleName: "karavi-metrics-powerscale",
	csmv1.PowerScale:     "karavi-metrics-powerscale",
	csmv1.PowerFlexName:  "karavi-metrics-powerflex",
	csmv1.PowerFlex:      "karavi-metrics-powerflex",
}

// CustomTest -
type CustomTest struct {
	Name string `json:"name" yaml:"name"`
	Run  string `json:"run" yaml:"run"`
}

// Scenario -
type Scenario struct {
	Scenario   string     `json:"scenario" yaml:"scenario"`
	Path       string     `json:"path" yaml:"path"`
	Modules    []string   `json:"modules" yaml:"modules"`
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
