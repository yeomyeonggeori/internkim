package admind

import (
	"strings"
	"testing"
)

func TestTaskNotifyCategoryCoversEveryStatus(t *testing.T) {
	for status, wanted := range map[string]string{
		"waiting_approval": "approval",
		"completed":        "task",
		"failed":           "task",
	} {
		category, notifiable := taskNotifyCategory(status)
		if !notifiable || category != wanted {
			t.Fatalf("%s = %q,%v; want %q,true", status, category, notifiable, wanted)
		}
	}

	for _, status := range []string{"planned", "running", "waiting_user_input", "blocked", "interrupted", "cancelled", ""} {
		if category, notifiable := taskNotifyCategory(status); notifiable {
			t.Fatalf("%s should be silent, got %q", status, category)
		}
	}
}

func TestTaskNotifyContentRelaysWhatWasRecorded(t *testing.T) {
	approval := taskNotifyContent(taskNotifyRun{
		TaskRunID: "run-1",
		Status:    "waiting_approval",
		Prompt:    "예산안 정리해줘",
	}, "approval", "메일을 보내도 될까요?")
	if approval.Title != "승인 대기: 예산안 정리해줘" {
		t.Fatalf("title = %q", approval.Title)
	}
	if approval.Body != "메일을 보내도 될까요?" {
		t.Fatalf("body = %q", approval.Body)
	}
	if approval.OpenPath != "/tasks/run-1" || approval.Tag != "task-run-run-1" {
		t.Fatalf("openPath = %q tag = %q", approval.OpenPath, approval.Tag)
	}

	completed := taskNotifyContent(taskNotifyRun{
		TaskRunID: "run-2",
		Status:    "completed",
		Prompt:    "예산안 정리해줘",
		Result:    "정리해서 문서로 남겼습니다.",
	}, "task", "")
	if completed.Title != "작업 완료: 예산안 정리해줘" || completed.Body != "정리해서 문서로 남겼습니다." {
		t.Fatalf("completed = %+v", completed)
	}

	failed := taskNotifyContent(taskNotifyRun{
		TaskRunID:     "run-3",
		Status:        "failed",
		Prompt:        "예산안 정리해줘",
		FailureReason: "runtime restarted before task completed",
	}, "task", "")
	if failed.Body != "runtime restarted before task completed" {
		t.Fatalf("failed body = %q", failed.Body)
	}
}

func TestTaskNotifyContentFallsBackToThePrompt(t *testing.T) {
	approval := taskNotifyContent(taskNotifyRun{
		TaskRunID: "run-1",
		Status:    "waiting_approval",
		Prompt:    "예산안 정리해줘",
	}, "approval", "")
	if approval.Body != "예산안 정리해줘" {
		t.Fatalf("body = %q", approval.Body)
	}
}

func TestTaskNotifyExcerptCountsRunesAndFlattensLines(t *testing.T) {
	if got := taskNotifyExcerpt("첫 줄\n둘째 줄", 40); got != "첫 줄 둘째 줄" {
		t.Fatalf("got %q", got)
	}
	long := strings.Repeat("가", 50)
	got := taskNotifyExcerpt(long, 40)
	if runes := []rune(got); len(runes) != 41 || string(runes[40]) != "…" {
		t.Fatalf("got %q (%d runes)", got, len(runes))
	}
	if got := taskNotifyExcerpt("   ", 40); got != "" {
		t.Fatalf("blank should stay blank, got %q", got)
	}
}

func TestTaskNotifyAddressByPersonIDSkipsWhoTheDirectoryCannotName(t *testing.T) {
	byPersonID := taskNotifyAddressByPersonID([]adminUserMutation{
		{MemberID: "person-1", Email: "Person1@Example.com"},
		{MemberID: "person-2", Email: ""},
		{MemberID: "", Email: "person3@example.com"},
	})
	if len(byPersonID) != 1 || byPersonID["person-1"] != "person1@example.com" {
		t.Fatalf("byPersonID = %+v", byPersonID)
	}
}

// Somebody added since the company left Mattermost has no account there, and a
// task run they asked for is not a thing to go quiet about.
func TestTaskNotifyAddressByPersonIDReachesSomebodyWithNoMessengerAccount(t *testing.T) {
	byPersonID := taskNotifyAddressByPersonID([]adminUserMutation{{MemberID: "person-1", Email: "new@example.com"}})
	if byPersonID["person-1"] != "new@example.com" {
		t.Fatalf("byPersonID = %+v", byPersonID)
	}
}
