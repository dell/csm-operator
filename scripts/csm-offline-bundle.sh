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
   echo "-t             Test mode: build image manifest and display without pulling"
   echo "-r <registry>  Required if preparing offline bundle with '-p'"
   echo "               Supply the registry name/path which will hold the images"
   echo "               For example: my.registry.com:5000/dell/csm"
   echo "-o <config>    Optional override config file for image overrides"
   echo "               Each line: original_image=replacement_image"
   echo "-h             Displays this information"
   echo
   echo "Exactly one of '-c', '-p', or '-t' needs to be specified"
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

  # Find the newest configmap file dynamically
  find_newest_configmap

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

  # filter to keep only highest semantic versions
  filter_manifest_by_semver

  # apply image overrides if config provided
  apply_overrides
}

# apply_overrides
# if an override config file is provided, replace matching images in the manifest
# Format: image_name=full_registry_path:tag (clean format - just image name on left)
apply_overrides() {
  if [ -z "${OVERRIDE_CONFIG}" ] || [ ! -f "${OVERRIDE_CONFIG}" ]; then
    return
  fi

  status "Applying image overrides from ${OVERRIDE_CONFIG}"

  local tmpfile="${IMAGEMANIFEST}.ovr"
  cp "${IMAGEMANIFEST}" "${tmpfile}"

  while IFS= read -r line; do
    # skip comments and blank lines
    [[ "${line}" =~ ^[[:space:]]*$ ]] && continue
    [[ "${line}" =~ ^[[:space:]]*# ]] && continue

    local src="${line%%=*}"
    local dst="${line#*=}"
    src=$(echo "${src}" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
    dst=$(echo "${dst}" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')

    # Find and replace images matching the image name (source)
    while IFS= read -r image_line; do
      # Extract image name from full image path (last part after last slash)
      local image_name="${image_line##*/}"
      local image_name_base="${image_name%%:*}"
      
      if [[ "${image_name_base}" = "${src}" ]]; then
        echo "   ${image_line} -> ${dst}"
        sed -i "s|${image_line}|${dst}|g" "${tmpfile}"
      fi
    done < "${IMAGEMANIFEST}"
  done < "${OVERRIDE_CONFIG}"

  mv "${tmpfile}" "${IMAGEMANIFEST}"
  
  # Final deduplication after overrides
  sort "${IMAGEMANIFEST}" | uniq > "${IMAGEMANIFEST}.tmp"
  mv "${IMAGEMANIFEST}.tmp" "${IMAGEMANIFEST}"
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

  # Create a temporary file with original images before overrides were applied
  local original_manifest="${IMAGEMANIFEST}.orig"
  
  # Build manifest without overrides to get original images
  local temp_override="${OVERRIDE_CONFIG}"
  OVERRIDE_CONFIG=""
  build_image_manifest
  cp "${IMAGEMANIFEST}" "${original_manifest}"
  
  # Restore overrides and rebuild manifest
  OVERRIDE_CONFIG="${temp_override}"
  build_image_manifest
  
  # Process each image in the overridden manifest
  while IFS= read -r overridden; do
    # Extract image name from overridden image (last part after last slash, before tag)
    local image_name="${overridden##*/}"
    local image_name_base="${image_name%%:*}"
    
    # Find the corresponding original image by matching the image name base
    # For images with multiple versions, replace all versions that match the base name
    local found_originals=()
    while IFS= read -r orig_line; do
      local orig_image_name="${orig_line##*/}"
      local orig_image_name_base="${orig_image_name%%:*}"
      if [[ "${orig_image_name_base}" = "${image_name_base}" ]]; then
        found_originals+=("${orig_line}")
      fi
    done < "${original_manifest}"
    
    # Replace all matching original images with the target registry
    if [ ${#found_originals[@]} -gt 0 ]; then
      local NEWNAME="${REGISTRY}${overridden##*/}"
      for original in "${found_originals[@]}"; do
        echo "   changing: $original -> ${NEWNAME}"
        find "${ROOTDIR}" -type f -name "*.yaml" -exec sed -i "s|$original|$NEWNAME|g" {} \;
      done
    fi
  done < "${IMAGEMANIFEST}"
  
    
  # Clean up
  rm -f "${original_manifest}"
}

# compress the whole bundle
compress_bundle() {
  # Compresses the distribution directory into a tarball for offline installation
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
TEST="false"
REGISTRY=""
OVERRIDE_CONFIG=""

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
)

# Find the newest k8s_configmap.yaml version dynamically
find_newest_configmap() {
  local configmap_files=($(find "${REPODIR}/samples" -name "k8s_configmap.yaml" | sort -V))
  if [ ${#configmap_files[@]} -gt 0 ]; then
    # Get the last file (newest version) from the sorted list
    local newest_configmap="${configmap_files[-1]}"
    # Check if the configmap is already in the array
    local already_exists=0
    for file in "${FILES_WITH_IMAGE_NAMES[@]}"; do
      if [[ "$file" == "$newest_configmap" ]]; then
        already_exists=1
        break
      fi
    done
    if [[ $already_exists -eq 0 ]]; then
      FILES_WITH_IMAGE_NAMES+=("$newest_configmap")
      echo "   Found newest configmap: $newest_configmap"
    else
      echo "   Configmap already in list: $newest_configmap"
    fi
  else
    echo "   Warning: No k8s_configmap.yaml files found"
  fi
}

# filter_manifest_by_semver
# Filter manifest to keep only the highest semantic version for each image name
filter_manifest_by_semver() {
  local temp_manifest="${IMAGEMANIFEST}.filtered"
  local temp_dir=$(mktemp -d)
  
  # Create associative arrays to track highest versions
  declare -A highest_versions
  declare -A full_image_paths
  
  echo "   Filtering manifest to keep highest semantic versions..."
  
  # Process each image in the manifest
  while IFS= read -r image_line; do
    # Skip empty lines
    [[ -z "${image_line}" ]] && continue
    
    # Extract image name (last part after last slash, before tag)
    local image_name="${image_line##*/}"
    local image_name_base="${image_name%%:*}"
    local tag="${image_name##*:}"
    
    # Skip non-semver tags (like "latest", "nightly", or complex tags)
    if [[ ! "${tag}" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
      # Keep non-semver images as-is
      echo "${image_line}" >> "${temp_manifest}.nonsemver"
      continue
    fi
    
    # Store version information for comparison
    local key="${image_name_base}"
    local version="${tag#v}"  # Remove 'v' prefix for comparison (if present)
    
    # Create a temporary file for version comparison
    echo "${version}" > "${temp_dir}/${key}_current"
    
    if [[ -z "${highest_versions[${key}]}" ]]; then
      # First occurrence of this image name
      highest_versions["${key}"]="${version}"
      full_image_paths["${key}"]="${image_line}"
      echo "${version}" > "${temp_dir}/${key}_highest"
    else
      # Compare versions
      echo "${highest_versions[${key}]}" > "${temp_dir}/${key}_highest"
      
      # Use sort -V to compare semantic versions
      local higher_version=$(sort -V "${temp_dir}/${key}_highest" "${temp_dir}/${key}_current" | tail -1)
      
      if [[ "${higher_version}" == "${version}" ]]; then
        # Current version is higher, update
        highest_versions["${key}"]="${version}"
        full_image_paths["${key}"]="${image_line}"
      fi
    fi
  done < "${IMAGEMANIFEST}"
  
  # Output the filtered results
  > "${temp_manifest}"
  
  # Add non-semver images first
  if [[ -f "${temp_manifest}.nonsemver" ]]; then
    cat "${temp_manifest}.nonsemver" >> "${temp_manifest}"
  fi
  
  # Add highest semver versions
  for key in "${!full_image_paths[@]}"; do
    echo "${full_image_paths[${key}]}" >> "${temp_manifest}"
  done
  
  # Sort the final manifest
  sort "${temp_manifest}" | uniq > "${IMAGEMANIFEST}.tmp"
  mv "${IMAGEMANIFEST}.tmp" "${temp_manifest}"
  
  # Replace original manifest
  mv "${temp_manifest}" "${IMAGEMANIFEST}"
  
  # Cleanup
  rm -rf "${temp_dir}"
  rm -f "${temp_manifest}.nonsemver"
  
  echo "   Manifest filtered to $(wc -l < "${IMAGEMANIFEST}") unique highest versions"
}

# list of all files to be included
REQUIRED_FILES=(
  "${REPODIR}/deploy"
  "${REPODIR}/config"
  "${REPODIR}/bundle"
  "${REPODIR}/operatorconfig"
  "${REPODIR}/samples"
  "${REPODIR}/scripts"
  "${REPODIR}/*.md"
  "${REPODIR}/LICENSE"
)

while getopts "cptr:o:h" opt; do
  case $opt in
    c)
      CREATE="true"
      ;;
    p)
      PREPARE="true"
      ;;
    t)
      TEST="true"
      ;;
    r)
      REGISTRY="${OPTARG}"
      ;;
    o)
      OVERRIDE_CONFIG="${OPTARG}"
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

# make sure exactly one option for create/prepare/test was specified
mode_count=0
[ "${CREATE}" == "true" ] && ((mode_count++))
[ "${PREPARE}" == "true" ] && ((mode_count++))
[ "${TEST}" == "true" ] && ((mode_count++))
if [ ${mode_count} -ne 1 ]; then
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
  
  # Verify DISTDIR exists and has content before compressing
  if [ ! -d "${DISTDIR}" ] || [ -z "$(ls -A ${DISTDIR})" ]; then
    echo "Error: Distribution directory ${DISTDIR} is empty or does not exist after copy_files"
    exit 1
  fi
  
  # Remove existing tarball if it exists to ensure fresh creation
  if [ -f "${DISTFILE}" ]; then
    rm -f "${DISTFILE}"
  fi
  
  compress_bundle

  status "Complete"
  echo "Offline bundle file is: ${DISTFILE}"
fi

# test mode: build image manifest and display
if [ "${TEST}" == "true" ]; then
  build_image_manifest

  status "Test mode complete"
  echo "These images would be pulled:"
  wc -l < "${IMAGEMANIFEST}" | awk '{print "  " $1 " images"}'
  echo "Manifest saved to: ${IMAGEMANIFEST}"
fi

# prepare a bundle for installation
if [ "${PREPARE}" == "true" ]; then
  echo "Preparing a offline bundle for installation"
  
  # create bundle directory if it doesn't exist
  if [ ! -d "${DISTDIR}" ]; then
    mkdir -p "${DISTDIR}"
  fi
  
  restore_images
  fixup_files

  status "Complete"
fi

echo

exit 0
