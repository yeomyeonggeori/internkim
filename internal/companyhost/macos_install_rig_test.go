package companyhost

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func readTheMacInstallRig(t *testing.T) string {
	t.Helper()
	contents, errorValue := os.ReadFile(filepath.Join("..", "..", "tools", "test-macos-install"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return string(contents)
}

func TestTheMacRigExpectsEveryDaemonTheInstallWrites(t *testing.T) {
	layout := blueclaw.MacCompanyHostLayout(homebrewPrefixForTest)
	dataDaemons, errorValue := blueclaw.CompanyHostDataLaunchDaemons(layout)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	written := len(blueclaw.CompanyHostServices(layout)) + len(dataDaemons)

	match := regexp.MustCompile(`(?m)^DAEMON_COUNT = (\d+)$`).FindStringSubmatch(readTheMacInstallRig(t))
	if match == nil {
		t.Fatal("tools/test-macos-install no longer declares DAEMON_COUNT")
	}
	expected, _ := strconv.Atoi(match[1])
	if expected != written {
		t.Fatalf("the rig expects %d daemons and a Mac install writes %d", expected, written)
	}
}

func TestTheMacRigUndoesEveryAccountTheInstallCreates(t *testing.T) {
	match := regexp.MustCompile(`(?m)^SERVICE_ACCOUNTS = \(([^)]*)\)$`).FindStringSubmatch(readTheMacInstallRig(t))
	if match == nil {
		t.Fatal("tools/test-macos-install no longer declares SERVICE_ACCOUNTS")
	}
	rigAccounts := map[string]bool{}
	for _, quoted := range regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(match[1], -1) {
		rigAccounts[quoted[1]] = true
	}
	installAccounts := map[string]bool{}
	for _, account := range blueclaw.CompanyHostServiceAccounts(blueclaw.MacCompanyHostLayout(homebrewPrefixForTest)) {
		installAccounts[account.Name] = true
	}
	for name := range installAccounts {
		if !rigAccounts[name] {
			t.Errorf("a Mac install creates %s and the rig's --undo does not remove it", name)
		}
	}
	for name := range rigAccounts {
		if !installAccounts[name] {
			t.Errorf("the rig removes %s and a Mac install never creates it", name)
		}
	}
}
