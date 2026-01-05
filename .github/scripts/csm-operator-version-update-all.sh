
#!/usr/bin/env bash
# CSM unified version update orchestrator (TAG-ONLY, scoped)
# Order when --scope=all: Operator -> Drivers -> Modules
# Reuses:
#   ./.github/scripts/operator-version-update.sh   (expects env CSM_OPERATOR, CSM_VERSION; pass 'tag')
#   ./.github/scripts/driver-version-update.sh     (we pass --release_type tag)
#   ./.github/scripts/module-version-update.sh     (functions; tag-only)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
cd "${REPO_ROOT}"

export GITHUB_WORKSPACE="${GITHUB_WORKSPACE:-$REPO_ROOT}"

# --- CLI flags (TAG-ONLY) ---
SCOPE="all"               # operator | drivers | modules | all
OPERATOR_VERSION=""       # vX.Y.Z (exported as CSM_OPERATOR)
CSM_VERSION=""            # vX.Y.Z (exported as CSM_VERSION)

# Drivers (X.Y.Z)
POWERSCALE_VERSION=""
POWERMAX_VERSION=""
POWERFLEX_VERSION=""
POWERSTORE_VERSION=""
UNITY_VERSION=""

# Modules (vX.Y.Z unless OTEL which is plain A.B.C)
UPDATE_MODULES="all"      # all or CSV list: observability,resiliency,replication,reverseproxy,authorization
OBS_VERSION=""
RES_VERSION=""
REP_VERSION=""
REVPROXY_VERSION=""
AUTH_V2_VERSION=""
OTEL_VERSION=""

print_usage() {
  cat <<'EOF'
Usage: csm-version-update.sh [options]

General:
  --scope {operator|drivers|modules|all}     # which sections to run (default: all)

Operator:
  --operator_version vA.B.C                  # used when scope includes operator
  --csm_version      vA.B.C

Drivers (TAG-ONLY):
  --powerscale_version X.Y.Z
  --powermax_version  X.Y.Z
  --powerflex_version X.Y.Z
  --powerstore_version X.Y.Z
  --unity_version     X.Y.Z

Modules (TAG-ONLY):
  --update_modules {all|observability,resiliency,replication,reverseproxy,authorization}
  --obs_version      vA.B.C
  --res_version      vA.B.C
  --rep_version      vA.B.C
  --revproxy_version vA.B.C
  --auth_v2_version  vA.B.C
  --otel_version     A.B.C
EOF
}

OPTS=$(getopt -o "" \
  -l "scope:,operator_version:,csm_version:,powerscale_version:,powermax_version:,powerflex_version:,powerstore_version:,unity_version:,update_modules:,obs_version:,res_version:,rep_version:,revproxy_version:,auth_v2_version:,otel_version:,help" -- "$@") || {
    echo "❌ Invalid arguments"; print_usage; exit 1;
}
eval set -- "$OPTS"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --scope)             SCOPE="$2"; shift ;;
    --operator_version)  OPERATOR_VERSION="$2"; shift ;;
    --csm_version)       CSM_VERSION="$2"; shift ;;
    --powerscale_version)POWERSCALE_VERSION="$2"; shift ;;
    --powermax_version)  POWERMAX_VERSION="$2"; shift ;;
    --powerflex_version) POWERFLEX_VERSION="$2"; shift ;;
    --powerstore_version)POWERSTORE_VERSION="$2"; shift ;;
    --unity_version)     UNITY_VERSION="$2"; shift ;;
    --update_modules)    UPDATE_MODULES="$2"; shift ;;
    --obs_version)       OBS_VERSION="$2"; shift ;;
    --res_version)       RES_VERSION="$2"; shift ;;
    --rep_version)       REP_VERSION="$2"; shift ;;
    --revproxy_version)  REVPROXY_VERSION="$2"; shift ;;
    --auth_v2_version)   AUTH_V2_VERSION="$2"; shift ;;
    --otel_version)      OTEL_VERSION="$2"; shift ;;
    --help)              print_usage; exit 0 ;;
    --) shift; break ;;
  esac
  shift
done

command -v yq >/dev/null 2>&1 || { echo "❌ yq (v4) is required in PATH"; exit 1; }

ver_suffix() { printf "%s" "${1//./}"; }

