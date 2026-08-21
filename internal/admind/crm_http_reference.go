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

type crmHTTPDefinitionStage struct {
	ID       string `json:"id"`
	Outcome  string `json:"outcome"`
	Position int    `json:"position"`
}

func (service *Service) listCRMDefinitionsHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	if _, ok := service.crmHTTPActor(responseWriter, request); !ok {
		return
	}
	definitions, errorValue := service.readFlowDefinitions(request.Context())
	if errorValue != nil {
		writeCRMHTTPReadError(responseWriter, errorValue)
		return
	}
	businesses := definitions.Categories
	if businesses == nil {
		businesses = []string{}
	}
	stages, errorValue := service.listCRMPipelineStageDefinitions(request.Context())
	if errorValue != nil {
		writeCRMHTTPReadError(responseWriter, errorValue)
		return
	}
	stageResponses := make([]crmHTTPDefinitionStage, 0, len(stages))
	for _, stage := range stages {
		stageResponses = append(stageResponses, crmHTTPDefinitionStage{
			ID:       stage.Stage,
			Outcome:  stage.Outcome,
			Position: stage.Position,
		})
	}
	writeCRMHTTPJSON(responseWriter, http.StatusOK, map[string]any{
		"definitions": map[string]any{"businesses": businesses, "stages": stageResponses},
	})
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
