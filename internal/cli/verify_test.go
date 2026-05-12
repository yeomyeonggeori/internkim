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
		"delete_verify_channel",
		"http://localhost:8065/api/v4/posts/$post_id",
		"curl --fail --silent --show-error -X DELETE",
		"http://localhost:8065/api/v4/users/$user_id?permanent=true",
		"http://localhost:8065/api/v4/users/$user_id\" >/dev/null",
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

func TestVerifyMattermostScriptUsesTemporaryChannel(t *testing.T) {
	script := verifyMattermostScript()
	requiredFragments := []string{
		"create_verify_channel",
		"verify_channel_name=\"verify-$timestamp\"",
		`'{team_id:$team_id,name:$name,display_name:$display_name,type:"P"}'`,
		"join_channel \"$bot_user_id\"",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected Mattermost verify temporary channel setup to include %q", fragment)
		}
	}
	if strings.Contains(script, "/root/.internkim/env/channel-id") {
		t.Fatal("expected Mattermost verify to avoid posting into the configured Town Square channel")
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
	script := verifyMattermostPromptScript("브라우저 열어줘.", false, 90, true, nil, nil)
	requiredFragments := []string{
		"delete_stale_probe_users",
		"probe-mattermost-",
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

func TestVerifyMattermostPromptScriptCanRequireToolAndTaskEvents(t *testing.T) {
	script := verifyMattermostPromptScript("1분마다 알려줘.", false, 90, false, []string{"schedule.create"}, []string{"schedule.created"})
	requiredFragments := []string{
		"expected_tools_json=",
		"expected_events_json=",
		"tool.$expected_tool.requested",
		"expected requested tool event for $expected_tool",
		"expected task event $expected_event",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected Mattermost prompt event verification to include %q", fragment)
		}
	}
}

func TestVerifyAPIScriptChecksLiteRTWithCPUAccelerator(t *testing.T) {
	script := verifyAPIScript()
	requiredFragments := []string{
		`accelerator: "cpu"`,
		`python3 -c 'import json, sys; document=json.load(sys.stdin); backend=document.get("selectedBackend"); content=document.get("content"); raise SystemExit(0 if backend in ("gpu", "cpu") and isinstance(content, str) and len(content) > 0 else 1)'`,
		`litert capability: %s`,
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected API verify script to include %q", fragment)
		}
	}
	if strings.Contains(script, `jq -e '.selectedBackend as $backend`) {
		t.Fatal("expected LiteRT verify to avoid raw jq parsing of possibly non-JSON responses")
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
