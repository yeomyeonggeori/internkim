package blueclaw

import "fmt"

const (
	InternKimBlueclawTemporaryCleanupScriptPath  = "/usr/local/bin/internkim-blueclaw-tmp-clean"
	InternKimBlueclawTemporaryCleanupServicePath = "/etc/systemd/system/internkim-blueclaw-tmp-clean.service"
	InternKimBlueclawTemporaryCleanupTimerPath   = "/etc/systemd/system/internkim-blueclaw-tmp-clean.timer"
)

func InternKimBlueclawTemporaryCleanupScript() string {
	return `#!/bin/sh
set -eu

WORKSPACE_PATH="${BLUECLAW_WORKSPACE_PATH:-/root/.blueclaw/workspace}"
PEOPLE_PATH="$WORKSPACE_PATH/private/people"

if [ ! -d "$PEOPLE_PATH" ]; then
  exit 0
fi

find "$PEOPLE_PATH" \
  -mindepth 3 \
  -maxdepth 3 \
  -type d \
  -path "$PEOPLE_PATH/*/tmp/*" \
  -mtime +6 \
  -exec rm -rf {} \;
`
}

func InternKimBlueclawTemporaryCleanupServiceUnit() string {
	return fmt.Sprintf(`[Unit]
Description=InternKim Blueclaw user temporary workspace cleanup

[Service]
Type=oneshot
User=root
ExecStart=%s
`, InternKimBlueclawTemporaryCleanupScriptPath)
}

func InternKimBlueclawTemporaryCleanupTimerUnit() string {
	return `[Unit]
Description=InternKim Blueclaw user temporary workspace cleanup timer

[Timer]
OnBootSec=20m
OnUnitActiveSec=1d
AccuracySec=30m
Unit=internkim-blueclaw-tmp-clean.service

[Install]
WantedBy=timers.target
`
}
