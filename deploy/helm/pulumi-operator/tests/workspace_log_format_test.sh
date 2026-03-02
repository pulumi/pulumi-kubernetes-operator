#!/usr/bin/env bash

set -euo pipefail

chart_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

rendered_with_value="$(helm template "${chart_dir}" --set workspace.logFormat=json)"
value="$(printf '%s\n' "${rendered_with_value}" | yq eval '.spec.template.spec.containers[0].env[] | select(.name == "AGENT_LOG_FORMAT") | .value' -)"
test "${value}" = "json"

rendered_default="$(helm template "${chart_dir}")"
if printf '%s\n' "${rendered_default}" | yq eval '.spec.template.spec.containers[0].env[] | select(.name == "AGENT_LOG_FORMAT") | .value' - | grep -q .; then
	echo "AGENT_LOG_FORMAT should be absent when workspace.logFormat is unset" >&2
	exit 1
fi

# Workspace pod propagation is covered by the controller envtest that verifies
# the cluster-level AGENT_LOG_FORMAT default is copied into workspace pods.
