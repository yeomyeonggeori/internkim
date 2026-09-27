package blueclaw

import (
	"strings"
	"testing"
)

func TestChatdOnTheDeviceKeepsItsDeliveryRecordOnDisk(t *testing.T) {
	unit := ChatdServiceUnit("")
	if !strings.Contains(unit, "Environment=CHATD_STATE_DIRECTORY="+ChatdStateDirectoryPath+"\n") {
		t.Fatalf("chatd is given no state directory, so a restart forgets which messages it handed over, got:\n%s", unit)
	}
}

func TestChatdOnTheCompanyHostKeepsItsDeliveryRecordOnDisk(t *testing.T) {
	chatd, isBundled := CompanyHostServiceNamed(DebianCompanyHostLayout(), ChatdServiceName)
	if !isBundled {
		t.Fatalf("the bundle carries no %s", ChatdServiceName)
	}
	directory, isSet := environmentSettingOf(chatd, "CHATD_STATE_DIRECTORY")
	if !isSet || directory != CompanyHostChatdStatePath {
		t.Fatalf("chatd is told %q as its state directory, want %s", directory, CompanyHostChatdStatePath)
	}
}
