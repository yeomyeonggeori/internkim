package blueclaw

import (
	"path/filepath"
	"strings"
)

func HostWorkspacePayloadSyncCommand(temporaryPayloadPath string) string {
	sourcePath := filepath.Join(temporaryPayloadPath, "workspace", ".blueclaw", "runtime")
	targetPath := filepath.Join(BlueclawWorkspacePath, ".blueclaw", "runtime")
	return strings.Join([]string{
		"mkdir -p", quoteShellValue(filepath.Dir(targetPath)),
		"&& rsync -a --delete", quoteShellValue(sourcePath + "/"), quoteShellValue(targetPath + "/"),
		"&& chown -R blueclaw:blueclaw", quoteShellValue(targetPath),
	}, " ")
}

func StopForPayloadSyncCommand() string {
	return `systemctl stop ` + BlueclawServiceName + ` >/dev/null 2>&1 || true
blueclaw_process_pattern='[/]usr/local/bin/blueclaw-supervisor|[/]firecracker .*--api-sock /firecracker-api.socket'
for _ in $(seq 1 20); do
  if ! systemctl is-active --quiet ` + BlueclawServiceName + ` && ! pgrep -f "$blueclaw_process_pattern" >/dev/null; then
    exit 0
  fi
  sleep 1
done
systemctl kill ` + BlueclawServiceName + ` --kill-who=all --signal=KILL >/dev/null 2>&1 || true
for _ in $(seq 1 20); do
  if ! systemctl is-active --quiet ` + BlueclawServiceName + ` && ! pgrep -f "$blueclaw_process_pattern" >/dev/null; then
    exit 0
  fi
  sleep 1
done
systemctl status ` + BlueclawServiceName + ` --no-pager -l 2>/dev/null || true
pgrep -af "$blueclaw_process_pattern" || true
exit 1`
}

func StartAfterPayloadSyncCommand() string {
	return `systemctl start ` + BlueclawServiceName + `
for _ in $(seq 1 20); do
  if systemctl is-active --quiet ` + BlueclawServiceName + `; then
    exit 0
  fi
  sleep 1
done
systemctl status ` + BlueclawServiceName + ` --no-pager -l 2>/dev/null || true
exit 1`
}

func quoteShellValue(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
