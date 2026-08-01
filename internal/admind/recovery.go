package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

// ensureBuzzRelayTerminator keeps the loopback TLS terminator (:443 -> relay
// :3000) running whenever this box hosts the Buzz relay. The Debian SysV
// stunnel4 service is not auto-restarted, so a native systemd unit is installed
// and (re)started on every admind start. Runs detached from any request context.
func (service *Service) ensureBuzzRelayTerminator() {
	if strings.TrimSpace(service.Configuration.BuzzRelayURL) == "" {
		return
	}
	go func() {
		output, errorValue := service.runCommand(context.Background(), "sh", "-lc", buzzRelayRepairCommand())
		if errorValue != nil {
			log.Printf("buzz relay terminator ensure failed: %v: %s", errorValue, strings.TrimSpace(string(output)))
		}
	}()
}

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
	case "status", "snapshot", "restart-ssh", "restart-cloudflared-node-ssh", "journal-tail", "unlock-mattermost-admin", "reboot", "stop-tenant-pilots", "remove-tenant-pilots", "limit-blueclaw", "restart-blueclaw", "blueclaw-boot-diagnose", "blueclaw-journal", "blueclaw-workspace-repair", "blueclaw-postgres-salvage", "repair-buzz-relay", "buzz-relay-journal", "enable-buzz-mirror", "buzz-mirror-status", "buzz-orphan-inspect", "buzz-snapshot", "buzz-membership-recover", "buzz-restore", "buzz-repair-dryrun", "buzz-repair-apply", "buzz-reimport", "buzz-reimport-log", "buzz-read-test", "buzz-chatd-repair", "mattermost-unlock-users", "postgres-repair", "mattermost-restart", "mattermost-db-resync", "mattermost-fix-dbname", "mattermost-force-dbname":
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
	case "restart-blueclaw":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "restart Blueclaw", "sh", "-lc", blueclawRestartDiagnosticCommand()))
	case "blueclaw-boot-diagnose":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "diagnose Blueclaw guest boot", "sh", "-lc", blueclawBootDiagnoseCommand()))
	case "blueclaw-journal":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "read Blueclaw supervisor journal", "sh", "-lc", blueclawJournalCommand()))
	case "blueclaw-workspace-repair":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "repair Blueclaw workspace image", "sh", "-lc", blueclawWorkspaceRepairCommand()))
	case "blueclaw-postgres-salvage":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "salvage orphaned Blueclaw postgres cluster", "sh", "-lc", blueclawPostgresSalvageCommand()))
	case "repair-buzz-relay":
		repairContext, cancelRepair := context.WithTimeout(context.Background(), 60*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(repairContext, "repair Buzz relay TLS terminator", "sh", "-lc", buzzRelayRepairCommand()))
		cancelRepair()
	case "buzz-relay-journal":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "read Buzz relay TLS terminator journal", "sh", "-lc", buzzRelayJournalDiagnosticCommand()))
	case "enable-buzz-mirror":
		mirrorContext, cancelMirror := context.WithTimeout(context.Background(), 90*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(mirrorContext, "enable Buzz<->Mattermost mirror", "sh", "-lc", buzzMirrorEnableCommand()))
		cancelMirror()
	case "buzz-mirror-status":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "read Buzz<->Mattermost mirror status", "sh", "-lc", buzzMirrorStatusCommand()))
	case "buzz-orphan-inspect":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "inspect imported orphan-thread roots", "sh", "-lc", buzzOrphanInspectCommand()))
	case "buzz-membership-recover":
		membershipContext, cancelMembership := context.WithTimeout(context.Background(), 90*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(membershipContext, "recover buzz staff membership", "sh", "-lc", buzzMembershipRecoverCommand()))
		cancelMembership()
	case "buzz-restore":
		restoreContext, cancelRestore := context.WithTimeout(context.Background(), 300*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(restoreContext, "restore buzz relay database from snapshot", "sh", "-lc", buzzRestoreCommand()))
		cancelRestore()
	case "buzz-repair-dryrun":
		dryContext, cancelDry := context.WithTimeout(context.Background(), 200*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(dryContext, "dry-run orphan-root repair", "sh", "-lc", buzzRepairCommand(false)))
		cancelDry()
	case "buzz-repair-apply":
		applyContext, cancelApply := context.WithTimeout(context.Background(), 200*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(applyContext, "apply orphan-root repair", "sh", "-lc", buzzRepairCommand(true)))
		cancelApply()
	case "buzz-reimport":
		reimportContext, cancelReimport := context.WithTimeout(context.Background(), 300*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(reimportContext, "re-import Mattermost history into Buzz (wipe + bot-inclusive, loopback membership, self-healing)", "sh", "-lc", buzzReimportCommand()))
		cancelReimport()
	case "buzz-reimport-log":
		logContext, cancelLog := context.WithTimeout(context.Background(), 60*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(logContext, "tail Buzz re-import logs", "sh", "-lc", buzzReimportLogCommand()))
		cancelLog()
	case "buzz-read-test":
		readContext, cancelRead := context.WithTimeout(context.Background(), 60*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(readContext, "test the web channel-read path for a real user", "sh", "-lc", buzzReadTestCommand()))
		cancelRead()
	case "buzz-chatd-repair":
		chatdContext, cancelChatd := context.WithTimeout(context.Background(), 60*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(chatdContext, "restart chatd with TLS bypass + relay debug", "sh", "-lc", buzzChatdRepairCommand()))
		cancelChatd()
	case "mattermost-unlock-users":
		unlockContext, cancelUnlock := context.WithTimeout(context.Background(), 60*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(unlockContext, "diagnose + reset Mattermost login lockouts", "sh", "-lc", mattermostUnlockUsersCommand()))
		cancelUnlock()
	case "postgres-repair":
		pgContext, cancelPg := context.WithTimeout(context.Background(), 90*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(pgContext, "diagnose + restart postgres", "sh", "-lc", postgresRepairCommand()))
		cancelPg()
	case "mattermost-restart":
		mmContext, cancelMM := context.WithTimeout(context.Background(), 90*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(mmContext, "restart Mattermost + verify", "sh", "-lc", mattermostRestartCommand()))
		cancelMM()
	case "mattermost-db-resync":
		resyncContext, cancelResync := context.WithTimeout(context.Background(), 60*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(resyncContext, "resync mmuser db password + restart", "sh", "-lc", mattermostDbResyncCommand()))
		cancelResync()
	case "mattermost-fix-dbname":
		fixContext, cancelFix := context.WithTimeout(context.Background(), 60*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(fixContext, "point MM config at the mattermost db + restart", "sh", "-lc", mattermostFixDbnameCommand()))
		cancelFix()
	case "mattermost-force-dbname":
		forceContext, cancelForce := context.WithTimeout(context.Background(), 60*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(forceContext, "replace mattermost_test->mattermost everywhere + restart", "sh", "-lc", mattermostForceDbnameCommand()))
		cancelForce()
	case "buzz-snapshot":
		snapshotContext, cancelSnapshot := context.WithTimeout(context.Background(), 180*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(snapshotContext, "snapshot buzz relay database", "sh", "-lc", buzzSnapshotCommand()))
		cancelSnapshot()
	}
	response.Services = service.sshRecoveryServiceStates(ctx)
	response.JournalTail = service.sshRecoveryJournalTail(ctx)
	return response
}

