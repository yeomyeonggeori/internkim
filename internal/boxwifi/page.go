package boxwifi

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"sync"
)

//go:embed captive
var captiveFiles embed.FS

var pageFiles = mustSubdirectory(captiveFiles, "captive")

func mustSubdirectory(files fs.FS, directory string) fs.FS {
	subdirectory, errorValue := fs.Sub(files, directory)
	if errorValue != nil {
		panic("boxwifi: the embedded setup page has no " + directory + " directory: " + errorValue.Error())
	}
	return subdirectory
}

type networkListing struct {
	Networks      []listedNetwork `json:"networks"`
	HasJoinFailed bool            `json:"hasJoinFailed"`
}

type listedNetwork struct {
	SSID          string `json:"ssid"`
	IsSecured     bool   `json:"isSecured"`
	SignalPercent int    `json:"signalPercent"`
}

type page struct {
	listing   networkListing
	submitted chan<- submission
	files     http.Handler

	mutex        sync.Mutex
	hasSubmitted bool
}

func newPage(networks []Network, hasJoinFailed bool, submitted chan<- submission) http.Handler {
	listed := make([]listedNetwork, 0, len(networks))
	for _, network := range networks {
		listed = append(listed, listedNetwork{SSID: network.SSID, IsSecured: network.IsSecured, SignalPercent: network.SignalPercent})
	}
	return &page{
		listing:   networkListing{Networks: listed, HasJoinFailed: hasJoinFailed},
		submitted: submitted,
		files:     http.FileServerFS(pageFiles),
	}
}

func (handler *page) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Cache-Control", "no-store")
	if request.URL.Path == "/networks" && request.Method == http.MethodGet {
		handler.networks(responseWriter)
		return
	}
	if request.URL.Path == "/join" && request.Method == http.MethodPost {
		handler.join(responseWriter, request)
		return
	}
	if request.Method == http.MethodGet && isPageFile(request.URL.Path) {
		handler.files.ServeHTTP(responseWriter, request)
		return
	}
	http.Redirect(responseWriter, request, "/", http.StatusFound)
}

func isPageFile(path string) bool {
	if path == "/" {
		return true
	}
	name := strings.TrimPrefix(path, "/")
	if name == "index.html" {
		return false
	}
	info, errorValue := fs.Stat(pageFiles, name)
	return errorValue == nil && !info.IsDir()
}

func (handler *page) networks(responseWriter http.ResponseWriter) {
	responseWriter.Header().Set("Content-Type", "application/json")
	if errorValue := json.NewEncoder(responseWriter).Encode(handler.listing); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
	}
}

func (handler *page) join(responseWriter http.ResponseWriter, request *http.Request) {
	if errorValue := request.ParseForm(); errorValue != nil {
		http.Error(responseWriter, "the join form could not be read", http.StatusBadRequest)
		return
	}
	ssid := strings.TrimSpace(request.PostForm.Get("customSSID"))
	if ssid == "" {
		ssid = request.PostForm.Get("ssid")
	}
	if strings.TrimSpace(ssid) == "" {
		http.Error(responseWriter, "choose or enter a network", http.StatusBadRequest)
		return
	}
	password := request.PostForm.Get("password")

	if !handler.claimSubmission() {
		http.Error(responseWriter, "a network was already submitted", http.StatusConflict)
		return
	}
	handler.submitted <- submission{ssid: ssid, password: password}
	responseWriter.WriteHeader(http.StatusOK)
}

func (handler *page) claimSubmission() bool {
	handler.mutex.Lock()
	defer handler.mutex.Unlock()
	if handler.hasSubmitted {
		return false
	}
	handler.hasSubmitted = true
	return true
}
