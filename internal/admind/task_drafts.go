package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/llmbackend"
	"github.com/yeomyeonggeori/internkim/pkg/capabilityprotocol"
)

const taskAddToolName = "task_add"

type taskDraftNote struct {
	Note string `json:"note"`
}

type taskDrafts struct {
	Tasks []json.RawMessage `json:"tasks"`
}

type taskDraftRequester struct {
	Email string
	Name  string
}

func (service *Service) answerTaskDrafts(responseWriter http.ResponseWriter, request *http.Request) {
	var draft taskDraftNote
	if errorValue := json.NewDecoder(http.MaxBytesReader(responseWriter, request.Body, recordToolInputCeiling)).Decode(&draft); errorValue != nil || strings.TrimSpace(draft.Note) == "" {
		http.Error(responseWriter, "tasks are drafted from a note that says what the work is", http.StatusBadRequest)
		return
	}
	drafts, errorValue := service.draftTasks(request.Context(), draft.Note, service.taskDraftRequester(request))
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, drafts)
}

func (service *Service) taskDraftRequester(request *http.Request) taskDraftRequester {
	email := service.taskActorEmail(request)
	for _, member := range service.taskMembers(request) {
		if strings.EqualFold(member.Email, email) {
			return taskDraftRequester{Email: email, Name: member.Name}
		}
	}
	return taskDraftRequester{Email: email}
}

func (service *Service) draftTasks(ctx context.Context, note string, requester taskDraftRequester) (taskDrafts, error) {
	requestDocument, errorValue := json.Marshal(taskDraftChatRequest(note, requester, service.companyDateNow(ctx)))
	if errorValue != nil {
		return taskDrafts{}, errorValue
	}
	responseDocument, errorValue := service.callCapabilityLLM(ctx, "/v1/llm/chat", requestDocument)
	if errorValue != nil {
		return taskDrafts{}, errorValue
	}
	var response llmbackend.ChatResponse
	if errorValue := json.Unmarshal(responseDocument, &response); errorValue != nil {
		return taskDrafts{}, fmt.Errorf("the model answered something that is not a chat completion: %w", errorValue)
	}
	return taskDraftsOfToolCalls(response.Message.ToolCalls)
}

func taskDraftChatRequest(note string, requester taskDraftRequester, now time.Time) llmbackend.ChatRequest {
	return llmbackend.ChatRequest{
		ExecutionMode: "remote",
		Context:       llmbackend.RequestContext{RequesterEmail: requester.Email, RequesterName: requester.Name},
		Messages: []llmbackend.ChatMessage{
			{Role: "system", Content: taskDraftInstruction(requester, now)},
			{Role: "user", Content: note},
		},
		Tools:             []llmbackend.ChatTool{taskAddChatTool()},
		ToolChoice:        json.RawMessage(`"required"`),
		ParallelToolCalls: true,
	}
}

func taskDraftInstruction(requester taskDraftRequester, now time.Time) string {
	return strings.Join([]string{
		"The next message is a note " + firstNonEmpty(requester.Name, "a member of the company") + " wrote on the company's task board, listing work to add.",
		"Call task_add once for each separate piece of work the note describes, in the order the note gives them.",
		"Do not split one piece of work into several, and do not add work the note does not describe.",
		"The note is the whole request: nobody reads a reply, so there is nothing to ask.",
		"Today is " + now.Format("2006-01-02 (Monday)") + " in " + now.Location().String() + ".",
	}, "\n")
}

func taskAddChatTool() llmbackend.ChatTool {
	descriptor := capabilityprotocol.MustGeneratedToolDescriptors(taskAddToolName)[0]
	return llmbackend.ChatTool{
		Type: "function",
		Function: llmbackend.ChatFunction{
			Name:        descriptor.ModelName,
			Description: descriptor.Description,
			Parameters:  descriptor.InputSchema,
		},
	}
}

func taskDraftsOfToolCalls(toolCalls []llmbackend.ChatToolCall) (taskDrafts, error) {
	drafts := taskDrafts{Tasks: []json.RawMessage{}}
	for _, toolCall := range toolCalls {
		if toolCall.Function.Name != taskAddToolName {
			return taskDrafts{}, fmt.Errorf("the model called %q, and only %s was offered", toolCall.Function.Name, taskAddToolName)
		}
		var input map[string]json.RawMessage
		if errorValue := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); errorValue != nil {
			return taskDrafts{}, fmt.Errorf("the model sent %s arguments that are not a JSON object: %w", taskAddToolName, errorValue)
		}
		drafts.Tasks = append(drafts.Tasks, json.RawMessage(toolCall.Function.Arguments))
	}
	return drafts, nil
}
