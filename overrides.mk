# Copyright © 2025 Dell Inc. or its subsidiaries. All Rights Reserved.
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

# overrides file
# this file, included from the Makefile, will overlay default values with environment variables
#


# DEFAULT values
DEFAULT_MAJOR = 0
DEFAULT_MINOR = 0
DEFAULT_PATCH = 0
DEFAULT_NOTES =

# Override defaults with environment variables (version components)
MAJOR ?= $(DEFAULT_MAJOR)
MINOR ?= $(DEFAULT_MINOR)
PATCH ?= $(DEFAULT_PATCH)
NOTES ?= $(DEFAULT_NOTES)

# target to print some help regarding these overrides and how to use them
overrides-help:
	@echo
	@echo "The following environment variables can be set to control the build"
	@echo
	@echo "MAJOR   - The major version of image to be built, default is: $(DEFAULT_MAJOR)"
	@echo "              Current setting is: $(MAJOR)"
	@echo "MINOR   - The minor version of image to be built, default is: $(DEFAULT_MINOR)"
	@echo "              Current setting is: $(MINOR)"
	@echo "PATCH   - The patch version of image to be built, defaut is: $(DEFAULT_PATCH)"
	@echo "              Current setting is: $(PATCH)"
	@echo "NOTES   - The release notes of image to be built, default is an empty string."
	@echo "              Current setting is: '$(NOTES)'"
	@echo
