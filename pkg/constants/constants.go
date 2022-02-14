package constants

import (
	"time"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Constants for driver states etc
const (
	RetryCount           = 3
	Running              = csmv1.CSMStateType("Running")
	Succeeded            = csmv1.CSMStateType("Succeeded")
	Creating             = csmv1.CSMStateType("Creating")
	Failed               = csmv1.CSMStateType("Failed")
	InvalidConfig        = csmv1.CSMStateType("InvalidConfig")
	NoState              = csmv1.CSMStateType("")
	Updating             = csmv1.CSMStateType("Updating")
	DefaultRetryInterval = 5 * time.Second
	MaxRetryInterval     = 10 * time.Minute
	MaxRetryDuration     = 30 * time.Minute
)

// DriverReplicas - Replica count for controller
var DriverReplicas = int32(1)

// RevisionHistoryLimit - Max revision history limit for driver daemonset
var RevisionHistoryLimit = int32(10)

// MaxUnavailableUpdateStrategy - Maximum unavailable update strategy
var MaxUnavailableUpdateStrategy = intstr.IntOrString{IntVal: 1, StrVal: "1"}

// TerminationMessagePath for the container
const TerminationMessagePath = "/dev/termination-log"

// TerminationMessagePolicy determines the policy for termination message
const TerminationMessagePolicy = "File"

// DriverMountPath - Mount path for the driver container
const DriverMountPath = "/var/run/csi"

// DriverMountName - Socket directory volume mount name
const DriverMountName = "socket-dir"

// TerminationGracePeriodSeconds - grace period in seconds
var TerminationGracePeriodSeconds = int64(30)

// Reason - pod status
var Reason = "Reason"

// ContainerCreating - pod container
var ContainerCreating = "ContainerCreating"

// PendingCreate - pod pending
var PendingCreate = "Pending create"