func blueclawRestartDiagnosticCommand() string {
	return strings.TrimSpace(fmt.Sprintf(`
set +e
curl -fsS -m 10 -X POST http://127.0.0.1:8080/admin/api/runtime/prepare-shutdown >/dev/null 2>&1 || true
systemctl restart %s
restart_status=$?
printf 'systemctl restart %s exit=%%s\n' "$restart_status"
health_status=failed
for attempt in $(seq 1 15); do
	if curl -fsS -m 2 http://127.0.0.1:8080/admin/api/health >/tmp/internkim-blueclaw-health.json 2>/dev/null; then
    health_status=ok
    break
  fi
  sleep 1
done
printf 'blueclaw health %%s\n' "$health_status"
printf '\n== blueclaw status ==\n'
systemctl status %s --no-pager -l 2>/dev/null | tail -100 || true
printf '\n== blueclaw processes ==\n'
ps -eo pid,stat,comm | grep -E 'blueclaw|firecracker|jailer' || true
printf '\n== blueclaw ports ==\n'
ss -ltnp 2>/dev/null | grep ':8080' || true
printf '\n== blueclaw journal ==\n'
journalctl -u %s -n 180 --no-pager 2>/dev/null || true
[ "$health_status" = ok ]
	`,
		blueclaw.BlueclawServiceName,
		blueclaw.BlueclawServiceName,
		blueclaw.BlueclawServiceName,
		blueclaw.BlueclawServiceName,
	))
}

func blueclawJournalCommand() string {
	return strings.TrimSpace(fmt.Sprintf(`
set +e
printf '== %s journal (12h, last 500) ==\n'
journalctl -u %s --since '12 hours ago' --no-pager -o short-iso 2>/dev/null | tail -n 500
printf '\n== kernel oom (12h) ==\n'
journalctl -k --since '12 hours ago' --no-pager 2>/dev/null | grep -i -E 'oom|out of memory|killed process' | tail -n 40
printf '\n== %s unit state ==\n'
systemctl status %s --no-pager -l 2>/dev/null | head -25
`,
		blueclaw.BlueclawServiceName,
		blueclaw.BlueclawServiceName,
		blueclaw.BlueclawServiceName,
		blueclaw.BlueclawServiceName,
	))
}

func blueclawBootDiagnoseCommand() string {
	return strings.TrimSpace(`
set +e
printf '== firecracker processes ==\n'
ps -eo pid,stat,etimes,comm | grep -E 'blueclaw|firecracker|jailer' || true
printf '\n== newest guest log directories ==\n'
ls -dt /var/log/blueclaw-supervisor/* 2>/dev/null | head -4 || true
for logDirectory in $(ls -dt /var/log/blueclaw-supervisor/* 2>/dev/null | head -2); do
  printf '\n== %s stderr.log ==\n' "$logDirectory"
  tail -c 4000 "$logDirectory/stderr.log" 2>/dev/null || printf '(missing)\n'
  printf '\n== %s stdout.log ==\n' "$logDirectory"
  tail -c 4000 "$logDirectory/stdout.log" 2>/dev/null || printf '(missing)\n'
done
newestJailerRoot=$(ls -dt /var/lib/bc/firecracker/*/root 2>/dev/null | head -1)
printf '\n== jailer root %s ==\n' "$newestJailerRoot"
ls -la "$newestJailerRoot" 2>/dev/null || true
printf '\n== firecracker-config.json ==\n'
head -c 4000 "$newestJailerRoot/firecracker-config.json" 2>/dev/null || printf '(missing)\n'
printf '\n== boot input images ==\n'
ls -la /var/lib/blueclaw/ 2>/dev/null || true
df -h /var/lib/bc /var/lib/blueclaw 2>/dev/null || true
printf '\n== rootfs guest-init lines 150-240 ==\n'
debugfs -c -R 'cat /sbin/init' "$newestJailerRoot/rootfs.ext4" 2>&1 | awk 'NR>=150 && NR<=240 {print NR": "$0}'
printf '\n== guest postgres logs ==\n'
debugfs -c -R 'cat /.blueclaw/logs/postgres.log' /var/lib/blueclaw/workspace.ext4 2>/dev/null | tail -20
debugfs -c -R 'cat /.blueclaw/logs/postgres-init.log' /var/lib/blueclaw/workspace.ext4 2>/dev/null | tail -10
printf '\n== guest blueclaw log ==\n'
debugfs -c -R 'cat /.blueclaw/logs/blueclaw.log' /var/lib/blueclaw/workspace.ext4 2>/dev/null | tail -40
printf '\n== guest runtime current ==\n'
debugfs -c -R 'stat /.blueclaw/runtime/current' /var/lib/blueclaw/workspace.ext4 2>&1 | head -8
printf '\n== workspace lost+found ==\n'
debugfs -c -R 'ls -l /lost+found' /var/lib/blueclaw/workspace.ext4 2>/dev/null | head -30
printf '\n== current postgres cluster age ==\n'
debugfs -c -R 'stat /.blueclaw/postgres/data/PG_VERSION' /var/lib/blueclaw/workspace.ext4 2>/dev/null | grep -E 'ctime|crtime'
printf '\n== memory tree ==\n'
debugfs -c -R 'ls -l /.blueclaw/memory' /var/lib/blueclaw/workspace.ext4 2>&1 | head -8
debugfs -c -R 'ls -l /.blueclaw/memory/people' /var/lib/blueclaw/workspace.ext4 2>&1 | head -20
printf '\n== remaining lost+found orphan contents ==\n'
for orphanInode in $(debugfs -c -R 'ls /lost+found' /var/lib/blueclaw/workspace.ext4 2>/dev/null | tr -s ' ' '\n' | grep '^#'); do
  printf -- '-- %s --\n' "$orphanInode"
  debugfs -c -R "ls -l /lost+found/$orphanInode" /var/lib/blueclaw/workspace.ext4 2>/dev/null | grep -v 'debugfs 1' | head -10
  firstChild=$(debugfs -c -R "ls /lost+found/$orphanInode" /var/lib/blueclaw/workspace.ext4 2>/dev/null | tr -s ' ' '\n' | grep -vE '^\.{1,2}$|^$' | head -1)
  if [ -n "$firstChild" ]; then
    printf -- '   depth2 %s:\n' "$firstChild"
    debugfs -c -R "ls -l /lost+found/$orphanInode/$firstChild" /var/lib/blueclaw/workspace.ext4 2>/dev/null | grep -v 'debugfs 1' | head -8
  fi
done
`)
}

