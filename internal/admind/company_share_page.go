package admind

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (service *Service) serveCompanySharePage(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("X-Robots-Tag", "noindex, nofollow")
	if request.URL.Path == "/company" {
		http.Redirect(responseWriter, request, "/company/", http.StatusFound)
		return
	}
	relativePath := strings.TrimPrefix(request.URL.Path, "/company/")
	if relativePath != "" {
		filePath := filepath.Join(service.Configuration.AdminUIPath, "company", relativePath)
		if fileInfo, errorValue := os.Stat(filePath); errorValue == nil && !fileInfo.IsDir() {
			http.ServeFile(responseWriter, request, filePath)
			return
		}
	}
	indexPath := filepath.Join(service.Configuration.AdminUIPath, "company", "index.html")
	if fileInfo, errorValue := os.Stat(indexPath); errorValue == nil && !fileInfo.IsDir() {
		http.ServeFile(responseWriter, request, indexPath)
		return
	}
	http.ServeFile(responseWriter, request, filepath.Join(service.Configuration.AdminUIPath, "index.html"))
}
