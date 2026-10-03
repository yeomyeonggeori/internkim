package blueclaw

import (
	"testing"
)

func TestChatdOnTheCompanyHostKeepsItsDeliveryRecordOnDisk(t *testing.T) {
	chatd, isBundled := CompanyHostServiceNamed(LinuxCompanyHostLayout(), ChatdServiceName)
	if !isBundled {
		t.Fatalf("the bundle carries no %s", ChatdServiceName)
	}
	directory, isSet := environmentSettingOf(chatd, "CHATD_STATE_DIRECTORY")
	if !isSet || directory != CompanyHostChatdStatePath {
		t.Fatalf("chatd is told %q as its state directory, want %s", directory, CompanyHostChatdStatePath)
	}
}
