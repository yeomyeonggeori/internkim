package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

type sshRecoveryRequest struct {
	fleetSignedRequest
}

type sshRecoveryResponse struct {
	Status      string                     `json:"status"`
	Action      string                     `json:"action"`
	Services    map[string]string          `json:"services"`
	Results     []sshRecoveryCommandResult `json:"results,omitempty"`
	JournalTail string                     `json:"journalTail,omitempty"`
	Snapshot    string                     `json:"snapshot,omitempty"`
	NextStep    string                     `json:"nextStep"`
	Rejected    string                     `json:"rejected,omitempty"`
	ObservedAt  time.Time                  `json:"observedAt"`
}

type sshRecoveryCommandResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Output string `json:"output,omitempty"`
}

func (service *Service) handleSSHRecovery(responseWriter http.ResponseWriter, request *http.Request, path string) {
	if request.Method != http.MethodPost || path != "/recovery/ssh-tunnel/restart" {
		http.NotFound(responseWriter, request)
		return
	}
	var payload sshRecoveryRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, "invalid recovery request body", http.StatusBadRequest)
		return
	}
	if errorValue := service.validateSSHRecoveryRequest(payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusForbidden)
		return
	}
	response := service.runSSHRecovery(request.Context(), payload.Action)
	service.writeJSON(responseWriter, response)
}

func (service *Service) validateSSHRecoveryRequest(payload sshRecoveryRequest) error {
	return service.validateFleetSignedRequest(payload.fleetSignedRequest, isAllowedSSHRecoveryAction)
}

func isAllowedSSHRecoveryAction(action string) bool {
	switch action {
	case "status", "snapshot", "restart-ssh", "restart-cloudflared-node-ssh", "journal-tail", "unlock-mattermost-admin", "reboot", "stop-tenant-pilots", "remove-tenant-pilots", "limit-blueclaw":
		return true
	default:
		return false
	}
}

func (service *Service) runSSHRecovery(ctx context.Context, action string) sshRecoveryResponse {
	response := sshRecoveryResponse{
		Status:     "ok",
		Action:     action,
		Services:   service.sshRecoveryServiceStates(ctx),
		ObservedAt: time.Now().UTC(),
		NextStep:   "Run `internkim verify api --cloudflare-ssh` again.",
	}
	switch action {
	case "status":
		return response
	case "snapshot":
		response.Snapshot = service.sshRecoverySnapshot(ctx)
		return response
	case "restart-ssh":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "restart ssh", "systemctl", "restart", "ssh"))
	case "restart-cloudflared-node-ssh":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "restart cloudflared-node-ssh", "systemctl", "restart", "cloudflared-node-ssh"))
	case "journal-tail":
		response.JournalTail = service.sshRecoveryJournalTail(ctx)
	case "unlock-mattermost-admin":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "unlock Mattermost admin", "sh", "-lc", mattermostAdminUnlockCommand()))
	case "reboot":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "schedule reboot", "systemd-run", "--on-active=3sec", "--unit=internkim-recovery-reboot", "systemctl", "reboot"))
		response.NextStep = "Wait about two minutes, then run `internkim status`."
		return response
	case "stop-tenant-pilots":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "stop tenant pilots", "sh", "-lc", stopTenantPilotsCommand()))
	case "remove-tenant-pilots":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "remove tenant pilots", "sh", "-lc", removeTenantPilotsCommand()))
	case "limit-blueclaw":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "limit Blueclaw Firecracker", "sh", "-lc", blueclawResourceLimitCommand()))
	}
	response.Services = service.sshRecoveryServiceStates(ctx)
	response.JournalTail = service.sshRecoveryJournalTail(ctx)
	return response
}

