package cli

import (
	"os"
	"os/exec"
	"path/filepath"
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
	script := verifyMattermostPromptScript("브라우저 열어줘.", false, 90, true, false, nil, nil, false, false, false)
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
	script := verifyMattermostPromptScript("1분마다 알려줘.", false, 90, false, false, []string{"schedule.create"}, []string{"schedule.created"}, false, false, false)
	requiredFragments := []string{
		"expected_tools_json=",
		"expected_events_json=",
		`[ "$task_status" = "completed" ]`,
		"tool.$expected_tool.requested",
		"expected requested tool event for $expected_tool",
		"expected task event $expected_event",
		"fetch_latest_bot_post",
		"fetch latest bot reply",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected Mattermost prompt event verification to include %q", fragment)
		}
	}
}

func TestVerifyMattermostPromptScriptCanRequirePublicSiteURL(t *testing.T) {
	script := verifyMattermostPromptScript(defaultVerifySitePrompt, false, 90, false, true, []string{"site.create", "terminal.run", "site.publish"}, nil, false, false, false)
	requiredFragments := []string{
		"expect_public_url=true",
		"wait for final site reply",
		"public_url_verified=false",
		"grep -Eo 'https://[^[:space:])>]+'",
		"expected_tools_json=",
		"tool.$expected_tool.requested",
		"Sorry, we could not find the page.",
		"INTERNKIM_SITE_STARTER_REPLACE_ME",
		"site public URL returned starter scaffold",
		"site public URL did not return valid HTML",
		"capture_site_screenshots",
		"prepare_rootfs_browser_chroot",
		"/opt/internkim/blueclaw-runtime/rootfs.ext4",
		"--user-data-dir=\"$profile_dir\"",
		"site screenshot capture failed; chromium diagnostics follow",
		"--screenshot=\"$desktop_screenshot_file\"",
		"--screenshot=\"$mobile_screenshot_file\"",
		"siteScreenshotsVerified: $site_screenshots_verified",
		"siteScreenshotFiles: [$desktop_screenshot_file, $mobile_screenshot_file]",
		"site deploy final reply contained a generic infrastructure excuse",
		"http://127.0.0.1:8080/admin/api/sites/$site_id",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected Mattermost prompt site verification to include %q", fragment)
		}
	}
}

func TestVerifySiteRequiresExplicitTarget(t *testing.T) {
	errorValue := runVerifyArguments([]string{"site"})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "saved physical board") {
		t.Fatalf("expected verify site to reject implicit physical target, got %v", errorValue)
	}
}

func TestVerifyMattermostSitePromptRequiresExplicitTarget(t *testing.T) {
	errorValue := runVerifyArguments([]string{
		"mattermost",
		"--prompt",
		"웹사이트 하나 만들어서 배포해줘",
		"--expect-tool",
		"site.create",
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "saved physical board") {
		t.Fatalf("expected Mattermost site verification to reject implicit physical target, got %v", errorValue)
	}
}

func TestVerifyMattermostPromptScriptIsValidShell(t *testing.T) {
	script := verifyMattermostPromptScript(defaultVerifySitePrompt, false, 90, false, true, []string{"site.create", "terminal.run", "site.publish"}, nil, false, false, false)
	scriptPath := filepath.Join(t.TempDir(), "verify-site.sh")
	if errorValue := os.WriteFile(scriptPath, []byte(script), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	output, errorValue := exec.Command("bash", "-n", scriptPath).CombinedOutput()
	if errorValue != nil {
		t.Fatalf("expected generated verify script to parse, got %v: %s", errorValue, string(output))
	}
}

func TestVerifyMattermostPromptScriptCanDownloadFinalAttachments(t *testing.T) {
	script := verifyMattermostPromptScript("짧은 발표자료 만들어줘.", true, 90, false, false, []string{"file.deliver"}, nil, true, false, false)
	requiredFragments := []string{
		"download_files=true",
		"download_bot_files",
		"http://localhost:8065/api/v4/files/$file_id/info",
		"http://localhost:8065/api/v4/files/$file_id",
		"--rawfile content_base64",
		"contentBase64:$content_base64",
		"downloadedFiles: ($downloaded_files[0] // [])",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected Mattermost prompt attachment download to include %q", fragment)
		}
	}
}

func TestVerifyMattermostPromptScriptCanWaitForCompletion(t *testing.T) {
	script := verifyMattermostPromptScript("보고서 워드 파일로 만들어줘.", false, 90, false, false, nil, nil, true, true, false)
	requiredFragments := []string{
		"wait_for_completion=true",
		"find_probe_task_run_id",
		"should_wait_for_task=true",
		"expected a task for probe prompt before waiting for completion",
		`[ "$should_wait_for_task" = "true" ]`,
		`[ "$task_status" != "completed" ]`,
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected Mattermost prompt completion wait to include %q", fragment)
		}
	}
}

