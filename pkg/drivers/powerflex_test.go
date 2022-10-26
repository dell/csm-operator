package drivers

import (
	"context"
	"fmt"
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1"
	//"github.com/dell/csm-operator/pkg/utils"
	"github.com/dell/csm-operator/tests/shared"
	"github.com/dell/csm-operator/tests/shared/crclient"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	//"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	//"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	pflexCSMName			= "pflex-csm"
	pflexCredsName			= "pflex-csm-config"
	pFlexTestNS			= "pflex-test"
	powerFlexCSM                    = csmForPowerFlex()
	powerFlexCSMBadVersion          = csmForPowerFlexBadVersion()
	powerFlexClient                 = crclient.NewFakeClientNoInjector(objects)
	configJSONFileGood		= fmt.Sprintf("%s/driverconfig/%s/config.json", config.ConfigDirectory, csmv1.PowerFlex)
	configJSONFileBadUser		= fmt.Sprintf("%s/driverconfig/%s/config-empty-username.json", config.ConfigDirectory, csmv1.PowerFlex)
	configJSONFileBadPW		= fmt.Sprintf("%s/driverconfig/%s/config-empty-password.json", config.ConfigDirectory, csmv1.PowerFlex)
	configJSONFileBadSysID		= fmt.Sprintf("%s/driverconfig/%s/config-empty-sysid.json", config.ConfigDirectory, csmv1.PowerFlex)
	configJSONFileBadEndPoint	= fmt.Sprintf("%s/driverconfig/%s/config-empty-endpoint.json", config.ConfigDirectory, csmv1.PowerFlex)
	configJSONFileTwoDefaults	= fmt.Sprintf("%s/driverconfig/%s/config-two-defaults.json", config.ConfigDirectory, csmv1.PowerFlex)
	configJSONFileBadMDM		= fmt.Sprintf("%s/driverconfig/%s/config-invalid-mdm.json", config.ConfigDirectory, csmv1.PowerFlex)
	configJSONFileDuplSysID		= fmt.Sprintf("%s/driverconfig/%s/config-duplicate-sysid.json", config.ConfigDirectory, csmv1.PowerFlex)
	configJSONFileEmpty		= fmt.Sprintf("%s/driverconfig/%s/config-empty.json", config.ConfigDirectory, csmv1.PowerFlex)
	powerFlexSecret                 = shared.MakeSecretWithJSON(pflexCredsName, pFlexTestNS, configJSONFileGood)
	fakeSecret			= shared.MakeSecret("fake-secret", pFlexTestNS, shared.PFlexConfigVersion)

	powerFlexTests = []struct {
		// every single unit test name
		name string
		// csm object
		csm csmv1.ContainerStorageModule
		// client
		ct  client.Client
		// secret
		sec *corev1.Secret
		// expected error
		expectedErr string
	}{
		{"happy path", powerFlexCSM, powerFlexClient, powerFlexSecret, ""},
	}

	preCheckPowerFlexTest = []struct {
		// every single unit test name
		name string
		// csm object
		csm csmv1.ContainerStorageModule
		// client
		ct client.Client
		// secret
		sec *corev1.Secret
		// expected error
		expectedErr string
	}{
		//{"happy path", powerFlexCSM, powerFlexClient, powerFlexSecret, ""},
		{"missing secret", powerFlexCSM, powerFlexClient, fakeSecret, "no secrets found"},
		{"bad version", powerFlexCSMBadVersion, powerFlexClient, powerFlexSecret, "not supported"},
	}

	getMDMFromSecretConfigParseTests = []struct {
		// every single unit test name
		name string
		// csm object
		csm csmv1.ContainerStorageModule
		// client
		ct client.Client
		// secret
		sec *corev1.Secret
		// expected error
		expectedErr string
	}{
		{"bad username", csmForPowerFlexCustom("bad-user"), powerFlexClient, shared.MakeSecretWithJSON("bad-user-config", pFlexTestNS, configJSONFileBadUser), "invalid value for Username"},
		{"bad password", csmForPowerFlexCustom("bad-pw"), powerFlexClient, shared.MakeSecretWithJSON("bad-pw-config", pFlexTestNS, configJSONFileBadPW), "invalid value for Password"},
		{"bad system ID", csmForPowerFlexCustom("bad-sysid"), powerFlexClient, shared.MakeSecretWithJSON("bad-sysid-config", pFlexTestNS, configJSONFileBadSysID), "invalid value for SystemID"},
		{"bad endpoint", csmForPowerFlexCustom("bad-endpt"), powerFlexClient, shared.MakeSecretWithJSON("bad-endpt-config", pFlexTestNS, configJSONFileBadEndPoint), "invalid value for RestGateway"},
		{"two default systems", csmForPowerFlexCustom("two-def-sys"), powerFlexClient, shared.MakeSecretWithJSON("two-def-sys-config", pFlexTestNS, configJSONFileTwoDefaults), "parameter located in multiple places"},
		{"bad mdm ip", csmForPowerFlexCustom("bad-mdm"), powerFlexClient, shared.MakeSecretWithJSON("bad-mdm-config", pFlexTestNS, configJSONFileBadMDM), "Invalid MDM value"},
		{"duplicate system id", csmForPowerFlexCustom("dupl-sysid"), powerFlexClient, shared.MakeSecretWithJSON("dupl-sysid-config", pFlexTestNS, configJSONFileDuplSysID), "Duplicate SystemID"},
		{"empty config", csmForPowerFlexCustom("empty"), powerFlexClient, shared.MakeSecretWithJSON("empty-config", pFlexTestNS, configJSONFileEmpty), "Arrays details are not provided"},
	}
)

