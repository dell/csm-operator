//  Copyright © 2021 - 2025 Dell Inc. or its subsidiaries. All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//       http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package controllers

import (
	"context"
	"fmt"

	csmv1 "eos2git.cec.lab.emc.com/CSM/csm-operator/api/v1"
	"eos2git.cec.lab.emc.com/CSM/csm-operator/pkg/logger"
	"eos2git.cec.lab.emc.com/CSM/csm-operator/pkg/resources/configmap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrl "sigs.k8s.io/controller-runtime"
)

// createFSTRIMScriptsConfigMap creates a ConfigMap containing the FSTRIM setup script
func (r *ContainerStorageModuleReconciler) createFSTRIMScriptsConfigMap(ctx context.Context, cr csmv1.ContainerStorageModule, client client.Client) error {
	log := logger.GetLogger(ctx)
	
	configMapName := fmt.Sprintf("%s-fstrim-scripts", cr.Name)
	
	// Read the setup script from the embedded file
	setupScript := `#!/bin/bash
# Copyright © 2025-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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

set -euo pipefail

# Configuration
CRON_MARKER="# CSI PowerFlex FSTRIM"
WRAPPER_SCRIPT_PATH="/usr/local/bin/csi-powerflex-fstrim-wrapper.sh"
DRIVER_BINARY_PATH="${DRIVER_BINARY_PATH:-/csi-powerflex}"
LOG_FILE="${X_CSI_POWERFLEX_FSTRIM_LOG_FILE:-/var/log/csi-powerflex-fstrim.log}"

# Function: validate_cron_schedule
validate_cron_schedule() {
    local schedule="$1"
    
    local field_count=$(echo "$schedule" | awk '{print NF}')
    if [ "$field_count" -ne 5 ]; then
        echo "ERROR: Invalid cron schedule '$schedule'. Expected 5 fields." >&2
        return 1
    fi
    
    echo "Cron schedule validated: $schedule"
    return 0
}

# Function: install_wrapper_script
install_wrapper_script() {
    echo "Installing FSTRIM wrapper script to $WRAPPER_SCRIPT_PATH"
    
    cat > "$WRAPPER_SCRIPT_PATH" <<'EOF'
#!/bin/bash
# CSI PowerFlex FSTRIM Wrapper Script
set -euo pipefail

DRIVER_BINARY="${DRIVER_BINARY_PATH:-/csi-powerflex}"
LOG_FILE="${X_CSI_POWERFLEX_FSTRIM_LOG_FILE:-/var/log/csi-powerflex-fstrim.log}"

LOG_DIR=$(dirname "$LOG_FILE")
mkdir -p "$LOG_DIR"

echo "========================================" >> "$LOG_FILE"
echo "FSTRIM execution started at $(date)" >> "$LOG_FILE"
echo "========================================" >> "$LOG_FILE"

if "$DRIVER_BINARY" fstrim-run >> "$LOG_FILE" 2>&1; then
    echo "FSTRIM execution completed successfully at $(date)" >> "$LOG_FILE"
    exit 0
else
    EXIT_CODE=$?
    echo "FSTRIM execution failed with exit code $EXIT_CODE at $(date)" >> "$LOG_FILE"
    exit $EXIT_CODE
fi
EOF

    chmod +x "$WRAPPER_SCRIPT_PATH"
    echo "Wrapper script installed successfully"
}

# Function: install_cron
install_cron() {
    local schedule="${X_CSI_POWERFLEX_FSTRIM_SCHEDULE:-0 2 * * 0}"
    
    echo "Installing FSTRIM cron job with schedule: $schedule"
    
    if ! validate_cron_schedule "$schedule"; then
        echo "ERROR: Failed to validate cron schedule" >&2
        return 1
    fi
    
    install_wrapper_script
    
    crontab -l 2>/dev/null | grep -v "$CRON_MARKER" | crontab - || true
    
    (crontab -l 2>/dev/null || true; echo "$schedule $WRAPPER_SCRIPT_PATH $CRON_MARKER") | crontab -
    
    echo "FSTRIM cron job installed successfully"
    crontab -l | grep "$CRON_MARKER"
}

# Function: remove_cron
remove_cron() {
    echo "Removing FSTRIM cron job and cleaning up..."
    
    # Remove cron entry
    if crontab -l 2>/dev/null | grep -q "$CRON_MARKER"; then
        crontab -l 2>/dev/null | grep -v "$CRON_MARKER" | crontab - || true
        echo "Removed cron entry"
    else
        echo "No cron entry found to remove"
    fi
    
    # Remove wrapper script
    if [ -f "$WRAPPER_SCRIPT_PATH" ]; then
        rm -f "$WRAPPER_SCRIPT_PATH"
        echo "Removed wrapper script: $WRAPPER_SCRIPT_PATH"
    else
        echo "Wrapper script not found (already removed or never installed)"
    fi
    
    # Optional: Clean up old log files if requested
    if [ "${CLEANUP_LOGS:-false}" = "true" ] && [ -f "$LOG_FILE" ]; then
        rm -f "$LOG_FILE"
        echo "Removed log file: $LOG_FILE"
    fi
    
    echo "FSTRIM cron job cleanup completed successfully"
}

# Function: status
status() {
    echo "FSTRIM Cron Job Status:"
    echo "======================="
    
    if crontab -l 2>/dev/null | grep -q "$CRON_MARKER"; then
        echo "Status: INSTALLED"
        echo "Cron entry:"
        crontab -l | grep "$CRON_MARKER"
    else
        echo "Status: NOT INSTALLED"
    fi
    
    if [ -f "$WRAPPER_SCRIPT_PATH" ]; then
        echo "Wrapper script: EXISTS at $WRAPPER_SCRIPT_PATH"
    else
        echo "Wrapper script: NOT FOUND"
    fi
}

# Main execution
case "${1:-}" in
    install)
        install_cron
        ;;
    remove)
        remove_cron
        ;;
    status)
        status
        ;;
    *)
        echo "Usage: $0 {install|remove|status}"
        exit 1
        ;;
esac`
	
	// Create ConfigMap
	configMap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: cr.Namespace,
		},
		Data: map[string]string{
			"setup-fstrim-cron.sh": setupScript,
		},
	}
	
	// Set owner reference
	if err := ctrl.SetControllerReference(&configMap, &cr, r.Scheme); err != nil {
		log.Errorw("Failed to set controller reference", "error", err)
		return err
	}
	
	// Create or update ConfigMap
	return configmap.SyncConfigMap(ctx, configMap, client)
}
