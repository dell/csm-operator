package main

/*import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	csmv1 "github.com/dell/csm-operator/api/v1"
	k8sClient "github.com/dell/csm-operator/k8s"
	utils "github.com/dell/csm-operator/utils"
	"github.com/dell/csm-operator/utils/drivers"
	"github.com/dell/csm-operator/utils/modules"
	"sigs.k8s.io/yaml"
)

const (
	// K8sMinimumSupportedVersion is the minimum supported version for k8s
	K8sMinimumSupportedVersion = "1.19"
	// K8sMaximumSupportedVersion is the maximum supported version for k8s
	K8sMaximumSupportedVersion = "1.22"
	// OpenshiftMinimumSupportedVersion is the minimum supported version for openshift
	OpenshiftMinimumSupportedVersion = "4.6"
	// OpenshiftMaximumSupportedVersion is the maximum supported version for openshift
	OpenshiftMaximumSupportedVersion = "4.7"
)

func getOperatorConfig() utils.OperatorConfig {
	cfg := utils.OperatorConfig{}

	// Get the environment variSable config dir
	configDir := os.Getenv("X_CSI_OPERATOR_CONFIG_DIR")
	if configDir == "" {
		// Set the config dir to the folder pkg/config
		configDir = "operatorconfig/"
	}
	cfg.ConfigDirectory = configDir

	isOpenShift, err := k8sClient.IsOpenShift()
	if err != nil {
		panic(err.Error())
	}
	cfg.IsOpenShift = isOpenShift

	kubeVersion, err := k8sClient.GetVersion()
	if err != nil {
		panic(err.Error())
	}

	minVersion := 0.0
	maxVersion := 0.0

	if !isOpenShift {
		minVersion, err = strconv.ParseFloat(K8sMinimumSupportedVersion, 64)
		if err != nil {
			panic(err.Error())
		}
		maxVersion, err = strconv.ParseFloat(K8sMaximumSupportedVersion, 64)
		if err != nil {
			panic(err.Error())
		}
	} else {
		minVersion, err = strconv.ParseFloat(OpenshiftMinimumSupportedVersion, 64)
		if err != nil {
			panic(err.Error())
		}
		maxVersion, err = strconv.ParseFloat(OpenshiftMaximumSupportedVersion, 64)
		if err != nil {
			panic(err.Error())
		}
	}

	currentVersion, err := strconv.ParseFloat(kubeVersion, 64)
	if err != nil {
		panic(err.Error())
	}
	if currentVersion < minVersion {
		panic(fmt.Sprintf("version %s is less than minimum supported version of %f", kubeVersion, minVersion))
	}
	if currentVersion > maxVersion {
		panic(fmt.Sprintf("version %s is less than minimum supported version of %f", kubeVersion, minVersion))
	}

	k8sPath := fmt.Sprintf("%s/driverconfig/common/k8s-%s-values.yaml", cfg.ConfigDirectory, kubeVersion)
	buf, err := ioutil.ReadFile(k8sPath)
	if err != nil {
		panic(fmt.Sprintf("reading file, %s, from the configmap mount: %v", k8sPath, err))
	}

	var imageConfig utils.K8sImagesConfig
	err = yaml.Unmarshal(buf, &imageConfig)
	if err != nil {
		panic(fmt.Sprintf("unmarshalling: %v", err))
	}

	cfg.K8sVersion = imageConfig

	return cfg
}

func PrintYAML(in interface{}) {
	buf, err := yaml.Marshal(in)
	if err != nil {
		panic(err.Error())
	}

	fmt.Println(string(buf))
	fmt.Println("---")
}

func main() {
	operatorConfig := getOperatorConfig()

	buf, err := ioutil.ReadFile("samples/storage_v1_csm_powerscale.yaml")
	if err != nil {
		panic(err.Error())
	}

	var cr csmv1.ContainerStorageModule
	err = yaml.Unmarshal(buf, &cr)
	if err != nil {
		panic(fmt.Sprintf("unmarshalling: %v", err))
	}

	cf, err := drivers.GetPowerScaleConfigMap(cr, operatorConfig)
	if err != nil {
		panic(fmt.Sprintf("getting configMap: %v", err))
	}
	PrintYAML(cf)

	ds, err := drivers.GetPowerScaleNode(cr, operatorConfig)
	if err != nil {
		panic(fmt.Sprintf("getting configMap: %v", err))
	}
	PrintYAML(ds.Rbac.ClusterRole)
	PrintYAML(ds.Rbac.ServiceAccount)
	//PrintYAML(ds.DaemonSet)

	dp, err := drivers.GetPowerScaleController(cr, operatorConfig)
	if err != nil {
		panic(fmt.Sprintf("getting configMap: %v", err))
	}
	PrintYAML(dp.Rbac.ClusterRole)
	PrintYAML(dp.Rbac.ServiceAccount)
	//PrintYAML(dp.Deployment)

	dpYaml, err := modules.InjectDeployment(dp.Deployment, cr, operatorConfig)
	if err != nil {
		panic(fmt.Sprintf("getting dp: %v", err))
	}
	PrintYAML(dpYaml)

	dsYaml, err := modules.InjectDeaonset(ds.DaemonSet, cr, operatorConfig)
	if err != nil {
		panic(fmt.Sprintf("getting ds: %v", err))
	}
	PrintYAML(dsYaml)

	// loop through modules
	// get modified dp call specifi module with Deployment, deamonset, configMap

	// print(dp)
	// print(ds)
	// configmap(cf)
}
*/