func blueclawResourceLimitCommand() string {
	workspaceRuntimeConfigPath := blueclaw.BlueclawWorkspacePath + "/.blueclaw/config/runtime.json"
	return strings.TrimSpace(fmt.Sprintf(`
set -eu
virtual_cpu_count=%d
memory_mib=%d
for path in %s %s; do
  [ -f "$path" ] || continue
  temporary_path=$(mktemp)
  jq --argjson virtualCPUCount "$virtual_cpu_count" --argjson memoryMiB "$memory_mib" '.firecracker.vcpuCount = $virtualCPUCount | .firecracker.memoryMiB = $memoryMiB' "$path" > "$temporary_path"
  cat "$temporary_path" > "$path"
  rm -f "$temporary_path"
  printf '%%s updated\n' "$path"
done
systemctl restart %s
health_status=failed
for attempt in $(seq 1 90); do
	if curl -fsS -m 2 http://127.0.0.1:8080/admin/api/health >/tmp/internkim-blueclaw-health.json 2>/dev/null; then
    health_status=ok
    break
  fi
  sleep 2
done
printf 'blueclaw health %%s\n' "$health_status"
jq -r '"runtime vcpuCount=" + (.firecracker.vcpuCount|tostring) + " memoryMiB=" + (.firecracker.memoryMiB|tostring)' %s
ps -eo pcpu,pmem,rss,pid,comm --sort=-rss | head -8
free -h
[ "$health_status" = ok ]
	`,
		blueclaw.BlueclawFirecrackerDefaultVirtualCPUCount,
		blueclaw.BlueclawFirecrackerDefaultMemoryMiB,
		blueclaw.BlueclawRuntimeConfigPath,
		workspaceRuntimeConfigPath,
		blueclaw.BlueclawServiceName,
		blueclaw.BlueclawRuntimeConfigPath,
	))
}

func stopTenantPilotsCommand() string {
	return strings.TrimSpace(`
set -eu
units=$(systemctl list-units --all --no-legend --plain 'internkim-tenant-*' 'internkim-mattermost-pilot-*' | awk '{print $1}')
for unit in $units; do
  systemctl disable --now "$unit" >/dev/null 2>&1 || true
  printf '%s stopped\n' "$unit"
done
free -m | head -2
`)
}

func removeTenantPilotsCommand() string {
	return strings.TrimSpace(`
set -eu
reloaded=0
for unit in $(systemctl list-unit-files --no-legend 'internkim-tenant-*pilot*.service' 'internkim-mattermost-pilot-*.service' 2>/dev/null | awk '{print $1}'); do
  systemctl disable --now "$unit" >/dev/null 2>&1 || true
  rm -f "/etc/systemd/system/$unit" && printf 'unit %s removed\n' "$unit"
  reloaded=1
done
[ "$reloaded" = "1" ] && systemctl daemon-reload || true
for dir in /srv/internkim/tenants/pilot-*; do
  [ -e "$dir" ] || continue
  rm -rf "$dir" && printf 'dir %s removed\n' "$dir"
done
free -m | head -2
`)
}

func mattermostAdminUnlockCommand() string {
	return strings.TrimSpace(`
set -eu
su - postgres -c "psql -X -qAt -c \"SELECT datname FROM pg_database WHERE datistemplate = false\"" | while IFS= read -r database; do
  [ -n "$database" ] || continue
  escaped_database=$(printf "%s" "$database" | sed "s/'/'\\\\''/g")
  has_users=$(su - postgres -c "psql -X -qAt -d '$escaped_database' -c \"SELECT to_regclass('public.users') IS NOT NULL\"")
  [ "$has_users" = "t" ] || continue
  has_failed_attempts=$(su - postgres -c "psql -X -qAt -d '$escaped_database' -c \"SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'users' AND column_name = 'failedattempts')\"")
  [ "$has_failed_attempts" = "t" ] || continue
  updated=$(su - postgres -c "psql -X -qAt -d '$escaped_database' -c \"UPDATE users SET failedattempts = 0 WHERE username = 'admin' RETURNING username\"")
  [ -z "$updated" ] || printf "%s: admin failedattempts reset\n" "$database"
done
`)
}

func (service *Service) sshRecoveryServiceStates(ctx context.Context) map[string]string {
	return map[string]string{
		"ssh":                  service.sshRecoveryCommandOutput(ctx, "systemctl", "is-active", "ssh"),
		"cloudflared-node-ssh": service.sshRecoveryCommandOutput(ctx, "systemctl", "is-active", "cloudflared-node-ssh"),
		"cloudflared":          service.sshRecoveryCommandOutput(ctx, "systemctl", "is-active", "cloudflared"),
	}
}

