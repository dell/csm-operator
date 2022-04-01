package modules

import (
	"fmt"
	"io/ioutil"
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
