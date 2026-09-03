package setup

import (
	"os"
	"path/filepath"
	"strings"
)

func withPackageWorkSettled(command string) string {
	return command + `
wait_for_package_work_to_settle() {
  attempt=0
  while [ "$attempt" -lt 120 ]; do
    running=''
    for comm_path in /proc/[0-9]*/comm; do
      read -r process_name < "$comm_path" 2>/dev/null || continue
      case "$process_name" in
        apt|apt-get|dpkg|useradd|usermod|groupadd) running="$running $process_name" ;;
      esac
    done
    [ -n "$running" ] || return 0
    attempt=$((attempt + 1))
    sleep 1
  done
  echo "package installation is still running:$running" >&2
  return 1
}
wait_for_package_work_to_settle`
}

func trimmedRun(context *Context, command string) string {
	if context.SSH == nil {
		return ""
	}
	return strings.TrimSpace(context.SSH.Run(command))
}

func sshFileExists(context *Context, path string) bool {
	if context.SSH == nil {
		return false
	}
	return trimmedRun(context, "test -e "+shellQuote(path)+" && echo y || echo n") == "y"
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func readStagedFile(context *Context, stagePath string) ([]byte, error) {
	if context.SD == nil {
		return nil, ErrUnsupportedBackend
	}
	return os.ReadFile(filepath.Join(context.SD.RootPath(), stagePath))
}

func stagedFileExists(context *Context, stagePath string) bool {
	if context.SD == nil {
		return false
	}
	_, statError := os.Stat(filepath.Join(context.SD.RootPath(), stagePath))
	return statError == nil
}
