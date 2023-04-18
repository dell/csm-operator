# Registry for all images
REGISTRY ?= docker.io/dellemc

# IMAGE_TAG_BASE defines the docker.io namespace and part of the image name for remote images.
# This variable is used to construct full image tags for bundle and catalog images.
#
# For example, running 'make bundle-build bundle-push catalog-build catalog-push' will build and push both
# dell.com/csm-operator-bundle:$VERSION and dell.com/csm-operator-catalog:$VERSION.
IMAGE_TAG_BASE ?= dell-csm-operator

# Image tag base for community bundle images
BUNDLE_IMAGE_TAG_BASE_COMMUNITY ?= dell-csm-community-operator-bundle

# Image tag base for community catalog images
CATALOG_IMAGE_TAG_BASE_COMMUNITY ?= dell-csm-community-operator-catalog

# Operator version tagged with build number. For e.g. - v1.2.0.001
VERSION ?= v1.2.0

# Bundle Version is the semantic version(required by operator-sdk)
BUNDLE_VERSION ?= 1.2.0

# Timestamp local builds
TIMESTAMP := $(shell  date +%Y%m%d%H%M%S)

# Image URL to use all building/pushing image targets
# Local Image
ifeq ($(DEFAULT_IMG),)
DEFAULT_IMG ?= "$(IMAGE_TAG_BASE):$(TIMESTAMP)"
endif

# Operator image name
IMG ?= "$(REGISTRY)/$(IMAGE_TAG_BASE):$(VERSION)"

# OLM Images
# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= "$(REGISTRY)/$(BUNDLE_IMAGE_TAG_BASE_COMMUNITY):$(VERSION)"

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v1.2.0).
CATALOG_IMG ?= "$(REGISTRY)/$(CATALOG_IMAGE_TAG_BASE_COMMUNITY):$(VERSION)"
