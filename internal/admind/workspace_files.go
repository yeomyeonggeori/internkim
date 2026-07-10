package admind

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	workspaceAgentRoot         = "/workspace"
	workspacePrivatePeoplePath = "/workspace/private/people/"
	workspaceCirclesPath       = "/workspace/circles/"
	workspacePublicPath        = "/workspace/shared/public"
)

var errWorkspacePathForbidden = errors.New("workspace path is not accessible")

type workspaceRoot struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	AgentPath string `json:"agentPath"`
	Kind      string `json:"kind"`
}

type workspaceEntry struct {
	Name        string `json:"name"`
	AgentPath   string `json:"agentPath"`
	IsDirectory bool   `json:"isDirectory"`
	Size        int64  `json:"size"`
	ModifiedAt  string `json:"modifiedAt"`
}

type workspaceAccess struct {
	personID  string
	circleIDs map[string]bool
	circles   []adminCircleRecord
}

func (service *Service) serveFilesPage(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/files" {
		http.Redirect(responseWriter, request, "/files/", http.StatusFound)
		return
	}
	if service.serveFilesStaticFile(responseWriter, request) {
		return
	}
	service.serveFilesIndex(responseWriter, request)
}

func (service *Service) serveFilesStaticFile(responseWriter http.ResponseWriter, request *http.Request) bool {
	relativePath := strings.TrimPrefix(request.URL.Path, "/files/")
	if relativePath == "" {
		return false
	}
	filePath := filepath.Join(service.Configuration.AdminUIPath, "files", relativePath)
	fileInformation, errorValue := os.Stat(filePath)
	if errorValue != nil || fileInformation.IsDir() {
		return false
	}
	http.ServeFile(responseWriter, request, filePath)
	return true
}

func (service *Service) serveFilesIndex(responseWriter http.ResponseWriter, request *http.Request) {
	filesIndexPath := filepath.Join(service.Configuration.AdminUIPath, "files", "index.html")
	if fileInformation, errorValue := os.Stat(filesIndexPath); errorValue == nil && !fileInformation.IsDir() {
		http.ServeFile(responseWriter, request, filesIndexPath)
		return
	}
	http.ServeFile(responseWriter, request, filepath.Join(service.Configuration.AdminUIPath, "index.html"))
}

func (service *Service) handleFiles(responseWriter http.ResponseWriter, request *http.Request) {
	access, found, errorValue := service.resolveWorkspaceAccess(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(responseWriter, "workspace access required", http.StatusForbidden)
		return
	}
	path := strings.TrimPrefix(request.URL.Path, "/files/api")
	switch {
	case request.Method == http.MethodGet && path == "/roots":
		service.writeWorkspaceRoots(responseWriter, access)
	case request.Method == http.MethodGet && path == "/list":
		service.writeWorkspaceList(responseWriter, request, access)
	case request.Method == http.MethodGet && path == "/download":
		service.downloadWorkspaceFile(responseWriter, request, access)
	case request.Method == http.MethodPost && path == "/upload":
		service.uploadWorkspaceFiles(responseWriter, request, access)
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) resolveWorkspaceAccess(request *http.Request) (workspaceAccess, bool, error) {
	actorEmail := service.webActorEmail(request)
	if actorEmail == "" {
		return workspaceAccess{}, false, nil
	}
	var policyDocument map[string]any
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		return workspaceAccess{}, false, errorValue
	}
	personID := blueclawPersonIDByEmail(policyDocument, actorEmail)
	if personID == "" {
		return workspaceAccess{}, false, nil
	}
	memberCircleIDs := map[string]bool{}
	for _, circleID := range blueclawCirclesByEmail(policyDocument)[strings.ToLower(strings.TrimSpace(actorEmail))] {
		memberCircleIDs[strings.ToLower(strings.TrimSpace(circleID))] = true
	}
	memberCircles := []adminCircleRecord{}
	for _, circle := range blueclawAvailableCircles(policyDocument) {
		if memberCircleIDs[circle.CircleID] {
			memberCircles = append(memberCircles, circle)
		}
	}
	return workspaceAccess{personID: personID, circleIDs: memberCircleIDs, circles: memberCircles}, true, nil
}

func blueclawPersonIDByEmail(policyDocument map[string]any, email string) string {
	people, _ := policyDocument["people"].([]any)
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	for _, value := range people {
		person, isPerson := value.(map[string]any)
		if !isPerson {
			continue
		}
		emails, _ := person["emails"].([]any)
		for _, emailValue := range emails {
			if personEmail, isString := emailValue.(string); isString && strings.EqualFold(strings.TrimSpace(personEmail), normalizedEmail) {
				return strings.TrimSpace(mattermostPolicyString(person["personID"]))
			}
		}
	}
	return ""
}

