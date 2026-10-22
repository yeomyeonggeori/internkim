package box

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/companyhost"
)

func sampleConnectionDocument(t *testing.T) []byte {
	t.Helper()
	document, errorValue := json.Marshal(companyhost.Connection{
		SchemaVersion: 1,
		AppURL:        "https://intern.example.test",
		Company:       companyhost.Company{ID: sampleCompanyID, Name: "샘플 회사", Slug: "sample"},
		CentralPlane:  companyhost.CentralPlane{ProjectURL: "https://project.example.test", PublishableKey: "publishable"},
		GatewayURL:    "wss://gateway.example.test",
		AgentKey:      "header.claims.signature",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return document
}

func writeConnectionFile(t *testing.T, path string, document []byte) {
	t.Helper()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(path, document, 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestABoxWhoseInstallWasCutShortInstallsAgainOnItsNextBoot(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "header.claims.signature"}
	recorded := &installs{}
	daemon, places := daemonFor(t, plane, recorded)
	plane.sealedModelKey = sealTo(t, identityOf(t, places), "sk-or-v1-sample")
	cutShort := daemon
	cutShort.Install = func(companyhost.Request) error {
		writeConnectionFile(t, places.ConnectionFilePath, sampleConnectionDocument(t))
		return errors.New("the power went out")
	}
	runSteps(t, cutShort, 1)

	runSteps(t, daemon, 1)

	if len(recorded.requests) != 1 || daemon.installedCompany() != sampleCompanyID {
		t.Fatalf("installs = %d, installed company = %q", len(recorded.requests), daemon.installedCompany())
	}
}

func TestABoxLeftWithAnEmptyConnectionFileInstallsAgain(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "header.claims.signature"}
	recorded := &installs{}
	daemon, places := daemonFor(t, plane, recorded)
	plane.sealedModelKey = sealTo(t, identityOf(t, places), "sk-or-v1-sample")
	writeConnectionFile(t, places.ConnectionFilePath, nil)

	runSteps(t, daemon, 1)

	if len(recorded.requests) != 1 || daemon.installedCompany() != sampleCompanyID {
		t.Fatalf("installs = %d, installed company = %q", len(recorded.requests), daemon.installedCompany())
	}
}

func TestAResetFileIsHonouredOnAComputerThatLooksConnectedByFile(t *testing.T) {
	plane := &fakePlane{isClaimed: true}
	recorded := &installs{}
	daemon, places := daemonFor(t, plane, recorded)
	writeConnectionFile(t, places.ConnectionFilePath, sampleConnectionDocument(t))
	daemon.Places.ResetRequestPath = filepath.Join(filepath.Dir(places.StateDirectoryPath), "boot", "internkim-reset")
	writeConnectionFile(t, daemon.Places.ResetRequestPath, nil)

	runSteps(t, daemon, 1)

	if plane.releases != 1 || plane.announcements != 1 {
		t.Fatalf("releases = %d, announcements = %d", plane.releases, plane.announcements)
	}
	if _, errorValue := os.Stat(daemon.Places.ResetRequestPath); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("the reset file is still there: %v", errorValue)
	}

	runSteps(t, daemon, 1)

	if plane.announcements != 2 {
		t.Fatalf("after its reset the box announced itself %d times across two boots, want 2", plane.announcements)
	}
}
