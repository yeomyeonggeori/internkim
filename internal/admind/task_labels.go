package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"gitlab.com/eastriver/internkim/internal/llmbackend"
)

// Jev's Choice question accepts at most 32 criteria.
const largestTaskLabelChoice = 32

const noRegisteredTaskType = "none"

type taskLabelDraft struct {
	Title string `json:"title"`
	Note  string `json:"note"`
}

type taskLabels struct {
	Business string `json:"business"`
	Type     string `json:"type"`
	Size     string `json:"size"`
}

type taskLabelDecision struct {
	Answers map[string]llmbackend.DecisionAnswer `json:"answers"`
}

const taskLabelsPublicPath = "/task/labels"

func (service *Service) answerTaskLabels(responseWriter http.ResponseWriter, request *http.Request, actor publicToolGatewayActor) {
	if actorPermissionRank(actor) < publicAPIPermissionRank(publicAPIPermissionWrite) {
		http.Error(responseWriter, "a task's labels are decided while adding it, which takes write permission", http.StatusForbidden)
		return
	}
	var draft taskLabelDraft
	if errorValue := json.NewDecoder(http.MaxBytesReader(responseWriter, request.Body, recordToolInputCeiling)).Decode(&draft); errorValue != nil {
		http.Error(responseWriter, "a task's labels are decided from its title and note", http.StatusBadRequest)
		return
	}
	ctx := withTaskActor(request.Context(), actor.Actor.Email)
	definitions, errorValue := service.readTaskDefinitions(ctx)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	labels, errorValue := service.decideTaskLabels(ctx, draft, definitions)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, labels)
}

func (service *Service) decideTaskLabels(ctx context.Context, draft taskLabelDraft, definitions taskDefinitions) (taskLabels, error) {
	questions := taskLabelQuestions(definitions)
	state, errorValue := json.Marshal(map[string]taskLabelDraft{"task": draft})
	if errorValue != nil {
		return taskLabels{}, errorValue
	}
	requestDocument, errorValue := json.Marshal(map[string]any{"state": json.RawMessage(state), "questions": questions})
	if errorValue != nil {
		return taskLabels{}, errorValue
	}
	responseDocument, errorValue := service.callCapabilityLLM(ctx, "/v1/llm/decide", requestDocument)
	if errorValue != nil {
		return taskLabels{}, errorValue
	}
	var decision taskLabelDecision
	if errorValue := json.Unmarshal(responseDocument, &decision); errorValue != nil {
		return taskLabels{}, fmt.Errorf("the decision model answered something that is not a decision: %w", errorValue)
	}
	return taskLabelsOfAnswers(decision.Answers, questions), nil
}

func (service *Service) decideTaskLabelsAside(ctx context.Context, draft taskLabelDraft, definitions taskDefinitions) <-chan taskLabels {
	decided := make(chan taskLabels, 1)
	go func() {
		labels, errorValue := service.decideTaskLabels(ctx, draft, definitions)
		if errorValue != nil {
			log.Printf("task.labels_undecided: error=%v", errorValue)
		}
		decided <- labels
	}()
	return decided
}

func taskLabelQuestions(definitions taskDefinitions) map[string]llmbackend.DecisionQuestion {
	questions := map[string]llmbackend.DecisionQuestion{
		"size": llmbackend.ChoiceQuestion(
			"How much effort this work takes, judged against the company's size rubric. A deadline span is not effort.",
			taskSizeCriteria(definitions.Sizes),
		),
	}
	if businesses := labelCriteria(definitions.Categories); len(businesses) > 0 && len(businesses) <= largestTaskLabelChoice {
		questions["business"] = llmbackend.ChoiceQuestion("Which of the company's businesses this work is for.", businesses)
	}
	if types := labelCriteria(definitions.Types); len(types) > 0 && len(types) < largestTaskLabelChoice {
		types[noRegisteredTaskType] = "None of the company's registered types fits this work."
		questions["type"] = llmbackend.ChoiceQuestion("Which of the company's registered types this work is.", types)
	}
	return questions
}

func taskSizeCriteria(sizes []taskSizeDefinition) map[string]string {
	criteria := map[string]string{}
	for _, size := range sizes {
		criteria[size.Name] = fmt.Sprintf("at most %d hours; development: %s; other work: %s; %s", size.MaxHours, size.DevelopmentExample, size.OtherExample, size.Note)
	}
	return criteria
}

func labelCriteria(labels []string) map[string]string {
	criteria := map[string]string{}
	for _, label := range labels {
		if trimmed := strings.TrimSpace(label); trimmed != "" {
			criteria[trimmed] = trimmed
		}
	}
	return criteria
}

func taskLabelsOfAnswers(answers map[string]llmbackend.DecisionAnswer, questions map[string]llmbackend.DecisionQuestion) taskLabels {
	taskType := chosenOption(answers, questions, "type")
	if taskType == noRegisteredTaskType {
		taskType = ""
	}
	return taskLabels{
		Business: chosenOption(answers, questions, "business"),
		Type:     taskType,
		Size:     chosenOption(answers, questions, "size"),
	}
}

func chosenOption(answers map[string]llmbackend.DecisionAnswer, questions map[string]llmbackend.DecisionQuestion, name string) string {
	question, asked := questions[name]
	if !asked {
		return ""
	}
	var options map[string]string
	if json.Unmarshal(question.Criteria, &options) != nil {
		return ""
	}
	choice := answers[name].Choice
	if _, offered := options[choice]; !offered {
		log.Printf("task.labels_choice_unoffered: question=%s choice=%q", name, choice)
		return ""
	}
	return choice
}
