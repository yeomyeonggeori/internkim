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
		return errors.New("usage: internkim reset blueclaw-history [--host <ip>] [--user <user>] [--password <password>] [--confirm <deviceID>] [--keep-mattermost-posts]")
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
	host := flagSet.String("host", "", "Board host")
	user := flagSet.String("user", "", "SSH user")
	password := flagSet.String("password", "", "SSH password")
	board := flagSet.String("board", "", "Board target")
	simulation := flagSet.Bool("sim", false, "Use simulation target")
	confirmDeviceID := flagSet.String("confirm", "", "Device ID required to execute the reset")
	isPlanOnly := flagSet.Bool("plan", false, "Print the reset plan without changing the board")
	keepMattermostPosts := flagSet.Bool("keep-mattermost-posts", false, "Keep visible Mattermost posts")
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return errorValue
	}

	targetArguments := []string{}
	if strings.TrimSpace(*host) != "" {
		targetArguments = append(targetArguments, "--host", *host)
	}
	if strings.TrimSpace(*user) != "" {
		targetArguments = append(targetArguments, "--user", *user)
	}
	if strings.TrimSpace(*password) != "" {
		targetArguments = append(targetArguments, "--password", *password)
	}
	if strings.TrimSpace(*board) != "" {
		targetArguments = append(targetArguments, "--board", *board)
	}
	if *simulation {
		targetArguments = append(targetArguments, "--sim")
	}
	verifyTarget, errorValue := resolveVerifyTarget(targetArguments)
	if errorValue != nil {
		return errorValue
	}

	deviceID := strings.TrimSpace(verifyTarget.sshClient.run("cat /root/.internkim/env/device-id 2>/dev/null || true"))
	if deviceID == "" {
		return errors.New("device id not found on target")
	}

	fmt.Printf("Target: %s@%s\n", verifyTarget.user, verifyTarget.host)
	fmt.Printf("Device ID: %s\n", deviceID)
	printBlueclawHistoryResetPlan(*keepMattermostPosts)

	if *isPlanOnly {
		return nil
	}
	if strings.TrimSpace(*confirmDeviceID) != deviceID {
		return fmt.Errorf("refusing to reset; pass --confirm %s", deviceID)
	}

	output, errorValue := verifyTarget.sshClient.runResult(blueclawHistoryResetScript(*keepMattermostPosts))
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

func printBlueclawHistoryResetPlan(keepMattermostPosts bool) {
	fmt.Println("This will delete Blueclaw conversation/runtime data:")
	fmt.Println("  - task runs, task events, task steps, task artifacts, waits, sessions, schedules")
	fmt.Println("  - raw events, attachments, content segments, conversations")
	fmt.Println("  - legacy memory records/sources")
	fmt.Println("  - Graphiti episode/namespace mirror rows")
	fmt.Println("  - Graphiti Kuzu files under /root/.blueclaw/workspace/.blueclaw/graphiti/kuzu*")
	if keepMattermostPosts {
		fmt.Println("This will keep Mattermost visible posts.")
		return
	}
	fmt.Println("This will also delete visible Mattermost posts while keeping Mattermost users, teams, channels, and secrets.")
	fmt.Println("This will keep people, invited emails, platform account links, policy, and secrets.")
}

func blueclawHistoryResetScript(keepMattermostPosts bool) string {
	script := `set -euo pipefail
echo "stopping blueclaw services"
systemctl stop blueclaw graphiti-memoryd

echo "resetting blueclaw task, conversation, and memory tables"
sudo -u postgres psql -d blueclaw <<'SQL'
TRUNCATE TABLE
  task_event,
  task_step,
  task_artifact,
  task_wait_token,
  task_session,
  task_schedule,
  task_run,
  memory_source,
  memory_record,
  content_segment,
  attachment,
  raw_event,
  conversation,
  graphiti_episode,
  graphiti_namespace
RESTART IDENTITY CASCADE;
SQL

echo "removing graphiti kuzu files"
mkdir -p /root/.blueclaw/workspace/.blueclaw/graphiti
find /root/.blueclaw/workspace/.blueclaw/graphiti -maxdepth 1 -name 'kuzu*' -exec rm -rf -- {} +
chown -R blueclaw:blueclaw /root/.blueclaw/workspace/.blueclaw/graphiti

`
	if !keepMattermostPosts {
		script += mattermostVisiblePostsResetScript()
	}
	script += `echo "starting blueclaw services"
systemctl start graphiti-memoryd blueclaw

echo "waiting for graphiti-memoryd"
for attempt in $(seq 1 60); do
  if curl --silent --show-error --fail http://127.0.0.1:7791/health >/dev/null 2>&1; then
    echo "graphiti-memoryd: ok"
    break
  fi
  sleep 1
done

echo "waiting for blueclaw"
for attempt in $(seq 1 60); do
  if curl --silent --show-error --fail http://127.0.0.1:8080/admin/api/policy >/dev/null 2>&1; then
    echo "blueclaw: ok"
    echo "blueclaw history and memory reset complete"
    exit 0
  fi
  sleep 1
done

echo "blueclaw: failed"
exit 1`
	return script
}

func mattermostVisiblePostsResetScript() string {
	return `echo "stopping mattermost"
systemctl stop mattermost || true

echo "resetting Mattermost visible posts"
sudo -u postgres psql -d mattermost <<'SQL'
DO $$
DECLARE
  reset_time bigint := (extract(epoch from now()) * 1000)::bigint;
BEGIN
  IF to_regclass('public.posts') IS NOT NULL THEN
    UPDATE posts
    SET deleteat = reset_time,
        updateat = reset_time
    WHERE deleteat = 0;
  END IF;

  IF to_regclass('public.reactions') IS NOT NULL THEN
    DELETE FROM reactions;
  END IF;

  IF to_regclass('public.threadmemberships') IS NOT NULL THEN
    DELETE FROM threadmemberships;
  END IF;

  IF to_regclass('public.threads') IS NOT NULL THEN
    DELETE FROM threads;
  END IF;
END $$;
SQL

echo "starting mattermost"
systemctl start mattermost

`
}
