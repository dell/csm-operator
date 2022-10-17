package drivers

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/logger"
	"github.com/dell/csm-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// PrecheckPowerFlex do input validation
func PrecheckPowerFlex(ctx context.Context, cr *csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig, ct client.Client) error {

	log := logger.GetLogger(ctx)

	// Check if driver version is supported by doing a stat on a config file
	configFilePath := fmt.Sprintf("%s/driverconfig/%s/%s/upgrade-path.yaml", operatorConfig.ConfigDirectory, csmv1.PowerFlex, cr.Spec.Driver.ConfigVersion)
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		log.Errorw("PreCheckPowerFlex failed in version check", "Error", err.Error())
		return fmt.Errorf("%s %s not supported", csmv1.PowerFlexName, cr.Spec.Driver.ConfigVersion)
	}

	mdmVar, err := GetMDMFromSecret(ctx, cr, ct)
	if err != nil {
		return err
	}
	var newmdm corev1.EnvVar
	for _, initcontainer := range cr.Spec.Driver.InitContainers {
		if initcontainer.Name == "sdc" {
			k := 0
			initenv := initcontainer.Envs
			for c, env := range initenv {
				if env.Name == "MDM" {
					env.Value = mdmVar
					newmdm = env
					k = c
					break
				}
			}
			initenv[k] = newmdm
			break
		}
	}
	return nil
}

// GetMDMFromSecret - Get MDM value from secret
func GetMDMFromSecret(ctx context.Context, cr *csmv1.ContainerStorageModule, ct client.Client) (string, error) {

	log := logger.GetLogger(ctx)
	//secretName := fmt.Sprintf("%s-config", cr.Name)
	secretName := cr.Name + "-config"
	credSecret, err := utils.GetSecret(ctx, secretName, cr.GetNamespace(), ct)
	if err != nil {
		return "", fmt.Errorf("reading secret [%s] error [%s]", secretName, err)
	}

	type StorageArrayConfig struct {
		Username                  string `json:"username"`
		Password                  string `json:"password"`
		SystemID                  string `json:"systemId"`
		Endpoint                  string `json:"endpoint"`
		SkipCertificateValidation bool   `json:"skipCertificateValidation,omitempty"`
		AllSystemNames            string `json:"allSystemNames"`
		IsDefault                 bool   `json:"isDefault,omitempty"`
		MDM                       string `json:"mdm"`
	}

	data := credSecret.Data
	configBytes := data["config"]
	mdmVal := ""
	mdmFin := ""
	ismdmip := false

	if string(configBytes) != "" {
		yamlConfig := make([]StorageArrayConfig, 0)
		configs, _ := yaml.JSONToYAML(configBytes)
		err := yaml.Unmarshal(configs, &yamlConfig)
		if err != nil {
			return "", fmt.Errorf("unable to parse multi-array configuration[%v]", err)
		}

		if len(yamlConfig) == 0 {
			return "", fmt.Errorf("Arrays details are not provided in vxflexos-config secret")
		}

		var noOfDefaultArrays int
		tempMapToFindDuplicates := make(map[string]interface{}, 0)
		for i, config := range yamlConfig {
			if config.SystemID == "" {
				return "", fmt.Errorf("invalid value for ArrayID at index [%d]", i)
			}
			if config.Username == "" {
				return "", fmt.Errorf("invalid value for Username at index [%d]", i)
			}
			if config.Password == "" {
				return "", fmt.Errorf("invalid value for Password at index [%d]", i)
			}
			if config.Endpoint == "" {
				return "", fmt.Errorf("invalid value for RestGateway at index [%d]", i)
			}
			if config.MDM != "" {
				mdmFin, ismdmip = ValidateIPAddress(config.MDM)
				if !ismdmip {
					return "", fmt.Errorf("Invalid MDM value. Ip address should be numeric and comma separated without space")
				}
				if i == 0 {
					mdmVal += mdmFin
				} else {
					mdmVal += "&" + mdmFin
				}
			}
			if config.AllSystemNames != "" {
				names := strings.Split(config.AllSystemNames, ",")
				log.Info("For systemID %s configured System Names found %#v ", config.SystemID, names)
			}

			if _, ok := tempMapToFindDuplicates[config.SystemID]; ok {
				return "", fmt.Errorf("Duplicate ArrayID [%s] found in storageArrayList parameter", config.SystemID)
			}
			tempMapToFindDuplicates[config.SystemID] = nil

			if config.IsDefault {
				noOfDefaultArrays++
			}

			if noOfDefaultArrays > 1 {
				return "", fmt.Errorf("'isDefaultArray' parameter located in multiple places ArrayID: %s. 'isDefaultArray' parameter should present only once in the storageArrayList", config.SystemID)
			}
		}
	} else {
		return "", fmt.Errorf("Arrays details are not provided in vxflexos-config secret")
	}
	fmt.Printf("mdmValFin: %s", mdmVal)
	return mdmVal, nil

}

// ValidateIPAddress -
func ValidateIPAddress(ipAdd string) (string, bool) {
	trimIP := strings.Split(ipAdd, ",")
	if len(trimIP) < 1 {
		return "", false
	}
	newIP := ""
	for i := range trimIP {
		trimIP[i] = strings.TrimSpace(trimIP[i])
		istrueip := IsIpv4Regex(trimIP[i])
		if istrueip {
			newIP = strings.Join(trimIP[:], ",")
		} else {
			return newIP, false
		}
	}
	return newIP, true
}

var (
	ipRegex, _ = regexp.Compile(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)
)

// IsIpv4Regex - Matches Ipaddress with regex
// returns error if the Ip Address doesn't match regex
func IsIpv4Regex(ipAddress string) bool {
	return ipRegex.MatchString(ipAddress)
}