func blueclawWorkspaceRepairCommand() string {
	repairScript := strings.TrimSpace(`
set +e
exec >/var/log/internkim-workspace-repair.log 2>&1
workspaceImage=/var/lib/blueclaw/workspace.ext4
hostPath=/root/.blueclaw/workspace
printf '== stopping blueclaw ==\n'
systemctl stop blueclaw
sleep 2
printf '\n== detaching every mount of the image ==\n'
for loopDevice in $(losetup -j "$workspaceImage" 2>/dev/null | cut -d: -f1); do
  for mountTarget in $(findmnt -rn -o TARGET -S "$loopDevice" 2>/dev/null); do
    printf 'umount %s\n' "$mountTarget"
    umount "$mountTarget" 2>&1 || { fuser -vm "$mountTarget" 2>&1 | head -6; umount -l "$mountTarget" 2>&1; }
  done
  losetup -d "$loopDevice" 2>/dev/null
done
printf '\n== fsck ==\n'
e2fsck -fy "$workspaceImage" 2>&1 | tail -40
printf 'e2fsck exit=%s\n' "$?"
printf '\n== quarantining broken paths inside the image ==\n'
inspectMount=$(mktemp -d)
if mount -o loop "$workspaceImage" "$inspectMount"; then
  stat "$inspectMount/.blueclaw/postgres" "$inspectMount/.blueclaw/postgres/data" 2>&1 | head -20
  for brokenCandidate in "$inspectMount/.blueclaw/postgres/data" "$inspectMount/.blueclaw/postgres"; do
    if [ -e "$brokenCandidate" ] && [ ! -d "$brokenCandidate" ]; then
      printf 'quarantining %s\n' "$brokenCandidate"
      mv "$brokenCandidate" "$brokenCandidate.broken.$(date +%s)" || rm -f "$brokenCandidate"
    fi
  done
  printf '\n== guest config and identity state ==\n'
  ls -la "$inspectMount/.blueclaw/config/" "$inspectMount/.blueclaw/identity-map.json" 2>&1 | head -20
  if [ -e "$inspectMount/.blueclaw/identity-map.json" ] && [ ! -s "$inspectMount/.blueclaw/identity-map.json" ]; then
    printf 'removing truncated identity-map.json for regeneration\n'
    rm -f "$inspectMount/.blueclaw/identity-map.json"
  fi
  for configDocument in "$inspectMount/.blueclaw/config/policy.json" "$inspectMount/.blueclaw/config/runtime.json"; do
    if [ -e "$configDocument" ] && [ ! -s "$configDocument" ]; then
      printf 'EMPTY config document: %s\n' "$configDocument"
    fi
  done
  umount "$inspectMount"
fi
rmdir "$inspectMount" 2>/dev/null
printf '\n== ensuring host staging path is a plain directory ==\n'
mountpoint -q "$hostPath" && umount "$hostPath"
mkdir -p "$hostPath"
printf '\n== starting blueclaw ==\n'
systemctl start blueclaw
for attempt in $(seq 1 120); do
  if curl -fsS -m 2 http://127.0.0.1:8080/admin/api/health >/dev/null 2>&1; then
    printf 'blueclaw health ok\n'
    exit 0
  fi
  sleep 2
done
printf 'blueclaw health failed\n'
`)
	return "systemd-run --unit=internkim-blueclaw-workspace-repair --collect sh -c " +
		quoteRecoveryShellValue(repairScript) +
		" && echo 'repair started; tail /var/log/internkim-workspace-repair.log for progress'"
}

