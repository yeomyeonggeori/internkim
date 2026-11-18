package box

import (
	"os"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/companyhost"
)

func TestAKeptSessionIsTheAgentKeyTheConnectionFileCarries(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "first.session.token"}
	daemon, places := daemonFor(t, plane, &installs{})
	writeConnectionFile(t, places.ConnectionFilePath, sampleConnectionDocument(t))
	writeCredentialFiles(t, places)
	before, errorValue := companyhost.ReadConnection(places.ConnectionFilePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := KeepTheSession(daemon.Places, "second.session.token"); errorValue != nil {
		t.Fatal(errorValue)
	}

	after, errorValue := companyhost.ReadConnection(places.ConnectionFilePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if after.AgentKey != "second.session.token" {
		t.Fatalf("the connection file still carries %q after the box kept a new session", after.AgentKey)
	}
	before.AgentKey = after.AgentKey
	if after != before {
		t.Fatalf("keeping the session changed more of the connection than its agent key: %+v", after)
	}
	information, errorValue := os.Stat(places.ConnectionFilePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if information.Mode().Perm() != 0o600 {
		t.Fatalf("the connection file is mode %o after the session was kept", information.Mode().Perm())
	}
}
