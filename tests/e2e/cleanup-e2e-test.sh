# Copyright © 2022 Dell Inc. or its subsidiaries. All Rights Reserved.
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

#!/bin/bash

PROG="${0}"

# usage will print command execution help and then exit
function usage() {
  echo
  echo "Help for $PROG"
  echo
  echo "Usage: $PROG options..."
  echo "Options:"
  echo "  Optional"
  echo "  --keep-logs                              Do not delete logfiles in cleanup"
  echo "  -h                                       Help"
  echo

  exit 0
}

while getopts ":h-:" optchar; do
  case "${optchar}" in
  -)
    case "${OPTARG}" in
    keep-logs)
      KEEPLOGS=1
      ;;
    *)
      decho "Unknown option --${OPTARG}"
      decho "For help, run $PROG -h"
      exit 1
      ;;
    esac
    ;;
  h)
    usage
    ;;
  *)
    decho "Unknown option -${OPTARG}"
    decho "For help, run $PROG -h"
    exit 1
    ;;
  esac
done

rm -f cert-csi isilon.db vxflexos.db

if [ -z "$KEEPLOGS" ]; then
    rm -f error.log fatal.log info.log report.path
fi

exit 0
