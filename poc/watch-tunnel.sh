#!/bin/zsh
# Reconcile the Cloudflare tunnel ingress with live container IPs.
# Apple Container assigns a new IP on every container restart, which silently
# breaks tunnel routes that were captured at provision time. Run this from a
# LaunchAgent every minute: it re-runs restart-tunnel.py only when the
# container IP map actually changed, so steady state costs one `container ls`.
set -uo pipefail
export PATH="/opt/homebrew/bin:/usr/local/bin:$PATH"
BASE="${POC_BASE:-$HOME/internkim-poc}"
cd "$BASE"
STATE_FILE="$BASE/.tunnel-ip-state"

current_state() {
  container ls 2>/dev/null | awk '$1 ~ /^(poc-mattermost|poc-tenant-[0-9]+)$/ && /running/ {for (i = 1; i <= NF; i++) if ($i ~ /^[0-9.]+\/[0-9]+$/) print $1, $i}' | sort
}

STATE="$(current_state)"
[ -n "$STATE" ] || exit 0

if ! container ls 2>/dev/null | awk '$1 == "cf-tunnel" && $5 == "running"' | grep -q cf-tunnel; then
  echo "$(date '+%F %T') cf-tunnel not running; restoring" >> "$BASE/tunnel-watch.log"
  /usr/bin/python3 restart-tunnel.py >> "$BASE/tunnel-watch.log" 2>&1 && printf '%s' "$STATE" > "$STATE_FILE"
  exit 0
fi

if [ ! -f "$STATE_FILE" ] || [ "$STATE" != "$(cat "$STATE_FILE")" ]; then
  echo "$(date '+%F %T') container IPs changed; reconciling tunnel" >> "$BASE/tunnel-watch.log"
  /usr/bin/python3 restart-tunnel.py >> "$BASE/tunnel-watch.log" 2>&1 && printf '%s' "$STATE" > "$STATE_FILE"
fi
