package admind

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
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
	response := service.runSSHRecovery(request.Context(), payload.Action, payload.Target)
	service.writeJSON(responseWriter, response)
}

func (service *Service) validateSSHRecoveryRequest(payload sshRecoveryRequest) error {
	return service.validateFleetSignedRequest(payload.fleetSignedRequest, isAllowedSSHRecoveryAction)
}

func isAllowedSSHRecoveryAction(action string) bool {
	switch action {
	case "status", "snapshot", "restart-ssh", "restart-cloudflared-node-ssh", "journal-tail", "unlock-mattermost-admin", "reboot", "stop-tenant-pilots", "remove-tenant-pilots", "limit-blueclaw", "restart-blueclaw", "blueclaw-boot-diagnose", "blueclaw-journal", "blueclaw-workspace-repair", "blueclaw-postgres-salvage", "blueclaw-postgres-inspect", "blueclaw-postgres-restore-previous", "repair-buzz-relay", "buzz-relay-journal", "enable-buzz-mirror", "buzz-mirror-status", "retire-mattermost-mirror", "stop-mattermost", "calendar-record-coverage", "calendar-carry-into-the-record", "organization-directory-coverage", "organization-seed-the-directory", "buzz-device-link-count", "buzz-rewrite-old-links", "buzz-rewrite-old-links-dryrun", "buzz-named-reaction-count", "buzz-orphan-inspect", "buzz-stranger-members", "buzz-stranger-members-remove", "buzz-profile-inspect", "buzz-probe-profile-count", "buzz-probe-profile-purge", "buzz-reconcile-channels", "buzz-republish-rooms", "buzz-restore-dm-discovery", "buzz-channel-visibility", "buzz-channel-visibility-repair", "buzz-close-channels-their-room-closed", "buzz-channel-members-their-room-lacks", "buzz-remove-members-their-room-lacks", "buzz-rooms-nobody-is-in", "buzz-retire-rooms-nobody-is-in", "buzz-retire-room", "circle-membership-read", "circle-membership-reconcile", "circle-room-read", "circle-room-reconcile", "buzz-whose-key", "buzz-snapshot", "buzz-membership-recover", "buzz-restore", "buzz-repair-dryrun", "buzz-repair-apply", "buzz-reimport", "buzz-refresh-profiles", "buzz-reimport-log", "buzz-read-test", "policy-circle-roster", "buzz-room-roster", "buzz-room-visibility", "buzz-close-rooms-except", "buzz-rename-room", "buzz-retire-room-by-name", "admind-journal", "buzz-chatd-repair", "mattermost-unlock-users", "postgres-repair", "release-setup-lock":
		return true
	default:
		return false
	}
}

