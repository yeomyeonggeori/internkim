package cli

import (
	"strings"
	"testing"
)

func TestVerifyMattermostScriptDeletesTestMessagesAndBotReplies(t *testing.T) {
	script := verifyMattermostScript()
	requiredFragments := []string{
		"delete_post",
		"delete_user",
		"delete_stale_verify_users",
		"delete_verify_replies",
		"http://localhost:8065/api/v4/posts/$post_id",
		"http://localhost:8065/api/v4/users/$user_id?permanent=true",
		"invited_post_id",
		"uninvited_post_id",
		".user_id == $bot_user_id and .create_at >= $test_started_at",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected Mattermost verify cleanup to include %q", fragment)
		}
	}
}

func TestVerifyMattermostScriptUsesStrictChannelMembership(t *testing.T) {
	script := verifyMattermostScript()
	requiredFragments := []string{
		"api_request \"join team $user_id\"",
		"api_request \"join channel $user_id\"",
		"api_request \"verify channel membership $user_id\"",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected Mattermost verify strict join to include %q", fragment)
		}
	}
}

func TestVerifyMattermostPromptScriptCanRequireBrowserOpenSuccess(t *testing.T) {
	script := verifyMattermostPromptScript("브라우저 열어줘.", false, 90, true)
	requiredFragments := []string{
		"expect_browser_open=true",
		"tool.browser.open.result",
		".isError != true",
		"expected successful tool.browser.open.result",
		"browserOpenVerified: $browser_open_verified",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected Mattermost prompt browser verification to include %q", fragment)
		}
	}
}

func TestParseMattermostBrowserOpenE2EPreparationUsesLastJSONLine(t *testing.T) {
	preparation, errorValue := parseMattermostBrowserOpenE2EPreparation(`curl: (52) Empty reply from server
{"deviceURL":"https://device.example","code":"1234-5678","email":"probe@example.com","username":"probe","password":"secret","userID":"user-1","channelID":"channel-1"}
`)
	if errorValue != nil {
		t.Fatalf("expected preparation parse success: %v", errorValue)
	}
	if preparation.Code != "1234-5678" || preparation.UserID != "user-1" {
		t.Fatalf("unexpected preparation: %+v", preparation)
	}
}
