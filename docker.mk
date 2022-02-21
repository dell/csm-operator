# Registry for all images
REGISTRY ?= dellemc

# Default Operator image name
OPERATOR_IMAGE ?= dell-csm-operator

# Operator version tagged with build number. For e.g. - v1.2.0.001
VERSION ?= 0.1.0

# Timestamp local builds
TIMESTAMP := $(shell  date +%Y%m%d%H%M%S)

# Image URL to use all building/pushing image targets
# Local Image
DEFAULT_IMG := "$(OPERATOR_IMAGE):$(TIMESTAMP)"
IMG ?= "$(REGISTRY)/$(OPERATOR_IMAGE):$(VERSION)"