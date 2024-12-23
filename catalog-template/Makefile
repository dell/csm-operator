# This Makefile provides a set of targets to generate and validate operator catalogs
# using the Operator Package Manager (opm) tool.

# A user can customize "catalog" target to generate the operator catalog in a way
# that suits the operator.
# CSM Operator uses basic template to generate basic catalog

PDW=$(shell pwd)
TOPDIR=$(abspath $(dir $(PWD))/../)
BINDIR=${TOPDIR}/bin

# Add the bin directory to the PATH
export PATH := $(BINDIR):$(PATH)

# Generate your catalog.yaml
.PHONY: catalogs
catalogs: basic

# basic target provides an example FBC generation from a `basic` template type.
# this example takes a single file as input and generates a well-formed FBC operator contribution as an output
.PHONY: basic
basic: ${BINDIR}/opm clean
	${BINDIR}/opm alpha render-template basic -o yaml ${TOPDIR}/csm-operator/catalog-template/csm-operator-template.yaml > ${TOPDIR}/csm-operator/catalog/csm-operator/catalog.yaml; 

# validate-catalogs target illustrates FBC validation
# all FBC must pass opm validation in order to be able to be used in a catalog
.PHONY: validate-catalogs
validate-catalogs: ${BINDIR}/opm
	${BINDIR}/opm validate ${TOPDIR}/csm-operator/catalog/csm-operator/ && echo "catalog validation passed" || echo "catalog validation failed";

.PHONY: clean
clean: 
	rm -f ${TOPDIR}/csm-operator/catalog/csm-operator/catalog.yaml


OS=$(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(shell uname -m | sed 's/x86_64/amd64/')

# Automatically download the opm binary
OPM_VERSION ?= v1.46.0
${BINDIR}/opm:
	if [ ! -d ${BINDIR} ]; then mkdir -p ${BINDIR}; fi
	curl -sLO https://github.com/operator-framework/operator-registry/releases/download/$(OPM_VERSION)/$(OS)-$(ARCH)-opm && chmod +x $(OS)-$(ARCH)-opm && mv $(OS)-$(ARCH)-opm ${BINDIR}/opm

