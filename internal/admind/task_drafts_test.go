package admind

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/llmbackend"
	"github.com/yeomyeonggeori/internkim/pkg/capabilityprotocol"
)

func TestTheDraftModelIsOfferedTheCatalogsTaskAddAndNothingElse(t *testing.T) {
	request := taskDraftChatRequest("견적서 보내기", taskDraftRequester{Name: "이샘플"}, time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC))
	descriptor := capabilityprotocol.MustGeneratedToolDescriptors(taskAddToolName)[0]

	if len(request.Tools) != 1 {
		t.Fatalf("offered %d tools, want task_add alone", len(request.Tools))
	}
	offered := request.Tools[0].Function
	if offered.Name != descriptor.ModelName || offered.Description != descriptor.Description || !bytes.Equal(offered.Parameters, descriptor.InputSchema) {
		t.Fatalf("offered %+v, which is not the catalog's task_add", offered)
	}
	if string(request.ToolChoice) != `"required"` || !request.ParallelToolCalls {
		t.Fatalf("toolChoice=%s parallel=%v; a note may hold several tasks and must produce calls", request.ToolChoice, request.ParallelToolCalls)
	}
}

func TestTheDraftModelIsToldTheCompanysDay(t *testing.T) {
	seoul, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	instruction := taskDraftInstruction(taskDraftRequester{Name: "이샘플"}, time.Date(2026, 11, 19, 23, 30, 0, 0, time.UTC).In(seoul))

	if !strings.Contains(instruction, "2026-11-20 (Friday) in Asia/Seoul") {
		t.Fatalf("instruction = %q", instruction)
	}
}

func TestDraftsAreTheTaskAddCallsInTheOrderTheModelMadeThem(t *testing.T) {
	drafts, errorValue := taskDraftsOfToolCalls([]llmbackend.ChatToolCall{
		taskAddCall(`{"title":"견적서 보내기","endsAt":"2026-11-20"}`),
		taskAddCall(`{"title":"디자인 리뷰","participantPersonHints":["박예시"]}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(drafts.Tasks) != 2 || !strings.Contains(string(drafts.Tasks[0]), "견적서") || !strings.Contains(string(drafts.Tasks[1]), "박예시") {
		t.Fatalf("drafts = %s", drafts.Tasks)
	}
}

func TestNoCallsAreNoDrafts(t *testing.T) {
	drafts, errorValue := taskDraftsOfToolCalls(nil)
	if errorValue != nil || drafts.Tasks == nil || len(drafts.Tasks) != 0 {
		t.Fatalf("drafts = %#v error = %v; an empty list answers a note with no work in it", drafts, errorValue)
	}
}

func TestADraftCallTheModelWasNotOfferedIsRefused(t *testing.T) {
	call := taskAddCall(`{"title":"견적서"}`)
	call.Function.Name = "task_update"

	if _, errorValue := taskDraftsOfToolCalls([]llmbackend.ChatToolCall{call}); errorValue == nil {
		t.Fatal("a call to a tool nobody offered became a draft")
	}
}

func TestDraftArgumentsThatAreNotAnObjectAreRefused(t *testing.T) {
	if _, errorValue := taskDraftsOfToolCalls([]llmbackend.ChatToolCall{taskAddCall(`["견적서"]`)}); errorValue == nil {
		t.Fatal("arguments that are not an object became a draft")
	}
}

func taskAddCall(arguments string) llmbackend.ChatToolCall {
	return llmbackend.ChatToolCall{Type: "function", Function: llmbackend.ChatToolCallFunction{Name: taskAddToolName, Arguments: arguments}}
}
