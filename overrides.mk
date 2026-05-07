# Copyright © 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#      http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

IMAGE_REGISTRY?="sample_registry"
IMAGE_NAME="dell-csm-operator"
IMAGE_TAG?=$(shell date +%Y%m%d%H%M%S)

# figure out if podman or docker should be used (use podman if found)
ifneq (, $(shell which podman 2>/dev/null))
export BUILDER=podman
else
export BUILDER=docker
endif

# target to print some help regarding these overrides and how to use them
overrides-help:
	@echo
	@echo "The following environment variables can be set to control the build"
	@echo
	@echo "IMAGE_REGISTRY  - The registry to push images to."
	@echo "                  Current setting is: $(IMAGE_REGISTRY)"
	@echo "IMAGE_NAME      - The image name to be built."
	@echo "                  Current setting is: $(IMAGE_NAME)"
	@echo "IMAGE_TAG       - The image tag to be built, default is the current date."
	@echo "                  Current setting is: $(IMAGE_TAG)"
	@echo
