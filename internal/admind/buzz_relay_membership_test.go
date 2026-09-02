package admind

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/buzzidentity"
)

func serviceRecordingRelayMembershipGrants(t *testing.T) (*Service, string) {
	t.Helper()
	service := serviceWhoseRecordIsDown(t)
	directory := t.TempDir()
	grantsPath := filepath.Join(directory, "granted")
	commandPath := filepath.Join(directory, "buzz-admin")
	script := "#!/bin/sh\nwhile [ $# -gt 0 ]; do\n  if [ \"$1\" = \"--pubkey\" ]; then printf '%s\\n' \"$2\" >> " + grantsPath + "; fi\n  shift\ndone\n"
	if errorValue := os.WriteFile(commandPath, []byte(script), 0o700); errorValue != nil {
		t.Fatalf("write fake buzz-admin: %v", errorValue)
	}
	service.Configuration.BuzzAdminCommandPath = commandPath
	service.Configuration.BuzzRelayURL = "ws://127.0.0.1:3000"
	service.Configuration.BuzzDatabaseURL = "postgres://buzz@127.0.0.1:1/buzz?sslmode=disable"
	return service, grantsPath
}

func TestRelayMembershipIsGrantedBeforeAnyRoomExists(t *testing.T) {
	service, grantsPath := serviceRecordingRelayMembershipGrants(t)

	service.ensureMemberChannelMembership(context.Background())

	granted, errorValue := os.ReadFile(grantsPath)
	if errorValue != nil {
		t.Fatalf("a relay with no rooms let nobody in, so nothing could ever open one: %v", errorValue)
	}
	agentPubkey, errorValue := buzzPublicKey(buzzidentity.Secret(identityTestSeed, buzzidentity.AgentSubject))
	if errorValue != nil {
		t.Fatalf("agent pubkey: %v", errorValue)
	}
	if !strings.Contains(string(granted), agentPubkey) {
		t.Fatalf("the agent must be a relay member before it can open the first room, got %s", strings.TrimSpace(string(granted)))
	}
}
