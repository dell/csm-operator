package modules

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	utils "github.com/dell/csm-operator/pkg/utils"
)

const (
	// DefaultPluginIdentifier - spring placeholder for driver plugin
	DefaultPluginIdentifier = "<DriverPluginIdentifier>"
	// DefaultDriverConfigParamsVolumeMount - string placeholder for Driver ConfigParamsVolumeMount
	DefaultDriverConfigParamsVolumeMount = "<DriverConfigParamsVolumeMount>"
)

// SupportedDriverParam -
type SupportedDriverParam struct {
	PluginIdentifier              string
	DriverConfigParamsVolumeMount string
}

func checkVersion(moduleType, givenVersion, configPath string) error {
	files, err := ioutil.ReadDir(fmt.Sprintf("%s/moduleconfig/%s/", configPath, moduleType))
	if err != nil {
		return err
	}
	found := false
	supportedVersions := ""
	for _, file := range files {
		supportedVersions += (file.Name() + ",")
		if file.Name() == givenVersion {
			found = true
		}
	}
	if !found {
		return fmt.Errorf(
			"CSM %s does not have %s version. The following are supported versions: %s",
			moduleType, givenVersion, supportedVersions[:len(supportedVersions)-1],
		)
	}
	return nil
}

func readConfigFile(module csmv1.Module, cr csmv1.ContainerStorageModule, op utils.OperatorConfig, filename string) ([]byte, error) {
	var err error
	moduleConfigVersion := module.ConfigVersion
	if moduleConfigVersion == "" {
		moduleConfigVersion, err = utils.GetModuleDefaultVersion(cr.Spec.Driver.ConfigVersion, cr.Spec.Driver.CSIDriverType, module.Name, op.ConfigDirectory)
		if err != nil {
			return nil, err
		}
	}

	configMapPath := fmt.Sprintf("%s/moduleconfig/%s/%s/%s", op.ConfigDirectory, module.Name, moduleConfigVersion, filename)
	return ioutil.ReadFile(filepath.Clean(configMapPath))
}