func TestPrecheckPowerFlex(t *testing.T) {
	ctx := context.Background()
	for _, tt := range preCheckPowerFlexTest {
		//tt.ct.Create(ctx, tt.sec)
		t.Run(tt.name, func(t *testing.T) {
			err := PrecheckPowerFlex(ctx, &tt.csm, config, tt.ct)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}

	for _, tt := range powerFlexTests {
		tt.ct.Create(ctx, tt.sec)
		t.Run(tt.name, func(t *testing.T) {
			err := PrecheckPowerFlex(ctx, &tt.csm, config, tt.ct)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}
}

func TestGetMDMFromSecretConfigParse(t *testing.T) {
	ctx := context.Background()
	for _, tt := range getMDMFromSecretConfigParseTests {
		tt.ct.Create(ctx, tt.sec)
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetMDMFromSecret(ctx, &tt.csm, tt.ct)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}
}
 
// makes a csm object with tolerations
func csmForPowerFlex() csmv1.ContainerStorageModule {
	res := shared.MakeCSM(pflexCSMName, pFlexTestNS, shared.PFlexConfigVersion)

	// Add log level to cover some code in GetConfigMap
	res.Spec.Driver.AuthSecret = pflexCredsName

	// Add pscale driver version
	res.Spec.Driver.ConfigVersion = shared.PFlexConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerFlex

	return res
}

// makes a csm object with tolerations, custom name and secret
func csmForPowerFlexCustom(customCSMName string) csmv1.ContainerStorageModule {
	res := shared.MakeCSM(customCSMName, pFlexTestNS, shared.PFlexConfigVersion)

	// Add log level to cover some code in GetConfigMap
	res.Spec.Driver.AuthSecret = pflexCredsName

	// Add pscale driver version
	res.Spec.Driver.ConfigVersion = shared.PFlexConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerFlex

	return res
}

// makes a csm object with tolerations
func csmForPowerFlexBadVersion() csmv1.ContainerStorageModule {
	res := shared.MakeCSM(pflexCSMName, pFlexTestNS, shared.PFlexConfigVersion)

	// Add pscale driver version
	res.Spec.Driver.ConfigVersion = shared.BadConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerFlex

	return res
}