func TestVerifyMattermostPromptScriptCanAutoConfirm(t *testing.T) {
	script := verifyMattermostPromptScript("일정을 삭제해줘.", false, 90, false, false, nil, nil, true, true, true)
	requiredFragments := []string{
		"auto_confirm=true",
		"approval_sent=false",
		`[ "$auto_confirm" = "true" ]`,
		"confirmation.requested",
		"post probe approval",
		`message:"해"`,
		"autoConfirmationSent: $auto_confirmation_sent",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected Mattermost prompt auto confirmation to include %q", fragment)
		}
	}
}

func TestWriteDownloadedMattermostFilesCanAllowNoAttachments(t *testing.T) {
	downloadedFilePaths, errorValue := writeDownloadedMattermostFilesAllowEmpty(`{"downloadedFiles":[]}`, t.TempDir())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(downloadedFilePaths) != 0 {
		t.Fatalf("expected no downloaded files, got %v", downloadedFilePaths)
	}
}

func TestParseMattermostVerificationOutputAllowsTrailingCleanupLogs(t *testing.T) {
	output := `{"ok":true,"botMessage":"done","downloadedFiles":[],"fileIDs":[]}` + "\n" +
		"curl: (22) The requested URL returned error: 401\n" +
		"jq: parse error: Invalid numeric literal at line 1, column 9\n"
	verificationOutput, errorValue := parseMattermostVerificationOutput(output)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if verificationOutput.BotMessage != "done" {
		t.Fatalf("unexpected bot message: %q", verificationOutput.BotMessage)
	}
}

func TestVerifyMattermostMessageDeleteE2EChecksIdentityBeforeWaitingForTask(t *testing.T) {
	script := verifyMattermostMessageDeleteE2EScript(false, 90)
	requiredFragments := []string{
		"verify delete probe policy",
		"identity.resolve",
		"delete E2E Mattermost identity resolve did not return the probe email",
		"probe identity resolve: $identity_response",
		"diagnose delete E2E channel posts",
		"connector event diagnostics for Mattermost message",
		"/admin/api/connector/events?platform=mattermost&messageID=$message_id&limit=5",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected Mattermost delete E2E script to include %q", fragment)
		}
	}
}

func TestRedactDownloadedMattermostFilesHidesAttachmentBytes(t *testing.T) {
	output := "log line\n" + `{"downloadedFiles":[{"fileID":"file-1","filename":"deck.html","contentBase64":"YWJjZA=="}]}` + "\n"
	redactedOutput := redactDownloadedMattermostFiles(output)
	if strings.Contains(redactedOutput, "YWJjZA==") {
		t.Fatalf("expected attachment base64 to be redacted, got %s", redactedOutput)
	}
	if !strings.Contains(redactedOutput, "redacted 8 base64 chars") {
		t.Fatalf("expected redaction marker, got %s", redactedOutput)
	}
}

func TestWriteDownloadedMattermostFilesWritesAttachments(t *testing.T) {
	downloadDirectory := t.TempDir()
	output := `{"downloadedFiles":[{"fileID":"file-1","filename":"deck.html","contentBase64":"PGh0bWw+PC9odG1sPg=="}]}`
	if errorValue := writeDownloadedMattermostFiles(output, downloadDirectory); errorValue != nil {
		t.Fatal(errorValue)
	}
	content, errorValue := os.ReadFile(filepath.Join(downloadDirectory, "deck.html"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(content) != "<html></html>" {
		t.Fatalf("unexpected downloaded file content: %s", string(content))
	}
}

func TestRunVerifyArgumentsAcceptsSiteSubcommand(t *testing.T) {
	errorValue := runVerifyArguments([]string{"site", "--unknown-site-flag"})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "flag provided but not defined") {
		t.Fatalf("expected site verify subcommand to resolve target, got %v", errorValue)
	}
}

func TestVerifyAPIScriptChecksLiteRTWithCPUAccelerator(t *testing.T) {
	script := verifyAPIScript()
	requiredFragments := []string{
		`accelerator: "cpu"`,
		`python3 -c 'import json, sys; document=json.load(sys.stdin); backend=document.get("selectedBackend"); content=document.get("content"); raise SystemExit(0 if backend in ("gpu", "cpu") and isinstance(content, str) and len(content) > 0 else 1)'`,
		`litert capability: %s`,
		`litert capability: optional local check failed`,
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

func TestVerifyAPIScriptChecksRemoteStructuredLLM(t *testing.T) {
	script := verifyAPIScript()
	requiredFragments := []string{
		`executionMode: "remote"`,
		`executionMode: "auto"`,
		`http://internkim/v1/llm/structured`,
		`--argjson schema "$schema"`,
		`structuredOutputSchema: {name:"smoke_reply", document:$schema, isStrictlyEnforced:true}`,
		`structuredOutputSchema: {name:"blueclaw_agent_turn_action", document:$schema, isStrictlyEnforced:true}`,
		`.provider == "openrouter" and .selectedBackend == "remote"`,
		`.constraintMode == "native_tool_call"`,
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected API verify script to include %q", fragment)
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