# --- Steps ---
run_operator_updates() {
  [[ "$SCOPE" == "operator" || "$SCOPE" == "all" ]] || return 0
  [[ -n "$OPERATOR_VERSION" ]] || { echo "❌ --operator_version (vX.Y.Z) is required"; exit 1; }
  [[ -n "$CSM_VERSION"     ]] || { echo "❌ --csm_version (vX.Y.Z) is required"; exit 1; }
  export CSM_OPERATOR="$OPERATOR_VERSION"
  export CSM_VERSION="$CSM_VERSION"
  echo "→ Updating operator (tag-only): operator=$OPERATOR_VERSION, csm=$CSM_VERSION"
  bash ".github/scripts/operator-version-update.sh" "_" "tag"
}

run_driver_updates() {
  [[ "$SCOPE" == "drivers" || "$SCOPE" == "all" ]] || return 0
  local all="${POWERSCALE_VERSION}${POWERMAX_VERSION}${POWERFLEX_VERSION}${POWERSTORE_VERSION}${UNITY_VERSION}"
  [[ -z "$all" ]] && { echo "ℹ️ Skipping driver updates (no driver versions provided)"; return 0; }
  echo "→ Updating drivers (tag-only)"
  bash ".github/scripts/driver-version-update.sh" \
    --driver_update_type "major" \
    --release_type "tag" \
    ${POWERSCALE_VERSION:+--powerscale_version "$POWERSCALE_VERSION"} \
    ${POWERMAX_VERSION:+--powermax_version "$POWERMAX_VERSION"} \
    ${POWERFLEX_VERSION:+--powerflex_version "$POWERFLEX_VERSION"} \
    ${POWERSTORE_VERSION:+--powerstore_version "$POWERSTORE_VERSION"} \
    ${UNITY_VERSION:+--unity_version "$UNITY_VERSION"}
}

run_module_updates() {
  [[ "$SCOPE" == "modules" || "$SCOPE" == "all" ]] || return 0
  [[ "$UPDATE_MODULES" == "none" ]] && { echo "ℹ️ Skipping module updates (--update_modules=none)"; return 0; }
  source ".github/scripts/module-version-update.sh"
  [[ -n "$OBS_VERSION"      ]] && obs_ver="$OBS_VERSION"
  [[ -n "$RES_VERSION"      ]] && res_ver="$RES_VERSION"
  [[ -n "$REP_VERSION"      ]] && rep_ver="$REP_VERSION"
  [[ -n "$REVPROXY_VERSION" ]] && revproxy_ver="$REVPROXY_VERSION"
  [[ -n "$AUTH_V2_VERSION"  ]] && auth_v2="$AUTH_V2_VERSION"
  [[ -n "$OTEL_VERSION"     ]] && otel_col="$OTEL_VERSION"
  [[ -n "$POWERMAX_VERSION"   ]] && CSI_POWERMAX="v${POWERMAX_VERSION}"   && pmax_driver_ver="$(ver_suffix "${CSI_POWERMAX}")"
  [[ -n "$POWERSCALE_VERSION" ]] && CSI_POWERSCALE="v${POWERSCALE_VERSION}" && pscale_driver_ver="$(ver_suffix "${CSI_POWERSCALE}")"
  [[ -n "$POWERFLEX_VERSION"  ]] && CSI_VXFLEXOS="v${POWERFLEX_VERSION}"    && pflex_driver_ver="$(ver_suffix "${CSI_VXFLEXOS}")"
  [[ -n "$POWERSTORE_VERSION" ]] && CSI_POWERSTORE="v${POWERSTORE_VERSION}"  && pstore_driver_ver="$(ver_suffix "${CSI_POWERSTORE}")"
  IFS=',' read -r -a mods <<<"$UPDATE_MODULES"
  [[ "$UPDATE_MODULES" == "all" ]] && mods=(observability resiliency replication reverseproxy authorization)
  echo "→ Updating modules (tag-only): ${mods[*]}"
  for m in "${mods[@]}"; do
    case "$m" in
      observability)  update_observability_tag_only ;;
      resiliency)     update_resiliency_tag_only ;;
      replication)    update_replication_tag_only ;;
      reverseproxy)   update_reverseproxy_tag_only ;;
      authorization)  update_authorization_v2_tag_only ;;
      ""|none)        ;;
      *) echo "⚠️ Unknown module '$m' — skipping";;
    esac
  done
  update_version_values_inplace
}

# --- Run according to scope ---
run_operator_updates
run_driver_updates
run_module_updates

echo "✅ Completed scope='${SCOPE}' (tag-only)."
