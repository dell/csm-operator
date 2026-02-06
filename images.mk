# Copyright © 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
#
# Dell Technologies, Dell and other trademarks are trademarks of Dell Inc.
# or its subsidiaries. Other trademarks may be trademarks of their respective
# owners.

include overrides.mk
include helper.mk

# Image tag base for community bundle images
BUNDLE_IMAGE_TAG_BASE_COMMUNITY ?= dell-csm-community-operator-bundle

# Image tag base for community catalog images
CATALOG_IMAGE_TAG_BASE_COMMUNITY ?= dell-csm-community-operator-catalog

# Bundle Version is the semantic version(required by operator-sdk)
BUNDLE_VERSION ?= 1.11.1

# Registry where images will be pushed (use by operator-sdk to set the newName)
REGISTRY ?= quay.io/dell/container-storage-modules

# Image tag base (use by operator-sdk to set the newName)
IMAGE_TAG_BASE ?= dell-csm-operator

# Operator Version is the semantic version(required by operator-sdk)
VERSION ?= v1.11.1

# Operator image name
IMG ?= "$(REGISTRY)/$(IMAGE_TAG_BASE):$(VERSION)"

# OLM Images
# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= "$(REGISTRY)/$(BUNDLE_IMAGE_TAG_BASE_COMMUNITY):$(VERSION)"

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v1.11.0).
CATALOG_IMG ?= "$(REGISTRY)/$(CATALOG_IMAGE_TAG_BASE_COMMUNITY):$(VERSION)"

images: download-csm-common vendor gen-semver
	$(eval include csm-common.mk)
	@echo "Building: $(IMAGE_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)"
	$(BUILDER) build --pull $(NOCACHE) -t "$(IMAGE_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)" --build-arg GOIMAGE=$(DEFAULT_GOIMAGE) --build-arg BASEIMAGE=$(CSM_BASEIMAGE) .

images-no-cache:
	@echo "Building with --no-cache ..."
	@make images NOCACHE=--no-cache

push:
	@echo "Pushing: $(IMAGE_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)"
	$(BUILDER) push "$(IMAGE_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)"
