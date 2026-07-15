package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type fakeMattermostScenarioRemote struct {
	runValue func(string) (string, error)
	scripts  []string
}

func (fake *fakeMattermostScenarioRemote) run(_ context.Context, script string) (string, error) {
	fake.scripts = append(fake.scripts, script)
	return fake.runValue(script)
}

func TestMattermostScenarioWorkspaceFilesRecursesThroughPathGlob(t *testing.T) {
	remote := &fakeMattermostScenarioRemote{runValue: func(script string) (string, error) {
		switch {
		case strings.Contains(script, "workspace/download"):
			return "quarterly report", nil
		case strings.Contains(script, "%2Fquarterly"):
			return `{"entries":[{"name":"review.docx","isDirectory":false}]}`, nil
		case strings.Contains(script, "path=%2Fworkspace%2Fcircles%2Fstaff%2Freports"):
			return `{"entries":[{"name":"quarterly","isDirectory":true}]}`, nil
		case strings.Contains(script, "path=%2Fworkspace%2Fcircles%2Fstaff"):
			return `{"entries":[{"name":"reports","isDirectory":true}]}`, nil
		case strings.Contains(script, "path=%2Fworkspace%2Fcircles"):
			return `{"entries":[{"name":"staff","isDirectory":true}]}`, nil
		case strings.Contains(script, "path=%2Fworkspace"):
			return `{"entries":[{"name":"circles","isDirectory":true}]}`, nil
		default:
			return `{"entries":[]}`, nil
		}
	}}
	admin := mattermostScenarioAdmin{remote: remote}
	step := mattermostScenarioStep{ExpectedWorkspaceFiles: []mattermostScenarioWorkspaceFile{{PathGlob: "circles/staff/reports/*/*.docx"}}}

	files, errorValue := admin.workspaceFiles(context.Background(), step)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(files) != 1 || files[0].Path != "/workspace/circles/staff/reports/quarterly/review.docx" || files[0].Content != "quarterly report" {
		t.Fatalf("unexpected workspace files: %#v scripts=%#v", files, remote.scripts)
	}
}

func TestMattermostScenarioCleanupRediscoversAndDeletesConversationResources(t *testing.T) {
	conversationID := "thread:channel:root"
	taskListCount := 0
	siteListCount := 0
	remote := &fakeMattermostScenarioRemote{runValue: func(script string) (string, error) {
		switch {
		case strings.Contains(script, "/admin/api/task/cancel"):
			return `{}`, nil
		case strings.Contains(script, "/admin/api/task/delete"):
			return `{}`, nil
		case strings.Contains(script, "/admin/api/sites/site-matching"):
			return `{}`, nil
		case strings.Contains(script, "/admin/api/sites"):
			siteListCount++
			if siteListCount == 1 {
				return `{"sites":[{"siteID":"site-other","conversationID":"other"},{"siteID":"site-matching","conversationID":"thread:channel:root","owner":"owner@example.com","ownerIdentity":{"personID":"person-owner","platform":"mattermost","platformUserID":"user-owner"}}]}`, nil
			}
			return `{"sites":[{"siteID":"site-other","conversationID":"other"}]}`, nil
		case strings.Contains(script, "/admin/api/people"):
			return `{}`, nil
		case strings.Contains(script, "/admin/api/task"):
			taskListCount++
			if taskListCount == 1 {
				return `[{"taskRunID":"task-1","originConversationID":"thread:channel:root"},{"taskRunID":"task-other","originConversationID":"other"},{"taskRunID":"task-2","originConversationID":"thread:channel:root"}]`, nil
			}
			return `[{"taskRunID":"task-other","originConversationID":"other"}]`, nil
		default:
			return `{}`, nil
		}
	}}
	admin := mattermostScenarioAdmin{remote: remote}
	result := mattermostScenarioResult{
		ChannelID:      "channel",
		ConversationID: conversationID,
		Steps:          []mattermostScenarioStepResult{{PublicURL: "https://example.example.test"}},
	}

	if errorValue := admin.cleanup(context.Background(), result, "probe@example.com"); errorValue != nil {
		t.Fatal(errorValue)
	}
	joinedScripts := strings.Join(remote.scripts, "\n")
	if strings.Count(joinedScripts, "/admin/api/task/delete") != 2 || taskListCount != 2 {
		t.Fatalf("unexpected task cleanup scripts:\n%s", joinedScripts)
	}
	if !scriptContainsJSONDocument(joinedScripts, map[string]any{
		"taskRunIDs": []string{"task-1", "task-2"},
		"reason":     "expensive Mattermost scenario cleanup",
	}) {
		t.Fatalf("task cancellation did not use taskRunIDs:\n%s", joinedScripts)
	}
	if !scriptContainsJSONDocument(joinedScripts, map[string]any{
		"confirm":       "DELETE",
		"userConfirmed": true,
		"requestedBy":   "owner@example.com",
		"requester": map[string]any{
			"personID":       "person-owner",
			"platform":       "mattermost",
			"platformUserID": "user-owner",
		},
	}) {
		t.Fatalf("site deletion did not preserve owner identity:\n%s", joinedScripts)
	}
	if !strings.Contains(joinedScripts, "/admin/api/sites/site-matching") || strings.Contains(joinedScripts, "/admin/api/sites/site-other") || siteListCount != 2 {
		t.Fatalf("unexpected site cleanup scripts:\n%s", joinedScripts)
	}
	if !strings.Contains(joinedScripts, "/admin/api/people?email=probe%40example.com") {
		t.Fatalf("scenario person was not deleted:\n%s", joinedScripts)
	}
}

