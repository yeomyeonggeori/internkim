package admind

import (
	"net/http"
)

const (
	taskAPIPrefix           = "/task/api"
	retiredTaskAPIPrefix    = "/flow/api"
	quickTaskPath           = taskAPIPrefix + "/tasks/quick"
	taskResolvedActorHeader = "X-INTERNKIM-RESOLVED-ACTOR-EMAIL"
)

// A prompt becomes a task through the model on this machine, and the task it
// infers is written to the record. This is the only task path admind answers.
func (service *Service) handleQuickTask(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || request.URL.Path != quickTaskPath {
		http.NotFound(responseWriter, request)
		return
	}
	request.Header.Del(taskResolvedActorHeader)
	request = request.WithContext(withTaskActor(request.Context(), service.taskActorEmail(request)))
	if !service.authorizeTaskRequest(request) {
		http.Error(responseWriter, "task access required", http.StatusForbidden)
		return
	}
	service.createQuickTask(responseWriter, request)
}

func writeTaskRequestError(responseWriter http.ResponseWriter, errorValue error) {
	http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
}
