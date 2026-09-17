package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var relayWorkspaceTablePattern = regexp.MustCompile(`(?s)workspaceCapabilityPaths: Record<string, string> = \{(.*?)\n\};`)

var relayWorkspacePathPattern = regexp.MustCompile(`:\s*'(/[^']+)'`)

func relayWorkspacePaths(t *testing.T) []string {
	t.Helper()
	source, errorValue := os.ReadFile(filepath.Join("..", "..", "host", "relay", "forward.ts"))
	if errorValue != nil {
		t.Fatalf("read the relay source: %v", errorValue)
	}
	table := relayWorkspaceTablePattern.FindStringSubmatch(string(source))
	if table == nil {
		t.Fatal("the relay names no workspace capability table, so this guard is reading the wrong source")
	}
	matches := relayWorkspacePathPattern.FindAllStringSubmatch(table[1], -1)
	paths := make([]string, 0, len(matches))
	for _, match := range matches {
		paths = append(paths, match[1])
	}
	return paths
}

func relayActorsProbeForTest(t *testing.T, agentKey string, companyResponse string) *exec.Cmd {
	t.Helper()
	directory := t.TempDir()
	agentKeyPath := filepath.Join(directory, "agent-key")
	appURLPath := filepath.Join(directory, "app-url")
	if agentKey != "" {
		if errorValue := os.WriteFile(agentKeyPath, []byte(agentKey), 0o600); errorValue != nil {
			t.Fatal(errorValue)
		}
		if errorValue := os.WriteFile(appURLPath, []byte("https://company.example"), 0o600); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	companyResponsePath := filepath.Join(directory, "company.json")
	if errorValue := os.WriteFile(companyResponsePath, []byte(companyResponse), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	curlPath := filepath.Join(directory, "curl")
	curlScript := "#!/bin/sh\nset -eu\nrequest_url=\"\"\nfor argument do request_url=\"$argument\"; done\ncase \"$request_url\" in\n  https://company.example/api/agent/member) cat \"$COMPANY_RESPONSE_PATH\" ;;\n  *) exit 7 ;;\nesac\n"
	if errorValue := os.WriteFile(curlPath, []byte(curlScript), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	probe := "set -euo pipefail\n" + verifyRelayActorsScript(agentKeyPath, appURLPath) + "\nprintf '%s|%s' \"$relay_requester\" \"$relay_admin\"\n"
	command := exec.Command("bash", "-c", probe)
	command.Env = append(os.Environ(), "PATH="+directory+":"+os.Getenv("PATH"), "COMPANY_RESPONSE_PATH="+companyResponsePath)
	return command
}

func TestVerifyRelayActorsReadsTheCompanyDirectory(t *testing.T) {
	command := relayActorsProbeForTest(t, "agent-key", `{"members":[{"email":"member@example.com","role":"member","status":"active"},{"email":"withdrawn@example.com","role":"admin","status":"withdrawn"},{"email":"admin@example.com","role":"admin","status":"active"}]}`)
	output, errorValue := command.Output()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(output) != "member@example.com|admin@example.com" {
		t.Fatalf("actors = %q", output)
	}
}

func TestVerifyRelayActorsFailsOnAHostWithNoCompanyKey(t *testing.T) {
	command := relayActorsProbeForTest(t, "", `{"members":[{"email":"member@example.com","role":"member"},{"email":"admin@example.com","role":"admin","status":"active"}]}`)
	if _, errorValue := command.Output(); errorValue == nil {
		t.Fatal("a host with no company key verified relay actors against nothing")
	}
}

func TestVerifyAPIScriptProbesEveryPathTheRelayForwards(t *testing.T) {
	paths := relayWorkspacePaths(t)
	if len(paths) == 0 {
		t.Fatal("the relay forwards no workspace paths, so this guard is reading the wrong source")
	}

	script := verifyAPIScript()
	for _, path := range paths {
		if !strings.Contains(script, path) {
			t.Errorf("the relay forwards %s but device verification never asks for it, so a device that cannot answer it ships unnoticed", path)
		}
	}
}

func TestVerifyAPIScriptFailsOnARefusedRelayPath(t *testing.T) {
	script := verifyAPIScript()
	if !strings.Contains(script, "403|404)") {
		t.Error("device verification must fail on a refused relay path; a 403 is what a device missing the loopback actor answers")
	}
	if !strings.Contains(script, "X-INTERNKIM-REQUESTER-EMAIL") {
		t.Error("the relay names its requester in a header, so verification has to ask the same way it does")
	}
}

func TestVerifySkillInventoryScriptChecksBothActors(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		memberStatus  string
		adminStatus   string
		shouldSucceed bool
	}{
		{name: "expected permissions", memberStatus: "403", adminStatus: "200", shouldSucceed: true},
		{name: "member unexpectedly allowed", memberStatus: "200", adminStatus: "200"},
		{name: "administrator denied", memberStatus: "403", adminStatus: "403"},
		{name: "administrator server error", memberStatus: "403", adminStatus: "500"},
		{name: "transport failure", memberStatus: "curl-failure", adminStatus: "200"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			curlDirectory := t.TempDir()
			curlPath := filepath.Join(curlDirectory, "curl")
			curlScript := `#!/bin/sh
requester_header=
previous_argument=
for argument in "$@"; do
  if [ "$previous_argument" = "-H" ]; then requester_header="$argument"; break; fi
  previous_argument="$argument"
done
case "$requester_header" in
  "X-INTERNKIM-REQUESTER-EMAIL: member@example.com") status="$SKILL_MEMBER_STATUS" ;;
  "X-INTERNKIM-REQUESTER-EMAIL: admin@example.com") status="$SKILL_ADMIN_STATUS" ;;
  *) status="curl-failure" ;;
esac
if [ "$status" = "curl-failure" ]; then exit 7; fi
printf '%s' "$status"
`
			if errorValue := os.WriteFile(curlPath, []byte(curlScript), 0o700); errorValue != nil {
				t.Fatal(errorValue)
			}
			probe := "set -euo pipefail\nrelay_requester=member@example.com\nrelay_admin=admin@example.com\n" + verifySkillInventoryScript()
			command := exec.Command("bash", "-c", probe)
			command.Env = append(os.Environ(),
				"PATH="+curlDirectory+":"+os.Getenv("PATH"),
				"SKILL_MEMBER_STATUS="+testCase.memberStatus,
				"SKILL_ADMIN_STATUS="+testCase.adminStatus,
			)
			errorValue := command.Run()
			if (errorValue == nil) != testCase.shouldSucceed {
				t.Fatalf("expected success=%t, got %v", testCase.shouldSucceed, errorValue)
			}
		})
	}
}
