#!/bin/bash
#
# Copyright (c) 2023 Dell Inc., or its subsidiaries. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#  http://www.apache.org/licenses/LICENSE-2.0

# bundle CSI driver and module , installation scripts, and
# container images into a tarball that can be used for offline installations via CSM operator

# display some usage information
usage() {
   echo
   echo "$0"
   echo "Make a package for offline installation of a CSI driver and modules"
   echo
   echo "Arguments:"
   echo "-c             Create an offline bundle"
   echo "-p             Prepare this bundle for installation"
   echo "-r <registry>  Required if preparing offline bundle with '-p'"
   echo "               Supply the registry name/path which will hold the images"
   echo "               For example: my.registry.com:5000/dell/csm"
   echo "-h             Displays this information"
   echo
   echo "Exactly one of '-c' or '-p' needs to be specified"
   echo
}

# status
# echos a brief status sttement to stdout
status() {
  echo
  echo "*"
  echo "* $@"
  echo 
}

# run_command
# runs a shell command
# exits and prints stdout/stderr when a non-zero return code occurs
run_command() {
  CMDOUT=$(eval "${@}" 2>&1)
  local rc=$?

  if [ $rc -ne 0 ]; then
    echo
    echo "ERROR"
    echo "Received a non-zero return code ($rc) from the following comand:"
    echo "  ${@}"
    echo
    echo "Output was:"
    echo "${CMDOUT}"
    echo
    echo "Exiting"
    exit 1
  fi
}

# build_image_manifest
# builds a manifest of all the images referred to by the latest operator csv
build_image_manifest() {
  local REGEX="([-_./:A-Za-z0-9]{3,}):([-_.A-Za-z0-9]{1,})"

  status "Building image manifest file"
  if [ -e "${IMAGEFILEDIR}" ]; then
    rm -rf "${IMAGEFILEDIR}"
  fi
  if [ -f "${IMAGEMANIFEST}" ]; then
    rm -rf "${IMAGEMANIFEST}"
  fi

  for F in ${FILES_WITH_IMAGE_NAMES[@]}; do
    echo "   Processing file ${F}"
    if [ ! -f "${F}" ]; then
      echo "Unable to find file, ${F}. Skipping"
    else
      # look for strings that appear to be image names, this will
      # - search all files for strings that make $REGEX
      # - exclude anything with double '//'' as that is a URL and not an image name
      # - make sure at least one '/' is found
      egrep -oh "${REGEX}" ${F} | egrep -v '//' | egrep '/' >> "${IMAGEMANIFEST}.tmp"
    fi
  done

  # sort and uniqify the list
  cat "${IMAGEMANIFEST}.tmp" | sort | uniq > "${IMAGEMANIFEST}"
  rm "${IMAGEMANIFEST}.tmp"
}

# archive_images
# archive the necessary docker images by pulling them locally and then saving them
archive_images() {
  status "Pulling and saving container images"

  if [ ! -d "${IMAGEFILEDIR}" ]; then
    mkdir -p "${IMAGEFILEDIR}"
  fi

  # the images, pull first in case some are not local
  while read line; do
      echo "   $line"
      run_command "${DOCKER}" pull "${line}"
      IMAGEFILE=$(echo "${line}" | sed 's|[/:]|-|g')
      # if we already have the image exported, skip it
      if [ ! -f "${IMAGEFILEDIR}/${IMAGEFILE}.tar" ]; then
        run_command "${DOCKER}" save -o "${IMAGEFILEDIR}/${IMAGEFILE}.tar" "${line}"
      fi
  done < "${IMAGEMANIFEST}"

} 

# restore_images
# load the images from an archive into the local registry
# then push them to the target registry
restore_images() {
  status "Loading docker images"
  find "${IMAGEFILEDIR}" -name \*.tar -exec "${DOCKER}" load -i {} \; 2>/dev/null

  status "Tagging and pushing images"
  while read line; do
      local NEWNAME="${REGISTRY}${line##*/}"
      echo "   $line -> ${NEWNAME}"
      run_command "${DOCKER}" tag "${line}" "${NEWNAME}"
      run_command "${DOCKER}" push "${NEWNAME}"
  done < "${IMAGEMANIFEST}"
}

