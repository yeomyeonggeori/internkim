package admind

import (
	"net/http"
)

const (
	taskAPIPrefix           = "/task/api"
	retiredTaskAPIPrefix    = "/flow/api"
	quickTaskPath           = taskAPIPrefix + "/tasks/quick"
	taskLabelsPath          = taskAPIPrefix + "/labels"
	taskResolvedActorHeader = "X-INTERNKIM-RESOLVED-ACTOR-EMAIL"
)

func (service *Service) taskAPIHandlers() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		quickTaskPath:  service.createQuickTask,
		taskLabelsPath: service.answerTaskLabels,
	}
}

func (service *Service) handleTaskAPI(responseWriter http.ResponseWriter, request *http.Request) {
	handler, isAnswered := service.taskAPIHandlers()[request.URL.Path]
	if request.Method != http.MethodPost || !isAnswered {
		http.NotFound(responseWriter, request)
		return
	}
	request.Header.Del(taskResolvedActorHeader)
	request = request.WithContext(withTaskActor(request.Context(), service.taskActorEmail(request)))
	if !service.authorizeTaskRequest(request) {
		http.Error(responseWriter, "task access required", http.StatusForbidden)
		return
	}
	handler(responseWriter, request)
}

func writeTaskRequestError(responseWriter http.ResponseWriter, errorValue error) {
	http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
}