func quoteRecoveryShellValue(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func blueclawPostgresSalvageCommand() string {
	salvageScript := strings.TrimSpace(`
set +e
exec >/var/log/internkim-postgres-salvage.log 2>&1
image=/var/lib/blueclaw/workspace.ext4
systemctl stop blueclaw
sleep 2
for loopDevice in $(losetup -j "$image" 2>/dev/null | cut -d: -f1); do
  for mountTarget in $(findmnt -rn -o TARGET -S "$loopDevice" 2>/dev/null); do umount "$mountTarget" || umount -l "$mountTarget"; done
  losetup -d "$loopDevice" 2>/dev/null
done
workspaceMount=$(mktemp -d)
mount -o loop "$image" "$workspaceMount" || { systemctl start blueclaw; exit 1; }
lostFound="$workspaceMount/lost+found"
dataPath="$workspaceMount/.blueclaw/postgres/data"
recoveredPath="$workspaceMount/.blueclaw/postgres/data-recovered"
finish() {
  umount "$workspaceMount"; rmdir "$workspaceMount" 2>/dev/null
  systemctl start blueclaw
}
rm -rf "$recoveredPath"
cp -a "$dataPath" "$recoveredPath"
rm -rf "$recoveredPath/base" "$recoveredPath/global" "$recoveredPath/pg_wal" "$recoveredPath/pg_xact" "$recoveredPath/pg_multixact" "$recoveredPath/pg_logical" "$recoveredPath/postmaster.pid"
for orphan in "$lostFound"/\#*; do
  [ -d "$orphan" ] || continue
  if [ -e "$orphan/pg_control" ]; then echo "global <= $orphan"; mv "$orphan" "$recoveredPath/global"
  elif [ -d "$orphan/members" ] && [ -d "$orphan/offsets" ]; then echo "pg_multixact <= $orphan"; mv "$orphan" "$recoveredPath/pg_multixact"
  elif [ -d "$orphan/mappings" ] || [ -d "$orphan/snapshots" ]; then echo "pg_logical <= $orphan"; mv "$orphan" "$recoveredPath/pg_logical"
  elif ls "$orphan" 2>/dev/null | grep -qE '^[0-9A-F]{24}$'; then echo "pg_wal <= $orphan"; mv "$orphan" "$recoveredPath/pg_wal"
  else
    firstEntry=$(ls "$orphan" 2>/dev/null | head -1)
    if [ -n "$firstEntry" ] && [ -d "$orphan/$firstEntry" ] && echo "$firstEntry" | grep -qE '^[0-9]+$'; then
      echo "base <= $orphan"; mv "$orphan" "$recoveredPath/base"
    elif [ -n "$firstEntry" ] && [ -f "$orphan/$firstEntry" ] && echo "$firstEntry" | grep -qE '^[0-9A-F]{4}$' && [ ! -e "$recoveredPath/pg_xact" ]; then
      echo "pg_xact <= $orphan"; mv "$orphan" "$recoveredPath/pg_xact"
    fi
  fi
done
for requiredPiece in global base pg_wal pg_xact; do
  if [ ! -e "$recoveredPath/$requiredPiece" ]; then
    echo "salvage aborted: $requiredPiece not identified in lost+found"
    rm -rf "$recoveredPath"
    finish
    exit 2
  fi
done
[ -e "$recoveredPath/pg_multixact" ] || mkdir -p "$recoveredPath/pg_multixact/members" "$recoveredPath/pg_multixact/offsets"
[ -e "$recoveredPath/pg_logical" ] || mkdir -p "$recoveredPath/pg_logical/mappings" "$recoveredPath/pg_logical/snapshots"
rm -rf "$recoveredPath/pg_stat" && mkdir -p "$recoveredPath/pg_stat"
chown -R 100:102 "$recoveredPath"
chmod 700 "$recoveredPath"
freshBackup="$workspaceMount/.blueclaw/postgres/data-fresh-$(date +%s)"
mv "$dataPath" "$freshBackup"
mv "$recoveredPath" "$dataPath"
echo "swapped in salvaged cluster; fresh cluster kept at $freshBackup"
umount "$workspaceMount"; rmdir "$workspaceMount" 2>/dev/null
systemctl start blueclaw
for attempt in $(seq 1 120); do
  if curl -fsS -m 2 http://127.0.0.1:8080/admin/api/health >/dev/null 2>&1; then
    echo "blueclaw health ok on salvaged cluster"
    exit 0
  fi
  sleep 2
done
echo "salvaged cluster failed health; reverting to fresh cluster"
systemctl stop blueclaw
sleep 2
for loopDevice in $(losetup -j "$image" 2>/dev/null | cut -d: -f1); do
  for mountTarget in $(findmnt -rn -o TARGET -S "$loopDevice" 2>/dev/null); do umount "$mountTarget" || umount -l "$mountTarget"; done
  losetup -d "$loopDevice" 2>/dev/null
done
workspaceMount=$(mktemp -d)
mount -o loop "$image" "$workspaceMount" || exit 1
freshBackup=$(ls -dt "$workspaceMount/.blueclaw/postgres/data-fresh-"* 2>/dev/null | head -1)
mv "$workspaceMount/.blueclaw/postgres/data" "$workspaceMount/.blueclaw/postgres/data-salvage-failed"
mv "$freshBackup" "$workspaceMount/.blueclaw/postgres/data"
umount "$workspaceMount"; rmdir "$workspaceMount" 2>/dev/null
systemctl start blueclaw
echo "reverted to fresh cluster"
`)
	return "systemd-run --unit=internkim-blueclaw-postgres-salvage --collect sh -c " +
		quoteRecoveryShellValue(salvageScript) +
		" && echo 'salvage started; tail /var/log/internkim-postgres-salvage.log for progress'"
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

func buzzRelayRepairCommand() string {
	return strings.TrimSpace(fmt.Sprintf(`
set +e
mkdir -p /root/.internkim/tls
cat > /root/.internkim/tls/buzz-relay-stunnel.conf <<'STUNNELCONF'
foreground = yes
[buzz-relay]
accept = 127.0.0.1:443
connect = %s
cert = %s
key = /root/.internkim/tls/relay.key
STUNNELCONF
rm -f /etc/stunnel/buzz-relay.conf
cat > /etc/systemd/system/buzz-relay-stunnel.service <<'STUNNELUNIT'
[Unit]
Description=Buzz relay TLS terminator (stunnel)
After=network-online.target %s.service
Wants=network-online.target
[Service]
ExecStart=/usr/bin/stunnel4 /root/.internkim/tls/buzz-relay-stunnel.conf
Restart=always
RestartSec=2
[Install]
WantedBy=multi-user.target
STUNNELUNIT
systemctl disable --now stunnel4 2>/dev/null
systemctl mask stunnel4 2>/dev/null
pkill -x stunnel4 2>/dev/null
systemctl daemon-reload
systemctl enable buzz-relay-stunnel 2>&1
systemctl restart buzz-relay-stunnel 2>&1
sleep 3
printf 'is-active: '; systemctl is-active buzz-relay-stunnel
printf '== unit status ==\n'; systemctl status buzz-relay-stunnel --no-pager -l 2>&1 | tail -18
systemctl restart chatd 2>&1
printf '== port443 ==\n'; ss -ltn 2>/dev/null | grep ':443' || printf '443 DOWN\n'
`,
		blueclaw.BuzzRelayBindAddress,
		blueclaw.BuzzRelayCertificatePath,
		blueclaw.BuzzRelayServiceName,
	))
}

func buzzRelayJournalDiagnosticCommand() string {
	return strings.TrimSpace(`
set +e
printf '== stunnel binaries ==\n'
ls -la /usr/bin/stunnel* 2>&1
printf '\n== buzz-relay-stunnel unit status ==\n'
systemctl status buzz-relay-stunnel --no-pager -l 2>&1 | tail -25
printf '\n== buzz-relay-stunnel journal ==\n'
journalctl -u buzz-relay-stunnel -n 40 --no-pager 2>&1 | tail -40
printf '\n== repair unit journal ==\n'
journalctl -u internkim-buzz-relay-repair -n 30 --no-pager 2>&1 | tail -30
printf '\n== config ==\n'
cat /etc/stunnel/buzz-relay.conf 2>&1
printf '\n== cert/key present ==\n'
ls -la /root/.internkim/tls/ 2>&1
`)
}

func buzzMirrorEnableCommand() string {
	return strings.TrimSpace(`
set +e
MM=http://127.0.0.1:8065
ADMIN_EMAIL=$(cat /root/.internkim/config/admin-email 2>/dev/null || cat /root/.internkim/admin-email 2>/dev/null)
[ -n "$ADMIN_EMAIL" ] || ADMIN_EMAIL=admin
ADMIN_PASS=$(cat /root/.internkim/secrets/mm-admin-pass 2>/dev/null)
[ -n "$ADMIN_PASS" ] || { echo "no admin password file"; exit 1; }
TOKEN=$(curl -s -D- -o /dev/null -X POST $MM/api/v4/users/login -H 'Content-Type: application/json' -d "$(jq -n --arg l "$ADMIN_EMAIL" --arg p "$ADMIN_PASS" '{login_id:$l,password:$p}')" | awk 'tolower($1)=="token:"{print $2}' | tr -d '\r')
[ -n "$TOKEN" ] || { echo "admin login failed for $ADMIN_EMAIL"; exit 1; }
curl -s -X PUT $MM/api/v4/config/patch -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"ServiceSettings":{"EnableUserAccessTokens":true}}' >/dev/null
ADMIN_ID=$(curl -s $MM/api/v4/users/me -H "Authorization: Bearer $TOKEN" | jq -r .id)
ADMIN_PAT=$(curl -s -X POST $MM/api/v4/users/$ADMIN_ID/tokens -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"description":"chatd mirror admin"}' | jq -r .token)
[ -n "$ADMIN_PAT" ] && [ "$ADMIN_PAT" != null ] || { echo "admin PAT creation failed"; exit 1; }
BOT=$(cat /root/.internkim/secrets/mattermost-bot-token 2>/dev/null)
[ -n "$BOT" ] || { echo "bot token missing"; exit 1; }
mkdir -p /etc/systemd/system/chatd.service.d
cat > /etc/systemd/system/chatd.service.d/mirror.conf <<EOF
[Service]
Environment=CHATD_MATTERMOST_BASE_URL=$MM
Environment=CHATD_MATTERMOST_BOT_TOKEN=$BOT
Environment=CHATD_MATTERMOST_ADMIN_TOKEN=$ADMIN_PAT
Environment=CHATD_BLUECLAW_INGRESS_URL=http://127.0.0.1:8080
EOF
systemctl daemon-reload
systemctl restart chatd
sleep 5
if [ "$(systemctl is-active chatd)" != active ]; then
  rm -f /etc/systemd/system/chatd.service.d/mirror.conf
  systemctl daemon-reload
  systemctl restart chatd
  echo "ROLLED BACK: chatd failed to start with mirror config; messenger restored"
  exit 1
fi
echo "mirror enabled; chatd active"
journalctl -u chatd -n 20 --no-pager 2>&1 | grep -iE "mirror|mattermost|connect|error|ready|listen" | tail -10
`)
}

func buzzMirrorStatusCommand() string {
	return strings.TrimSpace(`
set +e
printf '== chatd MM env (base URL present => mirror enabled) ==\n'
systemctl show chatd -p Environment 2>&1 | tr ' ' '\n' | grep -iE 'CHATD_MATTERMOST_BASE_URL|CHATD_BLUECLAW_INGRESS' || echo 'NO MM env (not enabled / rolled back)'
printf '== mirror drop-in present? ==\n'
test -f /etc/systemd/system/chatd.service.d/mirror.conf && echo 'drop-in PRESENT' || echo 'drop-in ABSENT (rolled back / not enabled)'
printf '== chatd state ==\n'
systemctl show chatd -p ActiveState,SubState,NRestarts 2>&1
printf '== chatd mirror journal ==\n'
journalctl -u chatd -n 60 --no-pager 2>&1 | grep -iE 'mirror|mattermost|puppet|connect|ready|error|ROLLED|adapters' | tail -18
`)
}

func buzzMembershipRecoverCommand() string {
	return strings.TrimSpace(`
set -e
mkdir -p /etc/systemd/system/` + blueclaw.BuzzRelayServiceName + `.service.d
cat > /etc/systemd/system/` + blueclaw.BuzzRelayServiceName + `.service.d/membership-recover.conf <<'DROPIN'
[Service]
Environment=BUZZ_REQUIRE_RELAY_MEMBERSHIP=false
Environment=BUZZ_RATE_LIMIT_HUMAN_MESSAGES_PER_MIN=1000000
Environment=BUZZ_RATE_LIMIT_HUMAN_API_CALLS_PER_MIN=1000000
Environment=BUZZ_RATE_LIMIT_HUMAN_WS_EVENTS_PER_SEC=100000
Environment=BUZZ_MEDIA_UPLOADS_PER_MINUTE=1000000
DROPIN
systemctl daemon-reload
systemctl restart ` + blueclaw.BuzzRelayServiceName + `
for attempt in $(seq 1 30); do curl -fsS --max-time 3 http://` + blueclaw.BuzzRelayBindAddress + `/_readiness >/dev/null 2>&1 && break; sleep 1; done
echo "== effective relay env =="; systemctl show ` + blueclaw.BuzzRelayServiceName + ` -p Environment | tr ' ' '\n' | grep -iE 'REQUIRE_RELAY|WS_EVENTS' || true
echo "relay permissive via drop-in — scheduling admind restart to resync staff membership"
systemd-run --on-active=3sec --unit=internkim-membership-admind-restart systemctl restart internkim-admind
echo "admind restart scheduled"
`)
}

func buzzRestoreCommand() string {
	return strings.TrimSpace(`
set -e
BACKUP=$(ls -t /root/.internkim/backups/buzz-*.sql 2>/dev/null | head -1)
if [ -z "$BACKUP" ]; then echo "NO BACKUP FOUND"; exit 1; fi
echo "restoring from $BACKUP ($(wc -c < "$BACKUP") bytes)"
systemctl stop ` + blueclaw.ChatdServiceName + ` 2>/dev/null || true
systemctl stop ` + blueclaw.BuzzRelayServiceName + `
su - postgres -c "dropdb --if-exists ` + blueclaw.BuzzRelayDatabaseName + ` && createdb -O ` + blueclaw.BuzzRelayDatabaseUser + ` ` + blueclaw.BuzzRelayDatabaseName + `"
cat "$BACKUP" | su - postgres -c "psql -q -d ` + blueclaw.BuzzRelayDatabaseName + `" >/dev/null 2>&1
rm -f /etc/systemd/system/` + blueclaw.BuzzRelayServiceName + `.service.d/membership-recover.conf
systemctl daemon-reload
systemctl start ` + blueclaw.BuzzRelayServiceName + `
for attempt in $(seq 1 40); do curl -fsS --max-time 3 http://` + blueclaw.BuzzRelayBindAddress + `/_readiness >/dev/null 2>&1 && break; sleep 1; done
systemctl start ` + blueclaw.ChatdServiceName + `
echo "restored from snapshot; relay+chatd started"
su - postgres -c "psql -X -qAt -d ` + blueclaw.BuzzRelayDatabaseName + ` -c \"SELECT count(*) FROM events WHERE kind=9\"" | sed 's/^/kind9 events: /'
`)
}

func buzzReimportCommand() string {
	relay := blueclaw.BuzzRelayServiceName
	chatd := blueclaw.ChatdServiceName
	database := blueclaw.BuzzRelayDatabaseName
	databaseUser := blueclaw.BuzzRelayDatabaseUser
	override := blueclaw.BuzzRelayImportOverrideEnvPath
	bind := blueclaw.BuzzRelayBindAddress
	marker := blueclaw.BuzzMigrateMarkerPath
	return strings.TrimSpace(`
set -e
mkdir -p /root/.internkim/backups
export SNAP="/root/.internkim/backups/buzz-$(date -u +%Y%m%dT%H%M%SZ).sql"
su - postgres -c "pg_dump ` + database + `" > "$SNAP"
echo "pre-reimport snapshot: $SNAP ($(wc -c < "$SNAP") bytes)"
systemctl stop ` + chatd + ` 2>/dev/null || true
systemctl stop ` + relay + `
{
  printf 'BUZZ_REQUIRE_RELAY_MEMBERSHIP=false\n'
  printf 'BUZZ_RATE_LIMIT_HUMAN_MESSAGES_PER_MIN=1000000\n'
  printf 'BUZZ_RATE_LIMIT_HUMAN_API_CALLS_PER_MIN=1000000\n'
  printf 'BUZZ_RATE_LIMIT_HUMAN_WS_EVENTS_PER_SEC=100000\n'
  printf 'BUZZ_MEDIA_UPLOADS_PER_MINUTE=1000000\n'
} > ` + override + `
chmod 600 ` + override + `
su - postgres -c "dropdb --if-exists ` + database + ` && createdb -O ` + databaseUser + ` ` + database + `"
systemctl daemon-reload
systemctl start ` + relay + `
for attempt in $(seq 1 40); do curl -fsS --max-time 3 http://` + bind + `/_readiness >/dev/null 2>&1 && break; sleep 1; done
echo "db wiped, relay in import mode (membership off)"
export MM_TOKEN=$(cat ` + blueclaw.BlueclawMattermostTokenPath + `)
export SEED=$(cat /root/.internkim/secrets/buzz-key-seed)
export DB_URL=$(grep '^DATABASE_URL=' ` + blueclaw.BuzzRelayDatabaseEnvironmentFilePath + ` | head -1 | sed 's/^DATABASE_URL=//')
export TEAM=$(curl -fsS -H "Authorization: Bearer $MM_TOKEN" ` + blueclaw.BlueclawMattermostLocalURL + `/api/v4/teams | jq -r '.[0].name')
DEVICE_HOST=$(sed -E 's#^[a-z]+://##; s#/.*$##' ` + blueclaw.DeviceURLFilePath + `)
case "$DEVICE_HOST" in
  *.*) export PUBLIC_HOST=$(printf '%s' "$DEVICE_HOST" | sed -E 's/\./-relay./') ;;
  *) export PUBLIC_HOST="${DEVICE_HOST}-relay" ;;
esac
export BUZZ_RELAY_PRIVATE_KEY=$(grep '^BUZZ_RELAY_PRIVATE_KEY=' /root/.internkim/secrets/buzz-relay-env | head -1 | sed 's/^BUZZ_RELAY_PRIVATE_KEY=//')
rm -f ` + marker + `
cat > /tmp/buzz-reimport-run.sh <<'RUNEOF'
export DATABASE_URL="$DB_URL"
` + blueclaw.BuzzMigrateBinaryPath + ` \
  --mattermost-url ` + blueclaw.BlueclawMattermostLocalURL + ` \
  --mattermost-token "$MM_TOKEN" \
  --team "$TEAM" \
  --buzz-database-url "$DB_URL" \
  --buzz-admin ` + blueclaw.BuzzAdminBinaryPath + ` \
  --key-seed "$SEED" \
  --relay-url wss://$PUBLIC_HOST \
  --relay-http-url https://$PUBLIC_HOST \
  --community-host "$PUBLIC_HOST" \
  --orphan-root-title "이전 대화" > /tmp/buzz-migrate.log 2>&1
RC=$?
COUNT=$(su - postgres -c "psql -X -qAt -d ` + database + ` -c \"SELECT count(*) FROM events WHERE kind=9\"" 2>/dev/null)
COUNT=${COUNT:-0}
echo "buzz-migrate rc=$RC imported kind9=$COUNT"
if [ "$RC" -ne 0 ] || [ "$COUNT" -lt 50 ]; then
  echo "IMPORT FAILED (rc=$RC kind9=$COUNT) — auto-restoring $SNAP"
  systemctl stop ` + relay + `
  su - postgres -c "dropdb --if-exists ` + database + ` && createdb -O ` + databaseUser + ` ` + database + `"
  cat "$SNAP" | su - postgres -c "psql -q -d ` + database + `" >/dev/null 2>&1
  rm -f ` + override + `
  systemctl daemon-reload
  systemctl start ` + relay + `
  for attempt in $(seq 1 40); do curl -fsS --max-time 3 http://` + bind + `/_readiness >/dev/null 2>&1 && break; sleep 1; done
  systemctl start ` + chatd + `
  echo REIMPORT_FAILED_AUTORESTORED
  exit 0
fi
touch ` + marker + `
rm -f ` + override + `
systemctl restart ` + relay + `
sleep 2
systemctl start ` + chatd + `
echo REIMPORT_DONE_OK
RUNEOF
chmod 700 /tmp/buzz-reimport-run.sh
setsid nohup bash /tmp/buzz-reimport-run.sh > /tmp/buzz-reimport.log 2>&1 < /dev/null &
sleep 1
echo "reimport launched (self-healing: auto-restores $SNAP if import fails) — poll with buzz-reimport-log"
`)
}

func mattermostForceDbnameCommand() string {
	return strings.TrimSpace(`
set +e
echo "== files referencing mattermost_test (before) =="
grep -rl 'mattermost_test' /opt/mattermost/config /etc/systemd/system /lib/systemd/system /etc/default /etc/mattermost 2>/dev/null | grep -vE 'pgbak|\.bak' | tee /tmp/mmref.txt
echo "== replacing mattermost_test -> mattermost =="
while IFS= read -r f; do [ -f "$f" ] || continue; cp -a "$f" "$f.pgbak" 2>/dev/null; sed -i 's/mattermost_test/mattermost/g' "$f"; echo "fixed: $f"; done < /tmp/mmref.txt
rm -f /tmp/mmref.txt
echo "== remaining refs (should be empty) =="
grep -rl 'mattermost_test' /opt/mattermost/config /etc/systemd/system /lib/systemd/system /etc/default /etc/mattermost 2>/dev/null | grep -vE 'pgbak|\.bak'
systemctl daemon-reload
echo "== restart mattermost =="
timeout 30 systemctl restart mattermost.service 2>&1; echo "rc=$?"
echo "done (check status: mattermost-8065)"
`)
}

func mattermostFixDbnameCommand() string {
	return strings.TrimSpace(`
set +e
CFG=/opt/mattermost/config/config.json
cp -a "$CFG" "$CFG.bak-$(date -u +%Y%m%dT%H%M%SZ)" 2>&1 | sed 's/^/backup: /'
echo "before: $(grep -oE '"DataSource": *"[^"]*"' "$CFG" | head -1 | sed -E 's#://[^:]+:[^@]+@#://U:P@#')"
python3 - "$CFG" <<'PYEOF'
import json, sys
cfg = sys.argv[1]
d = json.load(open(cfg))
sql = d.get('SqlSettings', {})
changed = 0
for k in ['DataSource', 'DataSourceReplicas', 'DataSourceSearchReplicas']:
    v = sql.get(k)
    if isinstance(v, str) and 'mattermost_test' in v:
        sql[k] = v.replace('mattermost_test', 'mattermost'); changed += 1
    elif isinstance(v, list):
        nv = [x.replace('mattermost_test', 'mattermost') if isinstance(x, str) else x for x in v]
        if nv != v: sql[k] = nv; changed += 1
json.dump(d, open(cfg, 'w'), indent=4)
print("fields changed:", changed)
PYEOF
echo "after: $(grep -oE '"DataSource": *"[^"]*"' "$CFG" | head -1 | sed -E 's#://[^:]+:[^@]+@#://U:P@#')"
echo "== restart mattermost =="
timeout 30 systemctl restart mattermost.service 2>&1; echo "rc=$?"
echo "done (check status: mattermost-8065)"
`)
}

func mattermostDbResyncCommand() string {
	return strings.TrimSpace(`
set +e
CFG=/opt/mattermost/config/config.json
[ -f "$CFG" ] || CFG=$(ls -t /opt/mattermost/config/config.json /mattermost/config/config.json 2>/dev/null | head -1)
echo "config: ${CFG:-NOT FOUND}"
python3 - "$CFG" > /tmp/mm-resync.sql 2>/tmp/mm-resync.err <<'PYEOF'
import json, sys
from urllib.parse import urlparse, unquote
ds = json.load(open(sys.argv[1]))['SqlSettings']['DataSource']
u = urlparse(ds)
user = (u.username or 'mmuser').replace('"','""')
pw = unquote(u.password or '')
sys.stderr.write("db_user=%s host=%s db=%s\n" % (u.username, u.hostname, (u.path or '').lstrip('/')))
print('ALTER USER "%s" WITH PASSWORD \'%s\';' % (user, pw.replace("'", "''")))
PYEOF
cat /tmp/mm-resync.err
chmod 644 /tmp/mm-resync.sql
echo "== ALTER mmuser password to match config =="
su - postgres -c "psql -X -d postgres -f /tmp/mm-resync.sql" 2>&1 | sed 's/^/psql: /'
rm -f /tmp/mm-resync.sql /tmp/mm-resync.err
echo "== restart mattermost =="
timeout 25 systemctl restart mattermost.service 2>&1; echo "rc=$?"
echo "done (check status: mattermost-8065)"
`)
}

func mattermostRestartCommand() string {
	return strings.TrimSpace(`
set +e
UNIT=$(systemctl list-unit-files --no-legend 2>/dev/null | awk '{print $1}' | grep -iE 'mattermost' | head -1)
CT=$(timeout 5 docker ps -a --format '{{.Names}}|{{.Status}}' 2>/dev/null | grep -iE 'mattermost' | head -1)
echo "unit=${UNIT:-none} container=${CT:-none}"
ps -eo comm,pid 2>/dev/null | grep -iE 'mattermost' | head -2
if [ -n "$UNIT" ]; then
  echo "state=$(systemctl is-active "$UNIT" 2>&1)"
  echo "why: $(timeout 8 journalctl -u "$UNIT" -n 4 --no-pager 2>&1 | tail -4 | tr '\n' '~')"
  timeout 25 systemctl restart "$UNIT" 2>&1; echo "restart_rc=$?"
elif [ -n "$CT" ]; then
  CN=$(printf '%s' "$CT" | cut -d'|' -f1)
  timeout 25 docker restart "$CN" 2>&1; echo "docker_restart_rc=$?"
else
  echo "NO_MM_UNIT_OR_CONTAINER_FOUND"
  systemctl list-unit-files --no-legend 2>/dev/null | awk '{print $1}' | grep -iE 'mm|chat|mattermost' | head
fi
echo "done (check status for mattermost-8065)"
`)
}

func postgresRepairCommand() string {
	return strings.TrimSpace(`
set +e
echo "== disk before =="; df -h / 2>&1 | tail -1
echo "== backups =="; du -sh /root/.internkim/backups 2>/dev/null; ls -laS /root/.internkim/backups/*.sql 2>/dev/null | head -6
echo "== postgres log (crash reason) =="; journalctl -u 'postgresql@*' -n 15 --no-pager 2>&1 | tail -15
echo "== free disk: old deploy snapshots + old daily/buzz backups (keep newest of each) =="
rm -f -v /root/.internkim/backups/preserved-*.tar.gz /root/.internkim/backups/preserved-*.gzip-status /root/.internkim/backups/preserved-*.gzip-test.log
ls -t /root/.internkim/backups/.internkim-backup-*.tar.gz 2>/dev/null | tail -n +2 | xargs -r rm -f -v
ls -t /root/.internkim/backups/buzz-*.sql 2>/dev/null | tail -n +2 | xargs -r rm -f -v
echo "== disk after cleanup =="; df -h / 2>&1 | tail -1
echo "== restart postgres =="; timeout 50 systemctl restart postgresql 2>&1; echo "restart rc=$?"
sleep 2
echo "== verify =="; timeout 8 su - postgres -c "psql -X -qAt -c 'SELECT 1'" 2>&1 | sed 's/^/psql SELECT 1: /'
echo "== disk final =="; df -h / 2>&1 | tail -1
`)
}

func mattermostUnlockUsersCommand() string {
	return strings.TrimSpace(`
set -eu
su - postgres -c "psql -X -qAt -c \"SELECT datname FROM pg_database WHERE datistemplate = false\"" | while IFS= read -r database; do
  [ -n "$database" ] || continue
  escaped_database=$(printf "%s" "$database" | sed "s/'/'\\\\''/g")
  has_users=$(su - postgres -c "psql -X -qAt -d '$escaped_database' -c \"SELECT to_regclass('public.users') IS NOT NULL AND EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='failedattempts')\"" 2>/dev/null)
  [ "$has_users" = "t" ] || continue
  printf "== %s: locked users (failedattempts>0) before ==\n" "$database"
  su - postgres -c "psql -X -qAt -d '$escaped_database' -c \"SELECT email || ' failed=' || failedattempts || ' del=' || deleteat || ' auth=' || COALESCE(NULLIF(authservice,''),'password') FROM users WHERE failedattempts > 0 AND deleteat = 0 ORDER BY failedattempts DESC\"" 2>/dev/null
  printf "== lee@example.com account state ==\n"
  su - postgres -c "psql -X -qAt -d '$escaped_database' -c \"SELECT email || ' failed=' || failedattempts || ' del=' || deleteat || ' auth=' || COALESCE(NULLIF(authservice,''),'password') FROM users WHERE lower(email)='lee@example.com'\"" 2>/dev/null
  updated=$(su - postgres -c "psql -X -qAt -d '$escaped_database' -c \"UPDATE users SET failedattempts = 0 WHERE deleteat = 0 AND failedattempts > 0 RETURNING email\"" 2>/dev/null | wc -l)
  printf "== %s: reset failedattempts for %s user(s) ==\n" "$database" "$updated"
done
`)
}

func buzzChatdRepairCommand() string {
	chatd := blueclaw.ChatdServiceName
	return strings.TrimSpace(`
set +e
mkdir -p /etc/systemd/system/` + chatd + `.service.d
rm -f /etc/systemd/system/` + chatd + `.service.d/tls-debug.conf
cat > /etc/systemd/system/` + chatd + `.service.d/tls.conf <<'DROPIN'
[Service]
Environment=NODE_TLS_REJECT_UNAUTHORIZED=0
DROPIN
systemctl daemon-reload
systemctl restart ` + chatd + `
for attempt in $(seq 1 15); do ss -ltn 2>/dev/null | grep -q ':18090' && break; sleep 1; done
ss -ltn 2>/dev/null | grep -q ':18090' && echo ':18090 LISTENING (chatd serving)' || echo ':18090 STILL DOWN'
echo "== chatd journal (last 20, with relay debug) =="
journalctl -u ` + chatd + ` -n 20 --no-pager 2>&1 | tail -20
`)
}

func buzzReadTestCommand() string {
	return strings.TrimSpace(`
set +e
ADMIN_TOKEN=$(cat /root/.internkim/secrets/mattermost-bot-token 2>/dev/null)
EMAIL=$(curl -fsS -H "Authorization: Bearer $ADMIN_TOKEN" "` + blueclaw.BlueclawMattermostLocalURL + `/api/v4/users?per_page=60&active=true" 2>/dev/null | jq -r '[.[] | select(.is_bot|not) | .email] | .[0]')
echo "test user email: $EMAIL"
echo "== /agent/api/channels (what the web lists) =="
curl -fsS -H "X-Forwarded-Email: $EMAIL" "http://127.0.0.1:18080/agent/api/channels" 2>&1 | jq '{count: (.conversations|length), names: [.conversations[]?.name][0:8]}' 2>/dev/null || curl -sS -H "X-Forwarded-Email: $EMAIL" "http://127.0.0.1:18080/agent/api/channels" 2>&1 | head -c 400
echo
echo "== chatd conversations.list direct (raw status) =="
SEC=$(curl -fsS "http://127.0.0.1:18080/bridge/api/identity?email=$EMAIL" 2>/dev/null | jq -r .secretHex)
curl -sS -o /dev/null -w "http %{http_code}\n" -X POST "` + blueclaw.ChatdEndpoint + `/v1/platform/buzz/conversations.list" -H "Content-Type: application/json" -d "{\"userSecretHex\":\"$SEC\"}" 2>&1
echo "== chatd :18090 listening? =="
ss -ltn 2>/dev/null | grep -q ':18090' && echo ':18090 LISTENING' || echo ':18090 NOT listening'
echo "== chatd journal (last 10) =="
journalctl -u ` + blueclaw.ChatdServiceName + ` -n 10 --no-pager 2>&1 | tail -10
`)
}

func buzzReimportLogCommand() string {
	return strings.TrimSpace(`
echo "=== /tmp/buzz-reimport.log ==="
tail -40 /tmp/buzz-reimport.log 2>/dev/null || echo "(no reimport log)"
echo "=== /tmp/buzz-migrate.log (tail) ==="
tail -40 /tmp/buzz-migrate.log 2>/dev/null || echo "(no buzz-migrate log)"
echo "=== current kind9 count ==="
su - postgres -c "psql -X -qAt -d ` + blueclaw.BuzzRelayDatabaseName + ` -c \"SELECT count(*) FROM events WHERE kind=9\"" 2>/dev/null | sed 's/^/kind9: /'
`)
}

func buzzRepairCommand(apply bool) string {
	applyValue := "false"
	if apply {
		applyValue = "true"
	}
	return strings.TrimSpace(`
body=$(curl -sS -X POST "http://127.0.0.1:18080/agent/api/buzz-repair-orphans?apply=` + applyValue + `")
printf '%s\n' "$body" | jq . 2>/dev/null || printf '%s\n' "$body"
`)
}

func buzzSnapshotCommand() string {
	return strings.TrimSpace(`
set -e
mkdir -p /root/.internkim/backups
out="/root/.internkim/backups/buzz-$(date -u +%Y%m%dT%H%M%SZ).sql"
su - postgres -c "pg_dump buzz" > "$out"
echo "wrote $out ($(wc -c < "$out") bytes)"
ls -la /root/.internkim/backups/ | tail -6
`)
}

func buzzOrphanInspectCommand() string {
	return strings.TrimSpace(`
set +e
q() { su - postgres -c "psql -X -d buzz -c \"$1\"" 2>&1; }
printf '== events schema ==\n';            q "\\d events"
printf '== thread_metadata schema ==\n';   q "\\d thread_metadata"
printf '== total synthetic 이전 대화 roots ==\n'
q "SELECT count(*) FROM events WHERE content='이전 대화'"
printf '== authoring pubkeys ==\n'
q "SELECT encode(pubkey,'hex'), count(*) FROM events WHERE content='이전 대화' GROUP BY 1 ORDER BY 2 DESC LIMIT 5"
printf '== kinds ==\n'
q "SELECT kind, count(*) FROM events WHERE content='이전 대화' GROUP BY 1"
printf '== synthetic roots per channel (top 20) ==\n'
q "SELECT channel_id::text, count(*) FROM events WHERE content='이전 대화' GROUP BY 1 ORDER BY 2 DESC LIMIT 20"
printf '== event kinds (9=msg 39002=channel-membership 13534=relay-membership 0=profile) ==\n'
q "SELECT kind, count(*) FROM events GROUP BY kind ORDER BY 2 DESC"
printf '== communities (id, host) ==\n'
q "SELECT id::text, host FROM communities"
printf '== events by community ==\n'
q "SELECT community_id::text, count(*) FROM events WHERE kind=9 GROUP BY 1"
printf '== channels count + sample ==\n'
q "SELECT community_id::text, count(*) FROM channels GROUP BY 1"
printf '== relay effective REQUIRE_RELAY_MEMBERSHIP ==\n'
systemctl show ` + blueclaw.BuzzRelayServiceName + ` -p Environment 2>&1 | tr ' ' '\n' | grep -iE 'REQUIRE_RELAY|COMMUNITY_HOST|BUZZ_COMMUNITY' || echo '(none set => production default)'
printf '== total kind9 events ==\n'
q "SELECT count(*) FROM events WHERE kind=9"
printf '== all kind9 authoring pubkeys (top 15) ==\n'
q "SELECT encode(pubkey,'hex'), count(*) FROM events WHERE kind=9 GROUP BY 1 ORDER BY 2 DESC LIMIT 15"
printf '== bootstrap-authored (67ac03f0) non-orphan messages ==\n'
q "SELECT count(*) FROM events WHERE kind=9 AND pubkey=decode('67ac03f0279bb2523574f1be46695f1f5d40d6d51626b555483ec2edac17307d','hex') AND content<>'이전 대화'"
printf '== recent 15 kind9 events (author short + content) ==\n'
q "SELECT to_char(created_at,'MM-DD HH24:MI'), substr(encode(pubkey,'hex'),1,8), left(content,45) FROM events WHERE kind=9 ORDER BY created_at DESC LIMIT 15"
`)
}

func (service *Service) sshRecoveryServiceStates(ctx context.Context) map[string]string {
	return map[string]string{
		"ssh":                  service.sshRecoveryCommandOutput(ctx, "systemctl", "is-active", "ssh"),
		"cloudflared-node-ssh": service.sshRecoveryCommandOutput(ctx, "systemctl", "is-active", "cloudflared-node-ssh"),
		"cloudflared":          service.sshRecoveryCommandOutput(ctx, "systemctl", "is-active", "cloudflared"),
		"blueclaw":             service.sshRecoveryCommandOutput(ctx, "systemctl", "is-active", blueclaw.BlueclawServiceName),
		"buzz-relay":           service.sshRecoveryCommandOutput(ctx, "systemctl", "is-active", blueclaw.BuzzRelayServiceName),
		"buzz-relay-stunnel":   service.sshRecoveryCommandOutput(ctx, "systemctl", "is-active", "buzz-relay-stunnel"),
		"chatd":                service.sshRecoveryCommandOutput(ctx, "systemctl", "is-active", "chatd"),
		"relay-tls-443":        service.sshRecoveryCommandOutput(ctx, "sh", "-lc", "ss -ltn 2>/dev/null | grep -q ':443' && echo listening || echo down"),
		"mattermost-8065":      service.sshRecoveryCommandOutput(ctx, "sh", "-lc", "curl -fsS --max-time 3 http://127.0.0.1:8065/api/v4/system/ping >/dev/null 2>&1 && echo up || echo down"),
		"mattermost-how":       service.sshRecoveryCommandOutput(ctx, "sh", "-lc", "u=$(systemctl list-unit-files --no-legend 2>/dev/null | awk '{print $1}' | grep -i mattermost | head -1); c=$(timeout 4 docker ps -a --format '{{.Names}}={{.Status}}' 2>/dev/null | grep -i mattermost | head -1); echo \"unit=${u:-none} ct=${c:-none}\""),
		"mattermost-state":     service.sshRecoveryCommandOutput(ctx, "sh", "-lc", "u=$(systemctl list-unit-files --no-legend 2>/dev/null | awk '{print $1}' | grep -i mattermost | head -1); [ -n \"$u\" ] && systemctl is-active \"$u\" 2>&1 || echo no-unit"),
		"mattermost-why":       service.sshRecoveryCommandOutput(ctx, "sh", "-lc", "L=$(ls -t /opt/mattermost/logs/mattermost.log 2>/dev/null | head -1); { tail -40 \"$L\" 2>/dev/null; timeout 6 journalctl -u mattermost.service -n 30 --no-pager 2>/dev/null | grep -v 'systemd\\['; } | grep -iE 'error|fatal|critical|panic|unable|refused|migrat|corrupt|no space|permission denied|too many|listen' | tail -1 | cut -c1-240"),
		"disk-root":            service.sshRecoveryCommandOutput(ctx, "sh", "-lc", "df -h / | awk 'NR==2{print $5\" used, \"$4\" free\"}'"),
		"postgres-dbs":         service.sshRecoveryCommandOutput(ctx, "sh", "-lc", "timeout 6 su - postgres -c \"psql -X -qAt -c 'SELECT datname FROM pg_database WHERE datistemplate=false'\" 2>&1 | tr '\\n' ' '"),
		"pg-clusters":          service.sshRecoveryCommandOutput(ctx, "sh", "-lc", "pg_lsclusters --no-header 2>/dev/null | awk '{print $1\"/\"$2\":\"$4}' | tr '\\n' ' '"),
		"mm-config-db":         service.sshRecoveryCommandOutput(ctx, "sh", "-lc", "grep -oE '\\\"DataSource\\\": *\\\"[^\\\"]*\\\"' /opt/mattermost/config/config.json 2>/dev/null | head -1 | sed -E 's#://[^:]+:[^@]+@#://USER:PASS@#'"),
		"mm-db-data":           service.sshRecoveryCommandOutput(ctx, "sh", "-lc", "timeout 6 su - postgres -c \"psql -X -qAt -d mattermost -c \\\"SELECT 'users='||count(*) FROM users\\\"; psql -X -qAt -d mattermost -c \\\"SELECT 'posts='||count(*) FROM posts\\\"\" 2>&1 | tr '\\n' ' '"),
		"mm-env-ds":            service.sshRecoveryCommandOutput(ctx, "sh", "-lc", "{ systemctl show mattermost.service -p Environment -p EnvironmentFiles 2>/dev/null | tr ' ' '\\n' | grep -iE 'DATASOURCE|EnvironmentFiles'; for f in $(systemctl show mattermost.service -p EnvironmentFiles 2>/dev/null | sed 's/EnvironmentFiles=//' | tr ' ' '\\n' | sed 's/^-//'); do grep -h DATASOURCE \"$f\" 2>/dev/null; done; } | grep -iE 'test|datasource|Environment' | sed -E 's#://[^:]+:[^@]+@#://U:P@#' | head -3 | tr '\\n' '  '"),
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