func (service *Service) runSSHRecovery(ctx context.Context, action string, actionTarget string) sshRecoveryResponse {
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
	case "release-setup-lock":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "release a setup lock nobody holds", "sh", "-lc", releaseSetupLockCommand()))
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
	case "blueclaw-postgres-inspect":
		inspectContext, cancelInspect := context.WithTimeout(context.Background(), 600*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(inspectContext, "preserve the workspace image and read the guest cluster", "sh", "-lc", blueclawPostgresInspectCommand()))
		cancelInspect()
	case "blueclaw-postgres-restore-previous":
		restoreContext, cancelRestore := context.WithTimeout(context.Background(), 900*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(restoreContext, "restore the guest cluster from the previous workspace image", "sh", "-lc", blueclawPostgresRestorePreviousCommand()))
		cancelRestore()
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
	case "retire-mattermost-mirror":
		retireMirrorContext, cancelRetireMirror := context.WithTimeout(context.Background(), 90*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(retireMirrorContext, "leave chatd holding Buzz alone", "sh", "-lc", mattermostMirrorRetireCommand()))
		cancelRetireMirror()
	case "stop-mattermost":
		stopContext, cancelStop := context.WithTimeout(context.Background(), 90*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(stopContext, "stop Mattermost and leave it stopped", "sh", "-lc", mattermostStopCommand()))
		cancelStop()
	case "buzz-whose-key":
		whoseKeyContext, cancelWhoseKey := context.WithTimeout(context.Background(), 300*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(whoseKeyContext, "name whose key this is, or say nothing here made it", "sh", "-lc", buzzWhoseKeyCommand(actionTarget)))
		cancelWhoseKey()
	case "circle-room-read":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "read who each circle room holds and who its circle holds", "sh", "-lc", circleRoomMembershipCommand(false)))
	case "circle-room-reconcile":
		circleRoomContext, cancelCircleRoom := context.WithTimeout(context.Background(), 600*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(circleRoomContext, "make each circle room hold exactly its circle", "sh", "-lc", circleRoomMembershipCommand(true)))
		cancelCircleRoom()
	case "circle-membership-read":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "read the circles this person carries", "sh", "-lc", circleMembershipReconcileCommand(actionTarget, false)))
	case "circle-membership-reconcile":
		circleContext, cancelCircle := context.WithTimeout(context.Background(), 300*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(circleContext, "make the messenger hold the circles this person carries", "sh", "-lc", circleMembershipReconcileCommand(actionTarget, true)))
		cancelCircle()
	case "buzz-retire-room":
		namedContext, cancelNamed := context.WithTimeout(context.Background(), 600*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(namedContext, "retire the room this action names and its buzz mirror", "sh", "-lc", buzzRetireNamedRoomCommand(actionTarget)))
		cancelNamed()
	case "buzz-rooms-nobody-is-in":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "count the private rooms nobody is in", "sh", "-lc", buzzChannelRetireCommand(false)))
	case "buzz-retire-rooms-nobody-is-in":
		retireContext, cancelRetire := context.WithTimeout(context.Background(), 600*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(retireContext, "retire the private rooms nobody is in and their buzz mirrors", "sh", "-lc", buzzChannelRetireCommand(true)))
		cancelRetire()
	case "buzz-channel-members-their-room-lacks":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "count the buzz channel members the room it mirrors does not hold", "sh", "-lc", buzzChannelMembershipRepairCommand(false)))
	case "buzz-remove-members-their-room-lacks":
		removeContext, cancelRemove := context.WithTimeout(context.Background(), 600*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(removeContext, "remove the buzz channel members the room it mirrors does not hold", "sh", "-lc", buzzChannelMembershipRepairCommand(true)))
		cancelRemove()
	case "buzz-channel-visibility-repair":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "count the buzz channels opener than the room they mirror", "sh", "-lc", buzzChannelVisibilityRepairCommand(false)))
	case "buzz-close-channels-their-room-closed":
		closeContext, cancelClose := context.WithTimeout(context.Background(), 600*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(closeContext, "close the buzz channels whose room is not open", "sh", "-lc", buzzChannelVisibilityRepairCommand(true)))
		cancelClose()
	case "buzz-channel-visibility":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "count what the community can read", "sh", "-lc", buzzChannelVisibilityCommand()))
	case "buzz-restore-dm-discovery":
		restoreContext, cancelRestore := context.WithTimeout(context.Background(), 900*time.Second)
		restoreCommand, restoreError := buzzRestoreDirectMessageDiscoveryCommand(actionTarget)
		response.Results = append(response.Results, service.runNamedRoomCommand(restoreContext, "give every direct message back the names it showed", restoreCommand, restoreError))
		cancelRestore()
	case "buzz-republish-rooms":
		republishContext, cancelRepublish := context.WithTimeout(context.Background(), 300*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(republishContext, "tell every client what each room is called and how open it is", "sh", "-lc", buzzRepublishRoomsCommand()))
		cancelRepublish()
	case "buzz-reconcile-channels":
		reconcileContext, cancelReconcile := context.WithTimeout(context.Background(), 120*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(reconcileContext, "emit the discovery events a nostr client needs to see a channel", "sh", "-lc", buzzReconcileChannelsCommand()))
		cancelReconcile()
	case "buzz-orphan-inspect":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "inspect imported orphan-thread roots", "sh", "-lc", buzzOrphanInspectCommand()))
	case "buzz-stranger-members":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "count members the directory does not name", "sh", "-lc", buzzStrangerMemberCommand(false)))
	case "buzz-stranger-members-remove":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "remove members the directory does not name", "sh", "-lc", buzzStrangerMemberCommand(true)))
	case "organization-directory-coverage":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "count what the company directory knows about people", "sh", "-lc", organizationDirectoryCoverageCommand(false)))
	case "organization-seed-the-directory":
		seedContext, cancelSeed := context.WithTimeout(context.Background(), 600*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(seedContext, "give the company directory the profiles this device holds", "sh", "-lc", organizationDirectoryCoverageCommand(true)))
		cancelSeed()
	case "calendar-record-coverage":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "count the calendar events the record holds", "sh", "-lc", calendarRecordCoverageCommand(false)))
	case "calendar-carry-into-the-record":
		carryContext, cancelCarry := context.WithTimeout(context.Background(), 600*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(carryContext, "carry the calendar events the record never took", "sh", "-lc", calendarRecordCoverageCommand(true)))
		cancelCarry()
	case "buzz-rewrite-old-links":
		rewriteContext, cancelRewrite := context.WithTimeout(context.Background(), 600*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(rewriteContext, "rewrite the links in messages already sent", "sh", "-lc", buzzRewriteOldLinksCommand(true)))
		cancelRewrite()
	case "buzz-rewrite-old-links-dryrun":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "count the messages a rewrite would change", "sh", "-lc", buzzRewriteOldLinksCommand(false)))
	case "buzz-device-link-count":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "count messages carrying a device link", "sh", "-lc", buzzDeviceLinkCountCommand()))
	case "buzz-named-reaction-count":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "count reactions published as a name", "sh", "-lc", buzzNamedReactionCountCommand()))
	case "buzz-profile-inspect":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "inspect the pictures published profiles point at", "sh", "-lc", buzzProfileInspectCommand()))
	case "buzz-probe-profile-count":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "count the profiles left by deleted probe accounts", "sh", "-lc", buzzProbeProfilePurgeCommand(false)))
	case "buzz-probe-profile-purge":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "delete the profiles left by deleted probe accounts", "sh", "-lc", buzzProbeProfilePurgeCommand(true)))
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
	case "buzz-refresh-profiles":
		profileContext, cancelProfiles := context.WithTimeout(context.Background(), 180*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(profileContext, "publish the names and faces without importing anything", "sh", "-lc", buzzRefreshProfilesCommand()))
		cancelProfiles()
	case "buzz-reimport-log":
		logContext, cancelLog := context.WithTimeout(context.Background(), 60*time.Second)
		response.Results = append(response.Results, service.runSSHRecoveryCommand(logContext, "tail Buzz re-import logs", "sh", "-lc", buzzReimportLogCommand()))
		cancelLog()
	case "admind-journal":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "read what admind said about the roster it delivers", "sh", "-lc", admindJournalCommand()))
	case "buzz-room-visibility":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "read how open each room is", "sh", "-lc", buzzRoomVisibilityCommand()))
	case "buzz-close-rooms-except":
		closeCommand, closeError := buzzCloseRoomsExceptCommand(strings.Split(actionTarget, ","))
		response.Results = append(response.Results, service.runNamedRoomCommand(ctx, "close every room but the ones this action names", closeCommand, closeError))
	case "buzz-rename-room":
		renameCommand, renameError := buzzRenameRoomCommand(actionTarget)
		response.Results = append(response.Results, service.runNamedRoomCommand(ctx, "rename the room this action names", renameCommand, renameError))
	case "buzz-retire-room-by-name":
		retireCommand, retireError := buzzRetireRoomByNameCommand(actionTarget)
		response.Results = append(response.Results, service.runNamedRoomCommand(ctx, "retire the room this action names, bridged or not", retireCommand, retireError))
	case "buzz-room-roster":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "read who each room holds", "sh", "-lc", buzzRoomRosterCommand()))
	case "policy-circle-roster":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "read the circles the policy declares and who carries them", "sh", "-lc", policyCircleRosterCommand()))
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
newestLogDirectories=$(find /var/log/blueclaw-supervisor -maxdepth 1 -mindepth 1 -type d -newermt '-3 minutes' 2>/dev/null | head -4)
if [ -z "$newestLogDirectories" ]; then
  newestLogDirectories=$(find /var/log/blueclaw-supervisor -maxdepth 1 -mindepth 1 -type d -newermt '-2 hours' 2>/dev/null | head -4)
fi
printf '%s\n' "$newestLogDirectories"
printf 'total run directories: %s\n' "$(find /var/log/blueclaw-supervisor -maxdepth 1 -mindepth 1 -type d 2>/dev/null | wc -l)"
for logDirectory in $(printf '%s\n' "$newestLogDirectories" | head -2); do
  printf '\n== %s stderr.log ==\n' "$logDirectory"
  tail -c 4000 "$logDirectory/stderr.log" 2>/dev/null || printf '(missing)\n'
  printf '\n== %s stdout.log ==\n' "$logDirectory"
  tail -c 4000 "$logDirectory/stdout.log" 2>/dev/null || printf '(missing)\n'
