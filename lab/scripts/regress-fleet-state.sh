#!/usr/bin/env bash
set -euo pipefail

regress_runtime_configuration() {
  local configuration_path="$1"
  [ -f "$configuration_path" ] || return 0
  python3 - "$configuration_path" <<'PYEOF'
import json, sys

configuration_path = sys.argv[1]
with open(configuration_path) as handle:
    document = json.load(handle)

language_model = document.get("languageModel")
if isinstance(language_model, dict):
    language_model.pop("llmd", None)
    language_model["defaultProvider"] = "capabilityLLM"

capabilities = document.get("capabilities")
if isinstance(capabilities, dict):
    capabilities["protocolVersion"] = "0.1.0"
    capabilities["aggregateProtocolHash"] = "pre-upgrade-stale-hash"

with open(configuration_path, "w") as handle:
    json.dump(document, handle, indent=2)
    handle.write("\n")
PYEOF
  chown root:blueclaw "$configuration_path" 2>/dev/null || true
  chmod 640 "$configuration_path"
  echo "regressed: $configuration_path"
}

regress_runtime_configuration /root/.blueclaw/config/runtime.json
regress_runtime_configuration /root/.blueclaw/workspace/.blueclaw/config/runtime.json

release_state_directory=/root/.internkim/state/admin/release-updates
mkdir -p "$release_state_directory"
cat > "$release_state_directory/current.json" <<'JSONEOF'
{
  "manifestVersion": 1,
  "releaseID": "20260701T000000Z-preupgrade00",
  "channel": "stable",
  "createdAt": "2026-07-01T00:00:00Z",
  "components": {}
}
JSONEOF
echo "regressed: $release_state_directory/current.json"

sync
echo "fleet state regressed to the pre-upgrade generation"
