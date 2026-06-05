package admind

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (service *Service) serveMailPage(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/mail" {
		http.Redirect(responseWriter, request, "/mail/", http.StatusFound)
		return
	}
	if service.serveMailStaticFile(responseWriter, request) {
		return
	}
	service.serveMailIndex(responseWriter, request)
}

func (service *Service) serveMailStaticFile(responseWriter http.ResponseWriter, request *http.Request) bool {
	relativePath := strings.TrimPrefix(request.URL.Path, "/mail/")
	if relativePath == "" {
		return false
	}
	filePath := filepath.Join(service.Configuration.AdminUIPath, "mail", relativePath)
	fileInformation, errorValue := os.Stat(filePath)
	if errorValue != nil || fileInformation.IsDir() {
		return false
	}
	http.ServeFile(responseWriter, request, filePath)
	return true
}

func (service *Service) serveMailIndex(responseWriter http.ResponseWriter, request *http.Request) {
	mailIndexPath := filepath.Join(service.Configuration.AdminUIPath, "mail", "index.html")
	if fileInformation, errorValue := os.Stat(mailIndexPath); errorValue == nil && !fileInformation.IsDir() {
		http.ServeFile(responseWriter, request, mailIndexPath)
		return
	}
	http.ServeFile(responseWriter, request, filepath.Join(service.Configuration.AdminUIPath, "index.html"))
}