done
newestJailerRoot=$(ls -dt /var/lib/bc/firecracker/*/root 2>/dev/null | head -1)
printf '\n== live guest task runs ==\n'
curl -s -m 6 http://127.0.0.1:8080/admin/api/run 2>&1 | head -c 1500
printf '\n== live guest failure detail ==\n'
failedTaskRunID=$(curl -s -m 6 http://127.0.0.1:8080/admin/api/run 2>/dev/null | tr ',' '\n' | grep -A0 'taskRunID' | head -1 | sed 's/.*"taskRunID":"//;s/".*//')
if [ -n "$failedTaskRunID" ]; then
  curl -s -m 8 "http://127.0.0.1:8080/admin/api/run/detail?taskRunID=$failedTaskRunID" 2>&1 | tr ',' '\n' | grep -E '"name":|"taskEventID":' | head -n 120
fi
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
printf '\n== delivered runtime ==\n'
ls -la ` + quoteBlueclawUpdateShellValue(blueclaw.BlueclawDeliveryRuntimePath) + ` 2>&1 | head -8
cat ` + quoteBlueclawUpdateShellValue(blueclaw.BlueclawDeliveryRuntimePath+"/manifest.json") + ` 2>&1 | head -12
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

// Blueclaw is left stopped on purpose: a guest whose postgres cannot reach a
// checkpoint kills its own init and panics the kernel every two minutes, and
// each cycle is another unclean shutdown of the cluster being diagnosed.
func blueclawPostgresRestorePreviousCommand() string {
	return strings.TrimSpace(`
set +e
image=/var/lib/blueclaw/workspace.ext4
previous=/var/lib/blueclaw/workspace.ext4.previous
dataInsideImage=.blueclaw/postgres/data
rollback=/var/lib/blueclaw/postgres-data-before-restore-$(date -u +%Y%m%dT%H%M%SZ)
[ -e "$previous" ] || { echo "no previous image at $previous"; exit 1; }
echo "== stop blueclaw =="; systemctl stop blueclaw 2>&1; sleep 3
previousMount=$(mktemp -d); currentMount=$(mktemp -d)
mount -o ro,noload,loop "$previous" "$previousMount" 2>&1 || { echo "previous mount FAILED"; exit 1; }
mount -o loop "$image" "$currentMount" 2>&1 || { echo "current mount FAILED"; umount "$previousMount"; exit 1; }
previousControl="$previousMount/$dataInsideImage/global/pg_control"
currentControl="$currentMount/$dataInsideImage/global/pg_control"
[ -f "$previousControl" ] || { echo "previous image carries no cluster"; umount "$currentMount"; umount "$previousMount"; exit 1; }
echo "== control file ages =="
echo "previous: $(date -u -r "$previousControl" +%FT%TZ)"
echo "current:  $(date -u -r "$currentControl" +%FT%TZ 2>/dev/null || echo missing)"
if [ -f "$currentControl" ] && [ ! "$previousControl" -nt "$currentControl" ]; then
  echo "previous cluster is not newer; refusing to overwrite a newer cluster with an older one"
  umount "$currentMount"; umount "$previousMount"; exit 1
fi
echo "== keep the current cluster on the host so this is reversible =="
cp -a "$currentMount/$dataInsideImage" "$rollback" 2>&1 && echo "rollback copy: $rollback" || { echo "rollback copy FAILED"; umount "$currentMount"; umount "$previousMount"; exit 1; }
du -sh "$rollback" 2>&1
echo "== install the previous cluster =="
rm -rf "$currentMount/$dataInsideImage" && cp -a "$previousMount/$dataInsideImage" "$currentMount/$dataInsideImage" && echo "installed" || echo "install FAILED"
sync
ls -la "$currentMount/$dataInsideImage/global/pg_control" 2>&1
umount "$currentMount"; umount "$previousMount"; rmdir "$currentMount" "$previousMount"
echo "== blueclaw stays stopped; start it with restart-blueclaw =="
systemctl is-active blueclaw 2>&1
`)
}

func blueclawPostgresInspectCommand() string {
	return strings.TrimSpace(`
set +e
image=/var/lib/blueclaw/workspace.ext4
preserved=/var/lib/blueclaw/workspace-preserved-$(date -u +%Y%m%dT%H%M%SZ).ext4
echo "== disk =="; df -h /var/lib/blueclaw 2>&1 | tail -1
echo "== stop blueclaw (ends the panic loop) =="; systemctl stop blueclaw 2>&1; sleep 3
echo "== preserve the image =="
existing=$(ls -t /var/lib/blueclaw/workspace-preserved-*.ext4 2>/dev/null | head -1)
if [ -n "$existing" ]; then
  echo "already preserved: $existing"
else
  cp --reflink=auto -a "$image" "$preserved" && echo "preserved: $preserved" || echo "preserve FAILED"
fi
ls -la /var/lib/blueclaw/workspace-preserved-*.ext4 2>&1 | tail -2
echo "== drop plain backup intermediates that never got encrypted =="
for plain in /root/.internkim/backups/.internkim-backup-*.tar.gz; do
  [ -e "$plain" ] || continue
  stamp=$(basename "$plain" .tar.gz); stamp=${stamp#.internkim-backup-}
  if [ -e "/root/.internkim/backups/internkim-backup-$stamp.ikbak" ]; then echo "keeping $plain (encrypted copy exists)"; else rm -f -v "$plain"; fi
done
echo "== daily backups =="; ls -la /root/.internkim/backups 2>&1 | tail -12
echo "== read the guest cluster read-only =="
mountPoint=$(mktemp -d)
mount -o ro,noload,loop "$image" "$mountPoint" 2>&1 || { echo "read-only mount FAILED"; rmdir "$mountPoint"; exit 1; }
dataPath="$mountPoint/.blueclaw/postgres/data"
echo "== candidate images =="; ls -la --time-style=+%FT%TZ /var/lib/blueclaw/*.ext4 /var/lib/blueclaw/*.previous 2>/dev/null
controlData=$(find /usr/lib/postgresql -path "*/bin/pg_controldata" -type f 2>/dev/null | sort -V | tail -1)
if [ -n "$controlData" ]; then "$controlData" -D "$dataPath" 2>&1 | grep -iE "state|checkpoint location|redo location|time line|latest checkpoint" | head -12; else echo "pg_controldata not on host"; fi
echo "== cluster size and wal =="; du -sh "$dataPath" 2>/dev/null; ls -la "$dataPath/pg_wal" 2>/dev/null | tail -6
echo "== lost+found =="; ls -la "$mountPoint/lost+found" 2>/dev/null | head -5
umount "$mountPoint"
for candidate in /var/lib/blueclaw/workspace.ext4.previous /var/lib/blueclaw/workspace-preserved-*.ext4; do
  [ -e "$candidate" ] || continue
  echo "== $candidate =="
  mount -o ro,noload,loop "$candidate" "$mountPoint" 2>&1 || { echo "  mount failed"; continue; }
  ls -la "$mountPoint/.blueclaw/postgres/data/pg_wal" 2>/dev/null | tail -4
  ls -la "$mountPoint/.blueclaw/postgres/data/global/pg_control" 2>/dev/null
  umount "$mountPoint"
done
rmdir "$mountPoint"
echo "== blueclaw left stopped on purpose =="; systemctl is-active blueclaw 2>&1
`)
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
CHATD_LISTEN_HOSTNAME_VALUE="` + blueclaw.ChatdListenHostname + `"
mkdir -p /etc/systemd/system/chatd.service.d
cat > /etc/systemd/system/chatd.service.d/mirror.conf <<EOF
[Service]
Environment=CHATD_MATTERMOST_BASE_URL=$MM
Environment=CHATD_MATTERMOST_BOT_TOKEN=$BOT
Environment=CHATD_MATTERMOST_ADMIN_TOKEN=$ADMIN_PAT
Environment=CHATD_BLUECLAW_INGRESS_URL=http://127.0.0.1:8080/connectors/mattermost/events
Environment=CHATD_LISTEN_HOSTNAME=${CHATD_LISTEN_HOSTNAME_VALUE}
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

func mattermostMirrorRetireCommand() string {
	return strings.TrimSpace(`
set +e
DROPIN=/etc/systemd/system/chatd.service.d/mirror.conf
KEPT=/etc/systemd/system/chatd.service.d/mirror.conf.retired
test -f "$DROPIN" && cp "$DROPIN" "$KEPT"
mkdir -p /etc/systemd/system/chatd.service.d
cat > "$DROPIN" <<EOF
[Service]
Environment=CHATD_LISTEN_HOSTNAME=` + blueclaw.ChatdListenHostname + `
EOF
systemctl daemon-reload
systemctl restart chatd
sleep 5
if [ "$(systemctl is-active chatd)" != active ]; then
  test -f "$KEPT" && cp "$KEPT" "$DROPIN"
  systemctl daemon-reload
  systemctl restart chatd
  echo "ROLLED BACK: chatd failed without the mattermost mirror; the mirror is back"
  exit 1
fi
echo "mattermost mirror retired; chatd active"
journalctl -u chatd -n 30 --no-pager 2>&1 | grep -iE "adapters|mirror|mattermost|listen|ready|error" | tail -10
`)
}

func mattermostStopCommand() string {
	return strings.TrimSpace(`
set +e
systemctl disable --now mattermost
sleep 3
echo "== mattermost =="
systemctl show mattermost -p ActiveState,UnitFileState 2>&1
echo "== does anything still answer on 8065? =="
curl -fsS --max-time 3 http://127.0.0.1:8065/api/v4/system/ping >/dev/null 2>&1 && echo "8065 still answers" || echo "8065 is silent"
echo "== chatd =="
systemctl show chatd -p ActiveState,NRestarts 2>&1
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
export DB_URL=$(grep '^DATABASE_URL=' ` + blueclaw.BuzzRelayDatabaseEnvironmentFilePath + ` | head -1 | sed 's/^DATABASE_URL=//')
export TEAM=$(curl -fsS -H "Authorization: Bearer $MM_TOKEN" ` + blueclaw.BlueclawMattermostLocalURL + `/api/v4/teams | jq -r '.[0].name')
export PUBLIC_HOST=$(systemctl show ` + relay + ` -p Environment | tr ' ' '\n' | sed -n 's/^RELAY_URL=//p' | head -1 | sed -E 's#^[a-z]+://##; s#/.*$##')
if [ -z "$PUBLIC_HOST" ]; then echo "the relay names no public host, and an import keyed to a guess lands in a community nothing serves"; exit 1; fi
export BUZZ_RELAY_PRIVATE_KEY=$(grep '^BUZZ_RELAY_PRIVATE_KEY=' /root/.internkim/secrets/buzz-relay-env | head -1 | sed 's/^BUZZ_RELAY_PRIVATE_KEY=//')
rm -f ` + marker + `
cat > /tmp/buzz-reimport-run.sh <<'RUNEOF'
export DATABASE_URL="$DB_URL"
` + centralPlaneFlagsForBuzzMigrate() + `
` + blueclaw.BuzzMigrateBinaryPath + ` $CENTRAL \
  --mattermost-url ` + blueclaw.BlueclawMattermostLocalURL + ` \
  --mattermost-token-path ` + blueclaw.BlueclawMattermostTokenPath + ` \
  --team "$TEAM" \
  --buzz-database-url "$DB_URL" \
  --buzz-admin ` + blueclaw.BuzzAdminBinaryPath + ` \
  --key-seed-path /root/.internkim/secrets/buzz-key-seed \
  --bridge-url ` + blueclaw.AdmindBaseURL + `/bridge/api \
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
  printf "== member1@example.com account state ==\n"
  su - postgres -c "psql -X -qAt -d '$escaped_database' -c \"SELECT email || ' failed=' || failedattempts || ' del=' || deleteat || ' auth=' || COALESCE(NULLIF(authservice,''),'password') FROM users WHERE lower(email)='member1@example.com'\"" 2>/dev/null
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

func admindJournalCommand() string {
	return strings.TrimSpace(`
set +e
journalctl -u internkim-admind -n 200 --no-pager 2>&1 | grep -iE 'roster|circle|users lookup|directory|policy|error|failed' | tail -40
`)
}

func buzzRoomChangePreamble() string {
	return `set +e
q() { su - postgres -c "psql -X -d buzz -c \"$1\"" 2>&1; }
`
}

// A client lists rooms from their kind 39000 discovery events, not from the
// channels table, and reconcile-channels writes an event only where none
// exists. A row changed without dropping its event leaves every client showing
// the room as it was, through a reload and through a restart.
func buzzRoomChangeRepublish() string {
	return `
export BUZZ_RELAY_PRIVATE_KEY=$(grep '^BUZZ_RELAY_PRIVATE_KEY=' ` + blueclaw.BuzzRelayKeyEnvironmentFilePath + ` | head -1 | sed 's/^BUZZ_RELAY_PRIVATE_KEY=//')
export DATABASE_URL=$(grep '^DATABASE_URL=' ` + blueclaw.BuzzRelayDatabaseEnvironmentFilePath + ` | head -1 | sed 's/^DATABASE_URL=//')
export RELAY_URL=$(systemctl show ` + blueclaw.BuzzRelayServiceName + ` -p Environment | tr ' ' '\n' | sed -n 's/^RELAY_URL=//p' | head -1)
if [ -z "$BUZZ_RELAY_PRIVATE_KEY" ] || [ -z "$RELAY_URL" ]; then
  echo "the rows changed but no client was told: this device names no relay key or public host"
  exit 1
fi
printf '== told the clients ==\n'
` + blueclaw.BuzzAdminBinaryPath + ` reconcile-channels
`
}

func sqlQuotedRoomName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", errors.New("this action names the room")
	}
	if strings.ContainsAny(trimmed, "\"\\$`\n") {
		return "", fmt.Errorf("a room name carrying a quote, a backslash, a dollar or a backtick is refused rather than mangled: %q", trimmed)
	}
	return "'" + strings.ReplaceAll(trimmed, "'", "''") + "'", nil
}

func buzzRoomVisibilityCommand() string {
	return strings.TrimSpace(`
set +e
q() { su - postgres -c "psql -X -d buzz -c \"$1\"" 2>&1; }
printf '== every room, how open it is, and how many are in it ==\n'
q "SELECT c.name AS room, c.visibility, count(m.*) FILTER (WHERE m.removed_at IS NULL) AS members, c.deleted_at IS NOT NULL AS retired FROM channels c LEFT JOIN channel_members m ON m.channel_id = c.id WHERE c.channel_type = 'stream' GROUP BY c.id, c.name, c.visibility, c.deleted_at ORDER BY 4, 2, 1"
`)
}

func buzzCloseRoomsExceptCommand(openRooms []string) (string, error) {
	quoted := make([]string, 0, len(openRooms))
	for _, room := range openRooms {
		literal, errorValue := sqlQuotedRoomName(room)
		if errorValue != nil {
			return "", errorValue
		}
		quoted = append(quoted, literal)
	}
	if len(quoted) == 0 {
		return "", errors.New("this action names the rooms that stay open")
	}
	stayOpen := strings.Join(quoted, ", ")
	return strings.TrimSpace(buzzRoomChangePreamble() + `printf '== rooms that stay open ==\n'
q "SELECT name FROM channels WHERE channel_type = 'stream' AND deleted_at IS NULL AND name IN (` + stayOpen + `) ORDER BY 1"
printf '== rooms closed by this action ==\n'
q "WITH changed AS (UPDATE channels SET visibility = 'private' WHERE channel_type = 'stream' AND deleted_at IS NULL AND visibility <> 'private' AND name NOT IN (` + stayOpen + `) RETURNING id, name), forgotten AS (DELETE FROM events WHERE kind IN (39000,39001,39002) AND channel_id IN (SELECT id FROM changed)) SELECT name FROM changed ORDER BY 1"` + buzzRoomChangeRepublish()), nil
}

func buzzRenameRoomCommand(target string) (string, error) {
	oldName, newName, isPair := strings.Cut(target, "::")
	if !isPair {
		return "", errors.New("this action names the room as old::new")
	}
	quotedOldName, errorValue := sqlQuotedRoomName(oldName)
	if errorValue != nil {
		return "", errorValue
	}
	quotedNewName, errorValue := sqlQuotedRoomName(newName)
	if errorValue != nil {
		return "", errorValue
	}
	return strings.TrimSpace(buzzRoomChangePreamble() + `printf '== renamed ==\n'
q "WITH changed AS (UPDATE channels SET name = ` + quotedNewName + ` WHERE channel_type = 'stream' AND deleted_at IS NULL AND name = ` + quotedOldName + ` RETURNING id, name), forgotten AS (DELETE FROM events WHERE kind IN (39000,39001,39002) AND channel_id IN (SELECT id FROM changed)) SELECT id::text, name FROM changed"` + buzzRoomChangeRepublish()), nil
}

func buzzRetireRoomByNameCommand(name string) (string, error) {
	quotedName, errorValue := sqlQuotedRoomName(name)
	if errorValue != nil {
		return "", errorValue
	}
	return strings.TrimSpace(buzzRoomChangePreamble() + `printf '== retired ==\n'
q "WITH changed AS (UPDATE channels SET deleted_at = now() WHERE channel_type = 'stream' AND deleted_at IS NULL AND name = ` + quotedName + ` RETURNING id, name), forgotten AS (DELETE FROM events WHERE kind IN (39000,39001,39002) AND channel_id IN (SELECT id FROM changed)) SELECT id::text, name FROM changed"` + buzzRoomChangeRepublish()), nil
}

func buzzRoomRosterCommand() string {
	return strings.TrimSpace(`
set +e
q() { su - postgres -c "psql -X -d buzz -c \"$1\"" 2>&1; }
printf '== who is in each room ==\n'
q "SELECT c.name AS room, coalesce(n.content::json->>'display_name', n.content::json->>'name', left(encode(m.pubkey, 'hex'), 8)) AS person FROM channels c JOIN channel_members m ON m.channel_id = c.id AND m.removed_at IS NULL LEFT JOIN (SELECT DISTINCT ON (pubkey) pubkey, content FROM events WHERE kind = 0 AND content LIKE '{%' ORDER BY pubkey, created_at DESC) n ON n.pubkey = m.pubkey WHERE c.channel_type = 'stream' AND c.deleted_at IS NULL ORDER BY 1, 2"
`)
}

func policyCircleRosterCommand() string {
	return strings.TrimSpace(`
set +e
policy=$(curl -fsS ` + blueclaw.BlueclawBaseURL + `/admin/api/policy 2>/dev/null)
[ -n "$policy" ] || { echo "blueclaw did not answer with a policy"; exit 1; }
printf '== who the policy names ==\n'
printf '%s' "$policy" | jq -r '.people[]? | "\(.personID // "?")  \(.displayName // "(unnamed)")  [\(.emails // [] | join(", "))]  circles: \(.circles // [] | join(", "))"'
printf '== the circles it declares ==\n'
printf '%s' "$policy" | jq -r '.circles[]? | "\(.circleID // "?")  \(.displayName // "")"'
`)
}

func buzzReadTestCommand() string {
	return strings.TrimSpace(`
set +e
EMAIL=$(jq -r '[.people[]?.emails[]?] | map(select(. != null and . != "")) | .[0] // empty' ` + blueclaw.BlueclawPolicyConfigPath + ` 2>/dev/null)
[ -n "$EMAIL" ] || { echo "the policy names nobody to read as"; exit 1; }
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

// Names and faces are published at the end of an import whatever it carried, so
// an import that reaches no post publishes them and nothing else. A since in the
// future skips the wipe and every message, which is the difference between the
// channels carrying their company's name today and waiting for whoever runs the
// next re-import.
// buzz-migrate reads the company's picture out of its closed bucket and puts it
// where the messenger's own store serves it. The four settings that reach the
// central plane go together or not at all: keeperFrom dies on a partial set, and
// a device whose company has not moved yet has none of them.
// A profile whose picture is a path into the closed asset bucket, or an address
// only a company session opens, renders as nothing in every Buzz app and looks
// exactly like a profile that was never republished. This tells the two apart.
// A probe's profile is the only thing left of an account that no longer exists,
// and nothing will ever replace it: kind 0 is replaceable per key, so a key
// nobody writes as keeps its last profile forever. Counting first, deleting
// second, because this is the company's own relay.
func buzzProbeProfilePurgeCommand(apply bool) string {
	probes := `WITH newest AS (SELECT DISTINCT ON (pubkey) pubkey, content FROM events WHERE kind=0 AND content LIKE '{%' ORDER BY pubkey, created_at DESC), probes AS (SELECT pubkey FROM newest WHERE content::json->>'display_name' LIKE 'probemm%')`
	action := `SELECT count(*) FROM events WHERE kind=0 AND pubkey IN (SELECT pubkey FROM probes)`
	outcome := "that would go"
	if apply {
		action = `, gone AS (DELETE FROM events WHERE kind=0 AND pubkey IN (SELECT pubkey FROM probes) RETURNING 1) SELECT count(*) FROM gone`
		outcome = "deleted"
	}
	return strings.TrimSpace(`
set +e
q() { su - postgres -c "psql -X -qAt -d ` + blueclaw.BuzzRelayDatabaseName + ` -c \"$1\"" 2>&1; }
q "` + probes + ` SELECT count(*) FROM probes" | sed 's/^/probe identities: /'
q "` + probes + ` SELECT count(*) FROM events WHERE kind<>0 AND pubkey IN (SELECT pubkey FROM probes)" | sed 's/^/everything else they wrote, left alone: /'
q "` + probes + ` ` + action + `" | sed 's/^/profile events ` + outcome + `: /'
`)
}

// A reaction whose content is a bare word is one the mirror published before it
// learned to convert - the platform's name for the emoji instead of the emoji.
// Already signed and published, so nothing rewrites it in place.
func buzzStrangerMemberCommand(apply bool) string {
	applyValue := "false"
	if apply {
		applyValue = "true"
	}
	return strings.TrimSpace(`
body=$(curl -sS -X POST "` + blueclaw.AdmindBaseURL + `/agent/api/buzz-stranger-members?apply=` + applyValue + `")
printf '%s\n' "$body" | jq . 2>/dev/null || printf '%s\n' "$body"
`)
}

// Every link the agent sent before it learned the company's address names a
// device host, and the identifier beside it is one only this device knows. A
// rewrite has to resolve each one, so the count comes first.
func organizationDirectoryCoverageCommand(seed bool) string {
	seedValue := "false"
	if seed {
		seedValue = "true"
	}
	return strings.TrimSpace(`
body=$(curl -sS "` + blueclaw.AdmindBaseURL + `/agent/api/organization-directory-coverage?seed=` + seedValue + `")
printf '%s\n' "$body" | jq . 2>/dev/null || printf '%s\n' "$body"
`)
}

func calendarRecordCoverageCommand(pair bool) string {
	pairValue := "false"
	if pair {
		pairValue = "true"
	}
	return strings.TrimSpace(`
body=$(curl -sS "` + blueclaw.AdmindBaseURL + `/agent/api/calendar-record-coverage?pair=` + pairValue + `")
printf '%s\n' "$body" | jq . 2>/dev/null || printf '%s\n' "$body"
`)
}

func buzzRewriteOldLinksCommand(apply bool) string {
	applyValue := "false"
	if apply {
		applyValue = "true"
	}
	return strings.TrimSpace(`
body=$(curl -sS -X POST "` + blueclaw.AdmindBaseURL + `/agent/api/buzz-rewrite-old-links?apply=` + applyValue + `")
printf '%s\n' "$body" | jq . 2>/dev/null || printf '%s\n' "$body"
`)
}

func buzzDeviceLinkCountCommand() string {
	return strings.TrimSpace(`
set +e
q() { su - postgres -c "psql -X -qAt -d ` + blueclaw.BuzzRelayDatabaseName + ` -c \"$1\"" 2>&1; }
host=$(systemctl show ` + blueclaw.BuzzRelayServiceName + ` -p Environment | tr ' ' '\n' | sed -n 's/^RELAY_URL=//p' | head -1 | sed -E 's#^[a-z]+://##; s#/.*$##')
printf 'relay host: %s\n' "$host"
q "SELECT count(*) FROM events WHERE kind=9 AND content LIKE '%.intern.kim/calendar%'" | sed 's/^/messages linking a calendar: /'
q "SELECT count(*) FROM events WHERE kind=9 AND content LIKE '%.intern.kim/flow%'" | sed 's/^/messages linking the board: /'
q "SELECT count(*) FROM events WHERE kind=9 AND content LIKE '%zd2df6qt6jmc%'" | sed 's/^/messages naming this device: /'
`)
}

func buzzNamedReactionCountCommand() string {
	return strings.TrimSpace(`
set +e
q() { su - postgres -c "psql -X -qAt -d ` + blueclaw.BuzzRelayDatabaseName + ` -c \"$1\"" 2>&1; }
named="content ~ '^[a-z0-9_+-]{2,}$'"
q "SELECT count(*) FROM events WHERE kind=7" | sed 's/^/reactions: /'
q "SELECT count(*) FROM events WHERE kind=7 AND $named" | sed 's/^/carrying a name instead of a character: /'
printf '== which names ==\n'
q "SELECT content, count(*) FROM events WHERE kind=7 AND $named GROUP BY 1 ORDER BY 2 DESC LIMIT 20"
`)
}

func buzzProfileInspectCommand() string {
	return strings.TrimSpace(`
set +e
q() { su - postgres -c "psql -X -qAt -d ` + blueclaw.BuzzRelayDatabaseName + ` -c \"$1\"" 2>&1; }
printf '== profiles ==\n'
q "SELECT count(DISTINCT pubkey) FROM events WHERE kind=0" | sed 's/^/people with a profile: /'
q "SELECT count(*) FROM events WHERE kind=0" | sed 's/^/profile events, all versions: /'
q "SELECT count(*) FROM (SELECT DISTINCT ON (pubkey) content FROM events WHERE kind=0 ORDER BY pubkey, created_at DESC) newest WHERE content LIKE '%storage/v1/object%'" | sed 's/^/pointing at the closed bucket: /'
q "SELECT count(*) FROM (SELECT DISTINCT ON (pubkey) content FROM events WHERE kind=0 ORDER BY pubkey, created_at DESC) newest WHERE content LIKE '%\"picture\"%'" | sed 's/^/carrying a picture: /'
printf '== who they are ==\n'
q "SELECT left(encode(pubkey, 'hex'), 8) || '  ' || coalesce(content::json->>'display_name', content::json->>'name', '(unnamed)') FROM (SELECT DISTINCT ON (pubkey) pubkey, content FROM events WHERE kind=0 ORDER BY pubkey, created_at DESC) newest WHERE content LIKE '{%' ORDER BY 1"
`)
}

func centralPlaneFlagsForBuzzMigrate() string {
	return `
CENTRAL=""
if [ -r ` + blueclaw.RelayEnvironmentFilePath + ` ] && [ -r ` + blueclaw.RelayAgentKeyPath + ` ]; then
  APP_URL=$(grep '^INTERNKIM_APP_URL=' ` + blueclaw.RelayEnvironmentFilePath + ` | head -1 | sed 's/^INTERNKIM_APP_URL=//')
  SUPABASE_URL=$(grep '^SUPABASE_URL=' ` + blueclaw.RelayEnvironmentFilePath + ` | head -1 | sed 's/^SUPABASE_URL=//')
  SUPABASE_KEY=$(grep '^SUPABASE_PUBLISHABLE_KEY=' ` + blueclaw.RelayEnvironmentFilePath + ` | head -1 | sed 's/^SUPABASE_PUBLISHABLE_KEY=//')
  if [ -n "$APP_URL" ] && [ -n "$SUPABASE_URL" ] && [ -n "$SUPABASE_KEY" ]; then
    CENTRAL="--app-url $APP_URL --agent-key-path ` + blueclaw.RelayAgentKeyPath + ` --supabase-url $SUPABASE_URL --supabase-publishable-key $SUPABASE_KEY"
  fi
fi
if [ -z "$CENTRAL" ]; then echo "no central plane on this device: the company keeps whatever picture it already had"; fi
`
}

func buzzRefreshProfilesCommand() string {
	return strings.TrimSpace(`
set -e
MM_TOKEN_PATH=` + blueclaw.BlueclawMattermostTokenPath + `
DB_URL=$(grep '^DATABASE_URL=' ` + blueclaw.BuzzRelayDatabaseEnvironmentFilePath + ` | head -1 | sed 's/^DATABASE_URL=//')
TEAM=$(curl -fsS -H "Authorization: Bearer $(cat $MM_TOKEN_PATH)" ` + blueclaw.BlueclawMattermostLocalURL + `/api/v4/teams | jq -r '.[0].name')
PUBLIC_HOST=$(systemctl show ` + blueclaw.BuzzRelayServiceName + ` -p Environment | tr ' ' '\n' | sed -n 's/^RELAY_URL=//p' | head -1 | sed -E 's#^[a-z]+://##; s#/.*$##')
if [ -z "$PUBLIC_HOST" ]; then echo "the relay names no public host, and a profile keyed to a guess lands in a community nothing serves"; exit 1; fi
export DATABASE_URL="$DB_URL"
` + centralPlaneFlagsForBuzzMigrate() + `
` + blueclaw.BuzzMigrateBinaryPath + ` $CENTRAL \
  --mattermost-url ` + blueclaw.BlueclawMattermostLocalURL + ` \
  --mattermost-token-path $MM_TOKEN_PATH \
  --team "$TEAM" \
  --buzz-database-url "$DB_URL" \
  --buzz-admin ` + blueclaw.BuzzAdminBinaryPath + ` \
  --key-seed-path /root/.internkim/secrets/buzz-key-seed \
  --bridge-url ` + blueclaw.AdmindBaseURL + `/bridge/api \
  --relay-url wss://$PUBLIC_HOST \
  --relay-http-url https://$PUBLIC_HOST \
  --community-host "$PUBLIC_HOST" \
  --since $(( $(date -u +%s) * 1000 + 60000 )) 2>&1 | tail -20
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

// cmd/buzz-migrate created every conversation it imported with visibility open,
// so the community can read them (#705). channel_members soft-deletes with
// removed_at, and the people who have left are exactly who a repair cannot put
// back, so they are counted apart.
func buzzWhoseKeyCommand(pubkey string) string {
	return strings.TrimSpace(`
body=$(curl -sS "` + blueclaw.AdmindBaseURL + `/agent/api/buzz-whose-key?pubkey=` + url.QueryEscape(strings.TrimSpace(pubkey)) + `")
printf '%s\n' "$body" | jq . 2>/dev/null || printf '%s\n' "$body"
`)
}

func circleRoomMembershipCommand(shouldApply bool) string {
	applyValue := "false"
	if shouldApply {
		applyValue = "true"
	}
	return strings.TrimSpace(`
body=$(curl -sS "` + blueclaw.AdmindBaseURL + `/agent/api/circle-room-membership?apply=` + applyValue + `")
printf '%s\n' "$body" | jq . 2>/dev/null || printf '%s\n' "$body"
`)
}

func circleMembershipReconcileCommand(email string, shouldApply bool) string {
	applyValue := "false"
	if shouldApply {
		applyValue = "true"
	}
	return strings.TrimSpace(`
body=$(curl -sS "` + blueclaw.AdmindBaseURL + `/agent/api/circle-membership-reconcile?apply=` + applyValue + `&email=` + url.QueryEscape(strings.TrimSpace(email)) + `")
printf '%s\n' "$body" | jq . 2>/dev/null || printf '%s\n' "$body"
`)
}

func buzzRetireNamedRoomCommand(room string) string {
	return strings.TrimSpace(`
body=$(curl -sS "` + blueclaw.AdmindBaseURL + `/agent/api/buzz-channel-retire?retire=true&room=` + url.QueryEscape(strings.TrimSpace(room)) + `")
printf '%s\n' "$body" | jq . 2>/dev/null || printf '%s\n' "$body"
`)
}

func buzzChannelRetireCommand(shouldRetire bool) string {
	retireValue := "false"
	if shouldRetire {
		retireValue = "true"
	}
	return strings.TrimSpace(`
body=$(curl -sS "` + blueclaw.AdmindBaseURL + `/agent/api/buzz-channel-retire?retire=` + retireValue + `")
printf '%s\n' "$body" | jq . 2>/dev/null || printf '%s\n' "$body"
`)
}

func buzzChannelMembershipRepairCommand(shouldRemove bool) string {
	removeValue := "false"
	if shouldRemove {
		removeValue = "true"
	}
	return strings.TrimSpace(`
body=$(curl -sS "` + blueclaw.AdmindBaseURL + `/agent/api/buzz-channel-membership-repair?remove=` + removeValue + `")
printf '%s\n' "$body" | jq . 2>/dev/null || printf '%s\n' "$body"
`)
}

func buzzChannelVisibilityRepairCommand(shouldClose bool) string {
	closeValue := "false"
	if shouldClose {
		closeValue = "true"
	}
	return strings.TrimSpace(`
body=$(curl -sS "` + blueclaw.AdmindBaseURL + `/agent/api/buzz-channel-visibility-repair?close=` + closeValue + `")
printf '%s\n' "$body" | jq . 2>/dev/null || printf '%s\n' "$body"
`)
}

func buzzChannelVisibilityCommand() string {
	return strings.TrimSpace(`
set +e
q() { su - postgres -c "psql -X -d buzz -c \"$1\"" 2>&1; }
printf '== channels by type and visibility ==\n'
q "SELECT channel_type, visibility, count(*) FROM channels GROUP BY 1,2 ORDER BY 3 DESC"
printf '== closed, and what closed them ==\n'
q "SELECT name, deleted_at IS NOT NULL AS gone_from_listings, archived_at IS NOT NULL AS archived FROM channels WHERE deleted_at IS NOT NULL OR archived_at IS NOT NULL ORDER BY name"
printf '== how many a client still lists ==\n'
q "SELECT count(*) FROM channels WHERE deleted_at IS NULL"
printf '== every open channel that is not a stream ==\n'
q "SELECT id::text, channel_type, name FROM channels WHERE visibility='open' AND channel_type <> 'stream' ORDER BY name"
printf '== members per open non-stream channel ==\n'
q "SELECT c.id::text, c.name, count(m.*) FILTER (WHERE m.removed_at IS NULL) AS members, count(m.*) FILTER (WHERE m.removed_at IS NOT NULL) AS left_since FROM channels c LEFT JOIN channel_members m ON m.channel_id = c.id WHERE c.visibility='open' AND c.channel_type <> 'stream' GROUP BY 1,2 ORDER BY 3 DESC"
printf '== channels per community ==\n'
q "SELECT coalesce(community_id::text, '(none)') AS community, channel_type, count(*) FROM channels GROUP BY 1,2 ORDER BY 1,2"
printf '== the communities a relay host maps to ==\n'
q "SELECT c.id::text, c.host, count(ch.*) AS channels FROM communities c LEFT JOIN channels ch ON ch.community_id = c.id GROUP BY 1,2 ORDER BY 2"
printf '== messages per community ==\n'
q "SELECT community_id::text, count(*) AS messages FROM events WHERE kind = 9 GROUP BY 1 ORDER BY 2 DESC"
`)
}

// A channel written straight into the database has no kind:39000 or kind:39002
// event, so a nostr client has nothing to discover it by. chatd finds such a
// channel anyway because it queries membership directly and tolerates missing
// metadata, so only a nostr client notices.
// buzz-admin's reconcile-channels writes a channel's discovery event from its
// row alone. The relay's own emitter adds a p tag per participant for a dm,
// "so clients can resolve display names without a separate kind:39002 fetch",
// and reconcile-channels does not. A dm whose event it wrote therefore shows
// in every client as DM with no name and no picture.
func buzzRestoreDirectMessageDiscoveryCommand(snapshotName string) (string, error) {
	name := strings.TrimSpace(snapshotName)
	if name == "" {
		return "", errors.New("this action names the snapshot to read the events back from")
	}
	if strings.ContainsAny(name, "/\\'\"$`\n ") {
		return "", fmt.Errorf("a snapshot is named by its file name alone, got %q", name)
	}
	return strings.TrimSpace(`
set -e
SNAPSHOT=/root/.internkim/backups/` + name + `
test -r "$SNAPSHOT" || { echo "no snapshot at $SNAPSHOT"; exit 1; }
CARRIED=/tmp/buzz-dm-discovery.csv
rm -f "$CARRIED"
q() { su - postgres -c "psql -X -qAt -d buzz -c \"$1\"" 2>&1; }
printf '== direct message discovery events now ==\n'
q "SELECT count(*) FROM events WHERE kind IN (39000,39001,39002) AND channel_id IN (SELECT id FROM channels WHERE channel_type = 'dm')"
su - postgres -c "dropdb --if-exists buzz_discovery_restore"
su - postgres -c "createdb buzz_discovery_restore"
su - postgres -c "psql -X -q -d buzz_discovery_restore -f $SNAPSHOT" >/dev/null 2>&1
su - postgres -c "psql -X -qAt -d buzz_discovery_restore -c \"COPY (SELECT e.* FROM events e JOIN channels c ON c.id = e.channel_id WHERE e.kind IN (39000,39001,39002) AND c.channel_type = 'dm') TO '$CARRIED' CSV\""
CARRIED_ROWS=$(wc -l < "$CARRIED" | tr -d ' ')
printf 'events the snapshot carries: %s\n' "$CARRIED_ROWS"
if [ "$CARRIED_ROWS" -lt 1 ]; then
  su - postgres -c "dropdb --if-exists buzz_discovery_restore"
  echo "the snapshot carries no direct message discovery; nothing replaced"
  exit 1
fi
q "DELETE FROM events WHERE kind IN (39000,39001,39002) AND channel_id IN (SELECT id FROM channels WHERE channel_type = 'dm')" | sed 's/^/dropped: /'
su - postgres -c "psql -X -qAt -d buzz -c \"COPY events FROM '$CARRIED' CSV\"" | sed 's/^/restored: /'
su - postgres -c "dropdb --if-exists buzz_discovery_restore"
rm -f "$CARRIED"
printf '== direct message discovery events after ==\n'
q "SELECT count(*) FROM events WHERE kind IN (39000,39001,39002) AND channel_id IN (SELECT id FROM channels WHERE channel_type = 'dm')"
printf '== how many carry a participant tag ==\n'
q "SELECT count(*) FROM events WHERE kind = 39000 AND tags::text LIKE '%\"p\"%' AND channel_id IN (SELECT id FROM channels WHERE channel_type = 'dm')"
`), nil
}

func buzzRepublishRoomsCommand() string {
	return strings.TrimSpace(`
set -e
export BUZZ_RELAY_PRIVATE_KEY=$(grep '^BUZZ_RELAY_PRIVATE_KEY=' ` + blueclaw.BuzzRelayKeyEnvironmentFilePath + ` | head -1 | sed 's/^BUZZ_RELAY_PRIVATE_KEY=//')
if [ -z "$BUZZ_RELAY_PRIVATE_KEY" ]; then echo "no relay signing key on this device; a republish would sign with a key that dies at restart"; exit 1; fi
export DATABASE_URL=$(grep '^DATABASE_URL=' ` + blueclaw.BuzzRelayDatabaseEnvironmentFilePath + ` | head -1 | sed 's/^DATABASE_URL=//')
export RELAY_URL=$(systemctl show ` + blueclaw.BuzzRelayServiceName + ` -p Environment | tr ' ' '\n' | sed -n 's/^RELAY_URL=//p' | head -1)
if [ -z "$RELAY_URL" ]; then echo "the relay names no public host, and buzz-admin works on the community that host names"; exit 1; fi
q() { su - postgres -c "psql -X -qAt -d ` + blueclaw.BuzzRelayDatabaseName + ` -c \"$1\"" 2>&1; }
printf '== discovery events before ==\n'
q "SELECT kind, count(*) FROM events WHERE kind IN (39000,39001,39002) GROUP BY kind ORDER BY kind"
q "DELETE FROM events WHERE kind IN (39000,39001,39002)" | sed 's/^/deleted: /'
` + blueclaw.BuzzAdminBinaryPath + ` reconcile-channels
printf '== discovery events after ==\n'
q "SELECT kind, count(*) FROM events WHERE kind IN (39000,39001,39002) GROUP BY kind ORDER BY kind"
printf '== what a client now lists ==\n'
q "SELECT c.name FROM channels c WHERE c.channel_type = 'stream' AND c.deleted_at IS NULL AND EXISTS (SELECT 1 FROM events e WHERE e.kind = 39000 AND e.channel_id = c.id) ORDER BY 1"
`)
}

func buzzReconcileChannelsCommand() string {
	return strings.TrimSpace(`
set -e
export BUZZ_RELAY_PRIVATE_KEY=$(grep '^BUZZ_RELAY_PRIVATE_KEY=' ` + blueclaw.BuzzRelayKeyEnvironmentFilePath + ` | head -1 | sed 's/^BUZZ_RELAY_PRIVATE_KEY=//')
if [ -z "$BUZZ_RELAY_PRIVATE_KEY" ]; then echo "no relay signing key on this device; reconcile would sign with a key that dies at restart"; exit 1; fi
export DATABASE_URL=$(grep '^DATABASE_URL=' ` + blueclaw.BuzzRelayDatabaseEnvironmentFilePath + ` | head -1 | sed 's/^DATABASE_URL=//')
export RELAY_URL=$(systemctl show ` + blueclaw.BuzzRelayServiceName + ` -p Environment | tr ' ' '\n' | sed -n 's/^RELAY_URL=//p' | head -1)
if [ -z "$RELAY_URL" ]; then echo "the relay names no public host, and buzz-admin works on the community that host names"; exit 1; fi
` + blueclaw.BuzzAdminBinaryPath + ` reconcile-channels
echo "== channels carrying discovery metadata =="
su - postgres -c "psql -X -qAt -d ` + blueclaw.BuzzRelayDatabaseName + ` -c \"SELECT kind, count(*) FROM events WHERE kind IN (39000,39002) GROUP BY kind ORDER BY kind\""
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
		"mm-pat-enabled":       service.sshRecoveryCommandOutput(ctx, "sh", "-lc", "grep -o '\\\"EnableUserAccessTokens\\\": *[a-z]*' /opt/mattermost/config/config.json 2>/dev/null | grep -oE 'true|false' | head -1"),
		"buzz-minio":           service.sshRecoveryCommandOutput(ctx, "systemctl", "is-active", "buzz-minio"),
		"minio-ready":          service.sshRecoveryCommandOutput(ctx, "sh", "-lc", "timeout 5 curl -fsS http://127.0.0.1:9000/minio/health/ready >/dev/null 2>&1 && echo ok || echo down"),
		"mm-db-data":           service.sshRecoveryCommandOutput(ctx, "sh", "-lc", "timeout 6 su - postgres -c \"psql -X -qAt -d mattermost -c \\\"SELECT 'users='||count(*) FROM users\\\"; psql -X -qAt -d mattermost -c \\\"SELECT 'posts='||count(*) FROM posts\\\"\" 2>&1 | tr '\\n' ' '"),
		"mm-env-ds":            service.sshRecoveryCommandOutput(ctx, "sh", "-lc", "{ systemctl show mattermost.service -p Environment -p EnvironmentFiles 2>/dev/null | tr ' ' '\\n' | grep -iE 'DATASOURCE|EnvironmentFiles'; for f in $(systemctl show mattermost.service -p EnvironmentFiles 2>/dev/null | sed 's/EnvironmentFiles=//' | tr ' ' '\\n' | sed 's/^-//'); do grep -h DATASOURCE \"$f\" 2>/dev/null; done; } | grep -iE 'test|datasource|Environment' | sed -E 's#://[^:]+:[^@]+@#://U:P@#' | head -3 | tr '\\n' '  '"),
	}
}

func (service *Service) runNamedRoomCommand(ctx context.Context, label string, command string, errorValue error) sshRecoveryCommandResult {
	if errorValue != nil {
		return sshRecoveryCommandResult{Name: label, Status: "error", Output: errorValue.Error()}
	}
	return service.runSSHRecoveryCommand(ctx, label, "sh", "-lc", command)
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
section sshd
ss -ltnp 2>/dev/null | grep ':22 ' || printf 'nothing listening on 22\n'
sshd -T 2>/dev/null | grep -iE '^(port|listenaddress|maxstartups|usepam|logingracetime) ' || printf 'sshd -T unavailable\n'
systemctl show ssh -p ActiveState,SubState,MainPID,NRestarts 2>/dev/null
printf 'local banner: '
timeout 8 bash -c 'exec 3<>/dev/tcp/127.0.0.1/22; IFS= read -r line <&3 && printf "%s" "$line"' 2>/dev/null || printf 'NO BANNER FROM 127.0.0.1:22'
printf '\n'
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

// setup takes the lock by creating a directory and releases it by name, so a run that
// dies between the two leaves it behind and every later setup is refused. /var/lock is
// sticky and the directory is root's, so the user setup connects as cannot clear it.
func releaseSetupLockCommand() string {
	return `set -eu
lock_directory=/var/lock/internkim-setup.lock
if [ ! -d "$lock_directory" ]; then
  echo "no setup lock is held"
  exit 0
fi
echo "== lock metadata =="
cat "$lock_directory/metadata.json" 2>/dev/null || echo "(no metadata)"
if pgrep -af "internkim setup" | grep -qv pgrep; then
  echo "== a setup is still running; leaving the lock alone =="
  pgrep -af "internkim setup" | grep -v pgrep
  exit 1
fi
rm -rf "$lock_directory"
echo "== released =="`
}
