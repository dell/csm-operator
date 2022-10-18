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

rm -f isilon.db vxflexos.db

if [ -z "$KEEPLOGS" ]; then
    rm -f error.log fatal.log info.log report.path
fi

exit 0