# copy in any necessary files
copy_files() {
  status "Copying necessary files"
  for f in ${REQUIRED_FILES[@]}; do
    echo " ${f}"
    cp -R "${f}" "${DISTDIR}"
    if [ $? -ne 0 ]; then
      echo "Unable to copy ${f} to the distribution directory"
      exit 1
    fi
  done
}

# fix any references in the operator configuration
fixup_files() {

  ROOTDIR="${REPODIR}"
  status "Preparing files within ${ROOTDIR}"

  # for each image in the manifest, replace the old name with the new
  while read line; do
      local NEWNAME="${REGISTRY}${line##*/}"
      echo "   changing: $line -> ${NEWNAME}"
      find "${ROOTDIR}" -type f -not -path "${SCRIPTDIR}/*" -exec sed -i "s|$line|$NEWNAME|g" {} \;
  done < "${IMAGEMANIFEST}"
}

# compress the whole bundle
compress_bundle() {
  status "Compressing release"
  cd "${DISTBASE}" && tar cvfz "${DISTFILE}" "${DRIVERDIR}"
  if [ $? -ne 0 ]; then
    echo "Unable to package build"
    exit 1
  fi
  rm -rf "${DISTDIR}"
}

#------------------------------------------------------------------------------
#
# Main script logic starts here
#

# default values, overridable by users
CREATE="false"
PREPARE="false"
REGISTRY=""

# some directories
SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
REPODIR="$( dirname "${SCRIPTDIR}" )"

DRIVERNAME="dell-csm-operator"
DISTBASE="${REPODIR}"
DRIVERDIR="${DRIVERNAME}-bundle"
DISTDIR="${DISTBASE}/${DRIVERDIR}"
DISTFILE="${DISTBASE}/${DRIVERDIR}.tar.gz"
IMAGEMANIFEST="${REPODIR}/scripts/images.manifest"
IMAGEFILEDIR="${REPODIR}/scripts/images.tar"


# directories to search all files for image names
FILES_WITH_IMAGE_NAMES=(
  "${REPODIR}/operatorconfig/driverconfig/common/default.yaml"
  "${REPODIR}/bundle/manifests/dell-csm-operator.clusterserviceversion.yaml"
  "${REPODIR}/samples/ocp/*/*.yaml"
)

# list of all files to be included
REQUIRED_FILES=(
  "${REPODIR}/deploy"
  "${REPODIR}/operatorconfig"
  "${REPODIR}/samples"
  "${REPODIR}/scripts"
  "${REPODIR}/*.md"
  "${REPODIR}/LICENSE"
)

while getopts "cpr:h" opt; do
  case $opt in
    c)
      CREATE="true"
      ;;
    p)
      PREPARE="true"
      ;;
    r)
      REGISTRY="${OPTARG}"
      ;;
    h)
      usage
      exit 0
      ;;
    \?)
      echo "Invalid option: -$OPTARG" >&2
      exit 1
      ;;
    :)
      echo "Option -$OPTARG requires an argument." >&2
      exit 1
      ;;
  esac
done

# make sure exatly one option for create/prepare was specified
if [ "${CREATE}" == "${PREPARE}" ]; then
  usage
  exit 1
fi

# validate prepare arguments
if [ "${PREPARE}" == "true" ]; then
  if [ "${REGISTRY}" == "" ]; then
    usage
    exit 1
  fi
fi

if [ "${REGISTRY: -1}" != "/" ]; then
  REGISTRY="${REGISTRY}/"
fi

# figure out if we should use docker or podman, preferring docker
DOCKER=$(which docker 2>/dev/null || which podman 2>/dev/null)   
if [ "${DOCKER}" == "" ]; then
  echo "Unable to find either docker or podman in $PATH"
  exit 1
fi

# create a bundle
if [ "${CREATE}" == "true" ]; then
  if [ -d "${DISTDIR}" ]; then
    rm -rf "${DISTDIR}"
  fi
  if [ ! -d "${DISTDIR}" ]; then
    mkdir -p "${DISTDIR}"
  fi
  if [ -f "${DISTFILE}" ]; then
    rm -f "${DISTFILE}"
  fi
  build_image_manifest
  archive_images
  copy_files
  compress_bundle

  status "Complete"
  echo "Offline bundle file is: ${DISTFILE}"
fi

# prepare a bundle for installation
if [ "${PREPARE}" == "true" ]; then
  echo "Preparing a offline bundle for installation"
  restore_images
  fixup_files

  status "Complete"
fi

echo

exit 0