func (service *Service) runSSHRecoveryCommand(ctx context.Context, label string, name string, arguments ...string) sshRecoveryCommandResult {
	output, errorValue := service.runCommand(ctx, name, arguments...)
	status := "ok"
	if errorValue != nil {
		status = "error"
	}
	return sshRecoveryCommandResult{
		Name:   label,
		Status: status,
		Output: redactRecoveryOutput(string(output)),
	}
}

func (service *Service) sshRecoveryCommandOutput(ctx context.Context, name string, arguments ...string) string {
	output, errorValue := service.runCommand(ctx, name, arguments...)
	if errorValue != nil {
		return strings.TrimSpace(redactRecoveryOutput(string(output)))
	}
	return strings.TrimSpace(redactRecoveryOutput(string(output)))
}

func (service *Service) sshRecoveryJournalTail(ctx context.Context) string {
	output, _ := service.runCommand(ctx, "journalctl", "-u", "ssh", "-u", "cloudflared-node-ssh", "-n", "80", "--no-pager")
	return redactRecoveryOutput(string(output))
}

func (service *Service) sshRecoverySnapshot(ctx context.Context) string {
	output, _ := service.runCommand(ctx, "sh", "-lc", sshRecoverySnapshotCommand())
	return redactRecoveryOutput(string(output))
}

func sshRecoverySnapshotCommand() string {
	return strings.TrimSpace(`
set +e
section() {
  printf '\n== %s ==\n' "$1"
}
section time
date -u
uptime
section routes
ip route show default
ip -brief address show
nmcli -t -f DEVICE,TYPE,STATE,CONNECTION device status
section ethernet
for device in /sys/class/net/en*; do
  [ -e "$device" ] || continue
  name=$(basename "$device")
  printf '%s carrier=' "$name"
  cat "$device/carrier" 2>/dev/null || true
  printf '%s operstate=' "$name"
  cat "$device/operstate" 2>/dev/null || true
  ethtool "$name" 2>/dev/null | grep -E 'Speed|Duplex|Auto-negotiation|Link detected' || true
done
section wifi
iw dev 2>/dev/null | sed -n '1,80p'
section pressure
cat /proc/pressure/cpu /proc/pressure/memory /proc/pressure/io 2>/dev/null
section memory
free -h
swapon --show 2>/dev/null
section top
top -b -n 1 | head -30
section processes
ps -eo pcpu,pmem,rss,pid,comm --sort=-pcpu | head -25
section failed-services
systemctl --no-pager --failed
section service-states
systemctl is-active ssh cloudflared cloudflared-node-ssh NetworkManager 2>/dev/null
section cloudflared-journal
journalctl -u cloudflared -u cloudflared-node-ssh -n 120 --no-pager 2>/dev/null
section networkmanager-journal
journalctl -u NetworkManager -n 80 --no-pager 2>/dev/null
section kernel-network
journalctl -k -n 120 --no-pager 2>/dev/null | grep -i -E 'oom|killed process|eth|wifi|wlan|carrier|link|r816|network|timeout|tls|dns|error|fail|reset' | tail -80
`)
}

func redactRecoveryOutput(value string) string {
	lines := strings.Split(value, "\n")
	for index, line := range lines {
		lines[index] = redactRecoveryLine(line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func redactRecoveryLine(value string) string {
	normalizedLine := strings.ToLower(value)
	if strings.Contains(normalizedLine, "authorization:") || strings.Contains(normalizedLine, "bearer ") {
		return "[redacted]"
	}
	fields := strings.Fields(value)
	for index, field := range fields {
		normalizedField := strings.ToLower(field)
		if strings.Contains(normalizedField, "token") ||
			strings.Contains(normalizedField, "secret") {
			fields[index] = "[redacted]"
			if index+1 < len(fields) && !strings.Contains(field, "=") {
				fields[index+1] = "[redacted]"
			}
		}
	}
	return strings.Join(fields, " ")
}