func (service *Service) writeWorkspaceRoots(responseWriter http.ResponseWriter, access workspaceAccess) {
	roots := []workspaceRoot{{
		ID:        "personal",
		Label:     service.workspaceRootLabels().personal,
		AgentPath: workspacePrivatePeoplePath + access.personID,
		Kind:      "personal",
	}}
	for _, circle := range access.circles {
		roots = append(roots, workspaceRoot{
			ID:        "circle:" + circle.CircleID,
			Label:     circle.DisplayName,
			AgentPath: workspaceCirclesPath + circle.CircleID,
			Kind:      "circle",
		})
	}
	roots = append(roots, workspaceRoot{
		ID:        "public",
		Label:     service.workspaceRootLabels().public,
		AgentPath: workspacePublicPath,
		Kind:      "public",
	})
	service.writeJSON(responseWriter, map[string]any{"roots": roots})
}

// The workspace lives inside the Blueclaw guest image, unreadable from the host,
// so listings and downloads proxy to Blueclaw's read-only workspace endpoints.
// admind still authorizes the web actor against the requested agent path first.
func (service *Service) writeWorkspaceList(responseWriter http.ResponseWriter, request *http.Request, access workspaceAccess) {
	agentPath, _, errorValue := service.resolveWorkspaceHostPath(access, request.URL.Query().Get("path"))
	if errorValue != nil {
		writeWorkspacePathError(responseWriter, errorValue)
		return
	}
	var blueclawResponse struct {
		Entries []workspaceEntry `json:"entries"`
	}
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodGet, "/admin/api/workspace/list?path="+url.QueryEscape(agentPath), nil, &blueclawResponse); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	entries := []workspaceEntry{}
	for _, entry := range blueclawResponse.Entries {
		entry.AgentPath = agentPath + "/" + entry.Name
		entries = append(entries, entry)
	}
	service.writeJSON(responseWriter, map[string]any{"path": agentPath, "entries": entries})
}

func (service *Service) downloadWorkspaceFile(responseWriter http.ResponseWriter, request *http.Request, access workspaceAccess) {
	agentPath, _, errorValue := service.resolveWorkspaceHostPath(access, request.URL.Query().Get("path"))
	if errorValue != nil {
		writeWorkspacePathError(responseWriter, errorValue)
		return
	}
	proxyURL := strings.TrimRight(service.Configuration.BlueclawBaseURL, "/") + "/admin/api/workspace/download?path=" + url.QueryEscape(agentPath)
	proxyRequest, errorValue := http.NewRequestWithContext(request.Context(), http.MethodGet, proxyURL, nil)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	proxyResponse, errorValue := service.httpClient().Do(proxyRequest)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	defer proxyResponse.Body.Close()
	if proxyResponse.StatusCode != http.StatusOK {
		http.NotFound(responseWriter, request)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/octet-stream")
	responseWriter.Header().Set("Content-Disposition", `attachment; filename="`+sanitizeContentDispositionFilename(filepath.Base(agentPath))+`"`)
	_, _ = io.Copy(responseWriter, proxyResponse.Body)
}

func (service *Service) uploadWorkspaceFiles(responseWriter http.ResponseWriter, request *http.Request, access workspaceAccess) {
	_, hostDirectoryPath, errorValue := service.resolveWorkspaceHostPath(access, request.URL.Query().Get("path"))
	if errorValue != nil {
		writeWorkspacePathError(responseWriter, errorValue)
		return
	}
	multipartReader, errorValue := request.MultipartReader()
	if errorValue != nil {
		http.Error(responseWriter, "multipart/form-data body required", http.StatusBadRequest)
		return
	}
	if errorValue := os.MkdirAll(hostDirectoryPath, 0o2770); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	uploadedNames := []string{}
	for {
		part, errorValue := multipartReader.NextPart()
		if errors.Is(errorValue, io.EOF) {
			break
		}
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
			return
		}
		fileName, isFile := workspaceUploadFileName(part)
		if !isFile {
			_ = part.Close()
			continue
		}
		if errorValue := writeWorkspaceUploadPart(hostDirectoryPath, fileName, part); errorValue != nil {
			_ = part.Close()
			http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
			return
		}
		_ = part.Close()
		uploadedNames = append(uploadedNames, fileName)
	}
	service.writeJSON(responseWriter, map[string]any{"uploaded": uploadedNames})
}

func workspaceUploadFileName(part *multipart.Part) (string, bool) {
	return sanitizeWorkspaceUploadFileName(part.FileName())
}

func sanitizeWorkspaceUploadFileName(rawName string) (string, bool) {
	trimmedName := strings.TrimSpace(rawName)
	if trimmedName == "" {
		return "", false
	}
	fileName := filepath.Base(filepath.FromSlash(trimmedName))
	if fileName == "" || fileName == "." || fileName == ".." || fileName == ".blueclaw" {
		return "", false
	}
	return fileName, true
}

