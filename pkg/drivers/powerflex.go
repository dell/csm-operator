package drivers

import (
	"context"
	"fmt"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	"github.com/dell/csm-operator/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func PrecheckPowerFlex(ctx context.Context, cr *csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig, ct client.Client) error {

	fmt.Println("Filler function for now")
	fmt.Printf("context is: %v", ctx)
	fmt.Println("")
	fmt.Printf("ContainerStorageModule is: %v", cr)
	fmt.Println("")
	fmt.Printf("operatorConfig is %v", operatorConfig)
	fmt.Println("")
	fmt.Printf("client is: %v", ct)
	return nil

}
