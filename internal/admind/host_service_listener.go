package admind

import (
	"net/http"
	"strings"

	blueclawruntime "github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
	"github.com/yeomyeonggeori/internkim/internal/servicetoken"
)

var pathsAnyoneMayAsk = []string{blueclawruntime.BlueclawHealthCheckPath, blueclawruntime.AdmindRosterReadinessPath}

func (service *Service) answeringOnlyTheHostServices(handler http.Handler) http.Handler {
	serviceTokenPath := strings.TrimSpace(service.Configuration.ServiceTokenPath)
	if serviceTokenPath == "" {
		return handler
	}
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if !isAskedOfAnyone(request.URL.Path) && !servicetoken.IsCarriedBy(request, serviceTokenPath) {
			http.Error(responseWriter, "this port answers only the host services that hold the service token", http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(responseWriter, request)
	})
}

func isAskedOfAnyone(path string) bool {
	for _, openPath := range pathsAnyoneMayAsk {
		if path == openPath {
			return true
		}
	}
	return false
}
