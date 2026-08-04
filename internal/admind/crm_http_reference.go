package admind

import "net/http"

func (service *Service) listCRMPipelinesHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	if _, ok := service.crmHTTPActor(responseWriter, request); !ok {
		return
	}
	activeOnly, errorValue := crmHTTPBooleanQuery(request, "activeOnly", true)
	if errorValue != nil {
		writeCRMHTTPError(responseWriter, http.StatusBadRequest, "invalid_request", "activeOnly must be true or false")
		return
	}
	pipelines, errorValue := service.listCRMPipelines(request.Context(), activeOnly)
	if errorValue != nil {
		writeCRMHTTPReadError(responseWriter, errorValue)
		return
	}
	responses := make([]crmHTTPPipeline, 0, len(pipelines))
	for _, pipeline := range pipelines {
		responses = append(responses, crmHTTPPipeline(pipeline))
	}
	writeCRMHTTPJSON(responseWriter, http.StatusOK, map[string]any{"pipelines": responses})
}

func (service *Service) listCRMPipelineStagesHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	if _, ok := service.crmHTTPActor(responseWriter, request); !ok {
		return
	}
	stages, errorValue := service.listCRMPipelineStages(request.Context(), request.PathValue("pipeline"))
	if errorValue != nil {
		writeCRMHTTPReadError(responseWriter, errorValue)
		return
	}
	responses := make([]crmHTTPPipelineStage, 0, len(stages))
	for _, stage := range stages {
		responses = append(responses, crmHTTPPipelineStage(stage))
	}
	writeCRMHTTPJSON(responseWriter, http.StatusOK, map[string]any{"stages": responses})
}

func (service *Service) listCRMLostReasonsHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	if _, ok := service.crmHTTPActor(responseWriter, request); !ok {
		return
	}
	activeOnly, errorValue := crmHTTPBooleanQuery(request, "activeOnly", true)
	if errorValue != nil {
		writeCRMHTTPError(responseWriter, http.StatusBadRequest, "invalid_request", "activeOnly must be true or false")
		return
	}
	reasons, errorValue := service.listCRMLostReasons(request.Context(), activeOnly)
	if errorValue != nil {
		writeCRMHTTPReadError(responseWriter, errorValue)
		return
	}
	responses := make([]crmHTTPLostReason, 0, len(reasons))
	for _, reason := range reasons {
		responses = append(responses, crmHTTPLostReason(reason))
	}
	writeCRMHTTPJSON(responseWriter, http.StatusOK, map[string]any{"lostReasons": responses})
}
