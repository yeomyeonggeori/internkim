package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

func runReset() {
	if errorValue := runResetArguments(osArgsTail()); errorValue != nil {
		fmt.Fprintf(os.Stderr, "✗ %v\n", errorValue)
		os.Exit(1)
	}
}

func osArgsTail() []string {
	if len(os.Args) <= 2 {
		return nil
	}
	return os.Args[2:]
}

func runResetArguments(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: internkim reset blueclaw-history [--node <nodeID>] [--host <ip>] [--user <user>] [--password <password>] [--confirm <fleetID>]")
	}

	switch arguments[0] {
	case "blueclaw-history":
		return runResetBlueclawHistory(arguments[1:])
	default:
		return fmt.Errorf("unknown reset target: %s", arguments[0])
	}
}

func runResetBlueclawHistory(arguments []string) error {
	flagSet := flag.NewFlagSet("reset blueclaw-history", flag.ContinueOnError)
	target := registerTargetFlags(flagSet)
	confirmFleetID := flagSet.String("confirm", "", "Fleet ID required to execute the reset")
	confirmNodeID := flagSet.String("confirm-node", "", "Node ID required for --node-local reset")
	isNodeLocal := flagSet.Bool("node-local", false, "Confirm that the reset is scoped to one node")
	isPlanOnly := flagSet.Bool("plan", false, "Print the reset plan without changing the board")
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return errorValue
	}

	verifyTarget, errorValue := target.resolveVerifyTarget()
	if errorValue != nil {
		return errorValue
	}

	fleetID := strings.TrimSpace(verifyTarget.sshClient.run("cat /root/.internkim/env/fleet-id 2>/dev/null || true"))
	if fleetID == "" {
		return errors.New("fleet id not found on target")
	}

	fmt.Printf("Target: %s@%s\n", verifyTarget.user, verifyTarget.host)
	fmt.Printf("Fleet ID: %s\n", fleetID)
	printBlueclawHistoryResetPlan()

	if *isPlanOnly {
		return nil
	}
	if strings.TrimSpace(*confirmFleetID) != fleetID {
		return fmt.Errorf("refusing to reset; pass --confirm %s", fleetID)
	}
	if *isNodeLocal {
		nodeID := strings.TrimSpace(verifyTarget.nodeID)
		if nodeID == "" {
			nodeID = strings.TrimSpace(verifyTarget.sshClient.run("cat /root/.internkim/env/node-id 2>/dev/null || true"))
		}
		if strings.TrimSpace(*confirmNodeID) != nodeID {
			return fmt.Errorf("refusing node-local reset; pass --confirm-node %s", nodeID)
		}
	}

	output, errorValue := verifyTarget.sshClient.runResult(blueclawHistoryResetScript())
	if strings.TrimSpace(output) != "" {
		fmt.Print(output)
		if !strings.HasSuffix(output, "\n") {
			fmt.Println()
		}
	}
	if errorValue != nil {
		return fmt.Errorf("reset blueclaw history failed: %w", errorValue)
	}
	return nil
}

func printBlueclawHistoryResetPlan() {
	fmt.Println("This will delete Blueclaw conversation/runtime data:")
	fmt.Println("  - task runs, task events, task steps, task artifacts, waits, sessions, schedules")
	fmt.Println("  - raw events, attachments, content segments, conversations")
	fmt.Println("  - every per-subject memory file under the workspace's .blueclaw/memory")
	fmt.Println("  - guest workspace Postgres runtime state when /var/lib/blueclaw/workspace.ext4 exists")
	fmt.Println("This will keep host policy and secrets. Guest runtime mirrors are rebuilt from policy on restart.")
}

func blueclawHistoryResetScript() string {
	return `set -euo pipefail
echo "stopping blueclaw services"
systemctl stop blueclaw 2>/dev/null || true

echo "resetting host blueclaw task and conversation tables"
if su -s /bin/bash postgres -c "psql -d blueclaw -Atc 'SELECT 1'" >/dev/null 2>&1; then
  su -s /bin/bash postgres -c "psql -v ON_ERROR_STOP=1 -d blueclaw" <<'SQL'
TRUNCATE TABLE
  task_event,
  task_step,
  task_artifact,
  task_wait_token,
  task_session,
  schedule,
  task_run,
  attachment,
  raw_event,
  conversation
RESTART IDENTITY CASCADE;
SQL
fi

if [ -s /var/lib/blueclaw/workspace.ext4 ]; then
  echo "resetting guest workspace runtime database"
  mount_path="$(mktemp -d /mnt/internkim-blueclaw-reset.XXXXXX)"
  cleanup_workspace_mount() {
    if mountpoint -q "$mount_path"; then
      umount "$mount_path"
    fi
    rmdir "$mount_path" 2>/dev/null || true
  }
  trap cleanup_workspace_mount EXIT
  mount -o loop /var/lib/blueclaw/workspace.ext4 "$mount_path"
  mkdir -p "$mount_path/.blueclaw/postgres"
  rm -rf "$mount_path/.blueclaw/postgres/data"
  chown -R postgres:postgres "$mount_path/.blueclaw/postgres"
  chmod 0770 "$mount_path/.blueclaw/postgres"
  echo "clearing guest per-subject memory files"
  rm -rf "$mount_path/.blueclaw/memory"
  cleanup_workspace_mount
  trap - EXIT
else
  echo "clearing per-subject memory files"
  rm -rf /workspace/.blueclaw/memory
fi

echo "starting blueclaw services"
systemctl start blueclaw

echo "waiting for blueclaw"
for attempt in $(seq 1 180); do
  if curl --silent --show-error --fail http://127.0.0.1:8080/admin/api/policy >/dev/null 2>&1; then
    echo "blueclaw: ok"
    echo "blueclaw history and memory reset complete"
    exit 0
  fi
  sleep 1
done

echo "blueclaw: failed"
exit 1`
}