func writeWorkspaceUploadPart(hostDirectoryPath string, fileName string, part *multipart.Part) error {
	destinationPath := filepath.Join(hostDirectoryPath, fileName)
	relativePath, errorValue := filepath.Rel(hostDirectoryPath, destinationPath)
	if errorValue != nil || relativePath != fileName {
		return errWorkspacePathForbidden
	}
	destinationFile, errorValue := os.Create(destinationPath)
	if errorValue != nil {
		return errorValue
	}
	defer destinationFile.Close()
	_, errorValue = io.Copy(destinationFile, part)
	return errorValue
}

func (service *Service) resolveWorkspaceHostPath(access workspaceAccess, requestedPath string) (string, string, error) {
	agentPath, errorValue := cleanWorkspaceAgentPath(requestedPath)
	if errorValue != nil {
		return "", "", errorValue
	}
	if !access.allowsAgentPath(agentPath) {
		return "", "", errWorkspacePathForbidden
	}
	workspacePath := service.Configuration.BlueclawWorkspacePath
	hostPath := filepath.Join(workspacePath, strings.TrimPrefix(agentPath, workspaceAgentRoot+"/"))
	hostPath, errorValue = cleanWorkspaceHostPath(workspacePath, hostPath)
	if errorValue != nil {
		return "", "", errorValue
	}
	return agentPath, hostPath, nil
}

func cleanWorkspaceAgentPath(requestedPath string) (string, error) {
	trimmedPath := strings.TrimSpace(requestedPath)
	if trimmedPath == "" {
		return "", errors.New("path is required")
	}
	if !filepath.IsAbs(trimmedPath) {
		return "", errors.New("path must be an absolute /workspace path")
	}
	cleanPath := filepath.ToSlash(filepath.Clean(trimmedPath))
	if cleanPath != workspaceAgentRoot && !strings.HasPrefix(cleanPath, workspaceAgentRoot+"/") {
		return "", errors.New("path must stay under /workspace")
	}
	for _, segment := range strings.Split(cleanPath, "/") {
		if segment == ".blueclaw" {
			return "", errors.New("path cannot access Blueclaw internal files")
		}
	}
	return cleanPath, nil
}

func cleanWorkspaceHostPath(workspacePath string, hostPath string) (string, error) {
	cleanWorkspacePath := filepath.Clean(workspacePath)
	cleanHostPath := filepath.Clean(hostPath)
	if !workspacePathContains(cleanWorkspacePath, cleanHostPath) {
		return "", errWorkspacePathForbidden
	}
	resolvedWorkspacePath, workspaceError := filepath.EvalSymlinks(cleanWorkspacePath)
	resolvedHostPath, hostError := filepath.EvalSymlinks(cleanHostPath)
	if workspaceError == nil && hostError == nil && !workspacePathContains(resolvedWorkspacePath, resolvedHostPath) {
		return "", errWorkspacePathForbidden
	}
	return cleanHostPath, nil
}

func workspacePathContains(root string, candidate string) bool {
	relativePath, errorValue := filepath.Rel(root, candidate)
	if errorValue != nil {
		return false
	}
	return relativePath == "." || (relativePath != ".." && !strings.HasPrefix(relativePath, ".."+string(filepath.Separator)))
}

func (access workspaceAccess) allowsAgentPath(agentPath string) bool {
	if isWithinWorkspaceRoot(agentPath, workspacePublicPath) {
		return true
	}
	if isWithinWorkspaceRoot(agentPath, workspacePrivatePeoplePath+access.personID) {
		return true
	}
	for circleID := range access.circleIDs {
		if isWithinWorkspaceRoot(agentPath, workspaceCirclesPath+circleID) {
			return true
		}
	}
	return false
}

func isWithinWorkspaceRoot(agentPath string, root string) bool {
	return agentPath == root || strings.HasPrefix(agentPath, root+"/")
}

func sanitizeContentDispositionFilename(name string) string {
	replacer := strings.NewReplacer(`"`, "", "\r", "", "\n", "")
	return replacer.Replace(name)
}

type workspaceRootLabelSet struct {
	personal string
	public   string
}

func (service *Service) workspaceRootLabels() workspaceRootLabelSet {
	if strings.HasPrefix(strings.ToLower(service.workspaceLanguage()), "en") {
		return workspaceRootLabelSet{personal: "My workspace", public: "Public"}
	}
	return workspaceRootLabelSet{personal: "개인", public: "공개"}
}

func writeWorkspacePathError(responseWriter http.ResponseWriter, errorValue error) {
	if errors.Is(errorValue, errWorkspacePathForbidden) {
		http.Error(responseWriter, errorValue.Error(), http.StatusForbidden)
		return
	}
	http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
}
