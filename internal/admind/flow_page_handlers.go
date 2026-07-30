package admind

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (service *Service) serveFlowPage(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/flow" {
		http.Redirect(responseWriter, request, "/flow/", http.StatusFound)
		return
	}
	if service.serveFlowStaticFile(responseWriter, request) {
		return
	}
	service.serveFlowIndex(responseWriter, request)
}

func (service *Service) serveFlowStaticFile(responseWriter http.ResponseWriter, request *http.Request) bool {
	relativePath := strings.TrimPrefix(request.URL.Path, "/flow/")
	if relativePath == "" {
		return false
	}
	filePath := filepath.Join(service.Configuration.AdminUIPath, "flow", relativePath)
	fileInfo, errorValue := os.Stat(filePath)
	if errorValue != nil || fileInfo.IsDir() {
		return false
	}
	http.ServeFile(responseWriter, request, filePath)
	return true
}

func (service *Service) serveFlowIndex(responseWriter http.ResponseWriter, request *http.Request) {
	flowIndexPath := filepath.Join(service.Configuration.AdminUIPath, "flow", "index.html")
	if fileInfo, errorValue := os.Stat(flowIndexPath); errorValue == nil && !fileInfo.IsDir() {
		http.ServeFile(responseWriter, request, flowIndexPath)
		return
	}
	http.ServeFile(responseWriter, request, filepath.Join(service.Configuration.AdminUIPath, "index.html"))
}

// serveBoardSection serves a prerendered board-UI section (its own index.html,
// falling back to the SPA shell) so a refresh on a client-side route is handled
// by the app instead of falling through to the Mattermost proxy on "/".
func (service *Service) serveBoardSection(section string) http.HandlerFunc {
	prefix := "/" + section
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path == prefix {
			http.Redirect(responseWriter, request, prefix+"/", http.StatusFound)
			return
		}
		relativePath := strings.TrimPrefix(request.URL.Path, prefix+"/")
		if relativePath != "" {
			filePath := filepath.Join(service.Configuration.AdminUIPath, section, relativePath)
			if fileInfo, errorValue := os.Stat(filePath); errorValue == nil && !fileInfo.IsDir() {
				http.ServeFile(responseWriter, request, filePath)
				return
			}
		}
		sectionIndexPath := filepath.Join(service.Configuration.AdminUIPath, section, "index.html")
		if fileInfo, errorValue := os.Stat(sectionIndexPath); errorValue == nil && !fileInfo.IsDir() {
			http.ServeFile(responseWriter, request, sectionIndexPath)
			return
		}
		http.ServeFile(responseWriter, request, filepath.Join(service.Configuration.AdminUIPath, "index.html"))
	}
}