func TestMattermostScenarioCleanupAggregatesFailuresAndVerifiesRemainingResources(t *testing.T) {
	remote := &fakeMattermostScenarioRemote{runValue: func(script string) (string, error) {
		switch {
		case strings.Contains(script, "/admin/api/task/cancel"):
			return "", errors.New("cancel failed")
		case strings.Contains(script, "/admin/api/task/delete"):
			return "", errors.New("task delete failed")
		case strings.Contains(script, "/admin/api/sites/site-1"):
			return "", errors.New("site delete failed")
		case strings.Contains(script, "/admin/api/sites"):
			return `{"sites":[{"siteID":"site-1","conversationID":"thread:channel:root","ownerIdentity":{"personID":"person-1"}}]}`, nil
		case strings.Contains(script, "/admin/api/people"):
			return "", errors.New("person delete failed")
		case strings.Contains(script, "/admin/api/task"):
			return `[{"taskRunID":"task-1","originConversationID":"thread:channel:root"}]`, nil
		default:
			return `{}`, nil
		}
	}}
	admin := mattermostScenarioAdmin{remote: remote}
	result := mattermostScenarioResult{
		ChannelID:      "channel",
		ConversationID: "thread:channel:root",
		Steps: []mattermostScenarioStepResult{{TaskEvents: []mattermostScenarioTaskEvent{{
			Name: "tool.capability.invoke.requested",
			Body: `{"operation":"site.create"}`,
		}}}},
	}

	errorValue := admin.cleanup(context.Background(), result, "probe@example.com")
	if errorValue == nil {
		t.Fatal("expected cleanup failure")
	}
	for _, fragment := range []string{
		"cancel Mattermost scenario tasks",
		"delete Mattermost scenario task task-1",
		"tasks remain after cleanup: task-1",
		"delete Mattermost scenario site site-1",
		"sites remain after cleanup: site-1",
		"delete Mattermost scenario person",
	} {
		if !strings.Contains(errorValue.Error(), fragment) {
			t.Fatalf("cleanup error is missing %q: %v", fragment, errorValue)
		}
	}
}

func TestMattermostScenarioCleanupSkipsSitesWithoutSiteEvidence(t *testing.T) {
	remote := &fakeMattermostScenarioRemote{runValue: func(string) (string, error) { return `[]`, nil }}
	admin := mattermostScenarioAdmin{remote: remote}
	result := mattermostScenarioResult{ConversationID: "thread:channel:root"}

	if errorValue := admin.cleanup(context.Background(), result, ""); errorValue != nil {
		t.Fatal(errorValue)
	}
	if scripts := strings.Join(remote.scripts, "\n"); strings.Contains(scripts, "/admin/api/sites") {
		t.Fatalf("cleanup without site evidence listed sites:\n%s", scripts)
	}
}

func TestMattermostScenarioCleanupWithoutConversationOnlyDeletesPerson(t *testing.T) {
	remote := &fakeMattermostScenarioRemote{runValue: func(string) (string, error) { return `{}`, nil }}
	admin := mattermostScenarioAdmin{remote: remote}

	if errorValue := admin.cleanup(context.Background(), mattermostScenarioResult{}, "probe@example.com"); errorValue != nil {
		t.Fatal(errorValue)
	}
	joinedScripts := strings.Join(remote.scripts, "\n")
	if strings.Contains(joinedScripts, "/admin/api/task") || strings.Contains(joinedScripts, "/admin/api/sites") {
		t.Fatalf("cleanup without a conversation touched conversation resources:\n%s", joinedScripts)
	}
	if !strings.Contains(joinedScripts, "/admin/api/people?email=probe%40example.com") {
		t.Fatalf("scenario person was not deleted:\n%s", joinedScripts)
	}
}

func scriptContainsJSONDocument(script string, value any) bool {
	document, errorValue := json.Marshal(value)
	if errorValue != nil {
		return false
	}
	return strings.Contains(script, base64.StdEncoding.EncodeToString(document))
}
