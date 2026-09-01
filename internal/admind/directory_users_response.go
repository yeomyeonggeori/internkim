package admind

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
)

func (service *Service) writeDirectoryUsersResponse(responseWriter http.ResponseWriter, request *http.Request, response pagesUsersResponse) {
	responseBody, errorValue := service.directoryUsersResponseBody(request.Context(), response)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	_, _ = responseWriter.Write(responseBody)
}

func (service *Service) directoryUsersResponseBody(ctx context.Context, response pagesUsersResponse) ([]byte, error) {
	response.Records = adminUserRecordsWithProfileImages(response.Records)
	responseBody, errorValue := json.Marshal(response)
	if errorValue != nil {
		return nil, errorValue
	}
	enhancedBody, errorValue := service.withBlueclawCircles(ctx, responseBody)
	if errorValue != nil {
		log.Printf("Blueclaw circle merge failed: %v", errorValue)
		enhancedBody = responseBody
	}
	bodyWithCircles := enhancedBody
	enhancedBody, errorValue = service.withOrganizationMetadata(ctx, bodyWithCircles)
	if errorValue != nil {
		log.Printf("Organization metadata merge failed: %v", errorValue)
		return bodyWithCircles, nil
	}
	return enhancedBody, nil
}
