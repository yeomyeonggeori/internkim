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

func (service *Service) listCRMDefinitionsHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	if _, ok := service.crmHTTPActor(responseWriter, request); !ok {
		return
	}
	definitions, errorValue := service.readTaskDefinitions(request.Context())
	if errorValue != nil {
		writeCRMHTTPReadError(responseWriter, errorValue)
		return
	}
	businesses := definitions.Categories
	if businesses == nil {
		businesses = []string{}
	}
	writeCRMHTTPJSON(responseWriter, http.StatusOK, map[string]any{
		"definitions": map[string]any{"businesses": businesses},
	})
}
