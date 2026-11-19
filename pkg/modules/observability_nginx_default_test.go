// ****
// Secure code generated utilizing Code Guardian.
// Applicable rules used: CG-K8S-001.2, CG-K8S-001.7
// ****

// Copyright © 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package modules

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/operatorutils"
)

// nginx-unprivileged 1.31.8 (Debian trixie) ships libbrotli1 1.1.0-2+deb13u1,
// the Debian fix for CVE-2026-111497. The default is pinned by digest so the
// fixed build cannot be silently replaced by a retag.
const brotliFixedNginxProxyImage = "quay.io/nginx/nginx-unprivileged:1.31.8"

func TestGetOtelCollector_DefaultNginxProxyImageIsBrotliFixedAndPinned(t *testing.T) {
	cr, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
	require.NoError(t, err)

	cleared := false
	for mi := range cr.Spec.Modules {
		if cr.Spec.Modules[mi].Name != csmv1.Observability {
			continue
		}
		for ci := range cr.Spec.Modules[mi].Components {
			if cr.Spec.Modules[mi].Components[ci].Name == ObservabilityOtelCollectorName {
				cr.Spec.Modules[mi].Components[ci].Envs = nil
				cleared = true
			}
		}
	}
	require.True(t, cleared, "fixture has no otel-collector component")

	yaml, err := getOtelCollector(context.Background(), operatorConfig, cr, operatorutils.VersionSpec{})
	require.NoError(t, err)

	assert.Contains(t, yaml, "image: "+brotliFixedNginxProxyImage)
	assert.NotContains(t, yaml, "nginx-unprivileged:1.27")
	assert.NotContains(t, yaml, "nginx-unprivileged:1.30.4")
}
