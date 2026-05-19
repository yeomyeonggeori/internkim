package admind

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"html"
	"io"
	"mime"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	SiteStatusDraft       = "draft"
	SiteStatusPublishing  = "publishing"
	SiteStatusPublished   = "published"
	SiteStatusUnpublished = "unpublished"
	SiteStatusFailed      = "failed"
	SiteStatusDeleting    = "deleting"
	SiteStatusDeleted     = "deleted"
)

const (
	sitePortStart = 19000
	sitePortEnd   = 19999
)

type SiteRecord struct {
	SiteID              string    `json:"siteID"`
	Slug                string    `json:"slug"`
	Title               string    `json:"title"`
	Owner               string    `json:"owner,omitempty"`
	Status              string    `json:"status"`
	Visibility          string    `json:"visibility"`
	Port                int       `json:"port"`
	CurrentVersionID    string    `json:"currentVersionID,omitempty"`
	PreviousVersionID   string    `json:"previousVersionID,omitempty"`
	PublishedURL        string    `json:"publishedURL,omitempty"`
	TLSStatus           string    `json:"tlsStatus,omitempty"`
	Platform            string    `json:"platform,omitempty"`
	ConversationID      string    `json:"conversationID,omitempty"`
	WorkspacePath       string    `json:"workspacePath,omitempty"`
	SourceWorkspacePath string    `json:"sourceWorkspacePath,omitempty"`
	AppWorkspacePath    string    `json:"appWorkspacePath,omitempty"`
	HostSourcePath      string    `json:"hostSourcePath,omitempty"`
	LastPublishedCommit string    `json:"lastPublishedCommit,omitempty"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
	UnpublishedAt       time.Time `json:"unpublishedAt,omitempty"`
	DeletedAt           time.Time `json:"deletedAt,omitempty"`
	LastError           string    `json:"lastError,omitempty"`
}

type siteCreateRequest struct {
	Slug                string `json:"slug"`
	Title               string `json:"title"`
	Prompt              string `json:"prompt"`
	DesignBrief         string `json:"designBrief"`
	PrototypeScope      string `json:"prototypeScope"`
	SourceWorkspacePath string `json:"sourceWorkspacePath"`
	Owner               string `json:"owner"`
	Visibility          string `json:"visibility"`
	RequestedBy         string `json:"requestedBy"`
	Platform            string `json:"platform"`
	ConversationID      string `json:"conversationID"`
}

type sitePublishRequest struct {
	SiteID                  string `json:"siteID"`
	Slug                    string `json:"slug"`
	Title                   string `json:"title"`
	Owner                   string `json:"owner"`
	Visibility              string `json:"visibility"`
	FrontendSourcePath      string `json:"frontendSourcePath"`
	PocketBaseMigrationPath string `json:"pocketBaseMigrationsPath"`
	PocketBaseHookPath      string `json:"pocketBaseHooksPath"`
	SourceWorkspacePath     string `json:"sourceWorkspacePath"`
	SourceBundleBase64      string `json:"sourceBundleBase64"`
	SourceBundleFormat      string `json:"sourceBundleFormat"`
	RequestedBy             string `json:"requestedBy"`
	Message                 string `json:"message"`
	Platform                string `json:"platform"`
	ConversationID          string `json:"conversationID"`
	PocketBaseHooksApproved bool   `json:"pocketBaseHooksApproved"`
}

type siteLifecycleRequest struct {
	RequestedBy   string `json:"requestedBy"`
	Reason        string `json:"reason"`
	Confirm       string `json:"confirm"`
	UserConfirmed bool   `json:"userConfirmed"`
}

type siteStateDocument struct {
	Sites []*SiteRecord `json:"sites"`
}

type siteWorkspaceMetadata struct {
	SiteID         string `json:"siteID"`
	Slug           string `json:"slug"`
	Title          string `json:"title"`
	PublishedURL   string `json:"publishedURL"`
	Platform       string `json:"platform,omitempty"`
	ConversationID string `json:"conversationID,omitempty"`
	Purpose        string `json:"purpose"`
	Stack          string `json:"stack"`
	DesignDefault  string `json:"designDefault"`
}

var siteSlugPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

func (service *Service) withSiteGateway(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		slug := service.siteSlugFromRequestHost(request.Host)
		if slug == "" {
			next.ServeHTTP(responseWriter, request)
			return
		}
		service.serveSiteHost(responseWriter, request, slug)
	})
}

func (service *Service) serveSiteHost(responseWriter http.ResponseWriter, request *http.Request, slug string) {
	site := service.findSiteBySlug(slug)
	if site == nil {
		http.NotFound(responseWriter, request)
		return
	}
	switch site.Status {
	case SiteStatusPublished:
		service.servePublishedSite(responseWriter, request, site)
	case SiteStatusUnpublished:
		http.Error(responseWriter, "site is unpublished", http.StatusGone)
	case SiteStatusFailed:
		http.Error(responseWriter, "site is unavailable", http.StatusServiceUnavailable)
	case SiteStatusDeleted:
		http.NotFound(responseWriter, request)
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) servePublishedSite(responseWriter http.ResponseWriter, request *http.Request, site *SiteRecord) {
	if isPocketBasePath(request.URL.Path) {
		service.proxySitePocketBase(responseWriter, request, site)
		return
	}
	service.serveSiteFrontend(responseWriter, request, site)
}

func isPocketBasePath(path string) bool {
	return strings.HasPrefix(path, "/api/") || path == "/api" || strings.HasPrefix(path, "/_/")
}

func (service *Service) proxySitePocketBase(responseWriter http.ResponseWriter, request *http.Request, site *SiteRecord) {
	targetURL, errorValue := url.Parse("http://127.0.0.1:" + strconv.Itoa(site.Port))
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	if service.HTTPClient != nil && service.HTTPClient.Transport != nil {
		proxy.Transport = service.HTTPClient.Transport
	}
	proxy.ServeHTTP(responseWriter, request)
}

func (service *Service) serveSiteFrontend(responseWriter http.ResponseWriter, request *http.Request, site *SiteRecord) {
	rootPath := filepath.Join(service.siteVersionPath(site.SiteID, site.CurrentVersionID), "frontend", "dist")
	relativePath, errorValue := cleanSiteFrontendPath(request.URL.Path)
	if errorValue != nil {
		http.NotFound(responseWriter, request)
		return
	}
	filePath := filepath.Join(rootPath, relativePath)
	if information, statError := os.Stat(filePath); statError == nil && !information.IsDir() {
		serveSiteStaticFile(responseWriter, request, filePath)
		return
	}
	http.ServeFile(responseWriter, request, filepath.Join(rootPath, "index.html"))
}

func cleanSiteFrontendPath(path string) (string, error) {
	trimmedPath := strings.TrimPrefix(path, "/")
	if strings.TrimSpace(trimmedPath) == "" {
		return "index.html", nil
	}
	cleanPath := filepath.Clean(trimmedPath)
	if cleanPath == "." || strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
		return "", errors.New("invalid site path")
	}
	for _, part := range strings.Split(cleanPath, string(filepath.Separator)) {
		if strings.HasPrefix(part, ".") {
			return "", errors.New("hidden files are not public")
		}
	}
	return cleanPath, nil
}

func serveSiteStaticFile(responseWriter http.ResponseWriter, request *http.Request, path string) {
	if contentType := mime.TypeByExtension(filepath.Ext(path)); contentType != "" {
		responseWriter.Header().Set("Content-Type", contentType)
	}
	http.ServeFile(responseWriter, request, path)
}

func (service *Service) siteSlugFromRequestHost(host string) string {
	deviceHost := service.deviceHost()
	if deviceHost == "" {
		return ""
	}
	normalizedHost := normalizeHTTPHost(host)
	if normalizedHost == deviceHost || !strings.HasSuffix(normalizedHost, "."+deviceHost) {
		return ""
	}
	slug := strings.TrimSuffix(normalizedHost, "."+deviceHost)
	if strings.Contains(slug, ".") || !isValidSiteSlug(slug) {
		return ""
	}
	return slug
}

func (service *Service) deviceHost() string {
	deviceURL := strings.TrimSpace(readTrimmedFile(service.Configuration.DeviceURLPath))
	if deviceURL != "" {
		parsedURL, errorValue := url.Parse(deviceURL)
		if errorValue == nil && parsedURL.Host != "" {
			return normalizeHTTPHost(parsedURL.Host)
		}
	}
	return normalizeHTTPHost(strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)))
}

func normalizeHTTPHost(host string) string {
	trimmedHost := strings.ToLower(strings.TrimSpace(host))
	hostWithoutPort, _, errorValue := net.SplitHostPort(trimmedHost)
	if errorValue == nil {
		return hostWithoutPort
	}
	return strings.Trim(trimmedHost, "[]")
}

func (service *Service) listSites(responseWriter http.ResponseWriter) {
	service.writeJSON(responseWriter, map[string]any{"sites": service.siteList()})
}

func (service *Service) createSite(responseWriter http.ResponseWriter, request *http.Request) {
	var payload siteCreateRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	site, errorValue := service.createSiteRecord(payload)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	service.writeJSON(responseWriter, site)
}

func (service *Service) handleSite(responseWriter http.ResponseWriter, request *http.Request, path string) {
	siteID, action := splitSitePath(path)
	if siteID == "" {
		http.NotFound(responseWriter, request)
		return
	}
	switch {
	case request.Method == http.MethodGet && action == "":
		service.writeSite(responseWriter, siteID)
	case request.Method == http.MethodPost && action == "publish":
		service.publishSiteFromRequest(responseWriter, request, siteID)
	case request.Method == http.MethodPost && action == "rollback":
		service.rollbackSiteFromRequest(responseWriter, request, siteID)
	case request.Method == http.MethodPost && action == "unpublish":
		service.unpublishSiteFromRequest(responseWriter, request, siteID)
	case request.Method == http.MethodPost && action == "restore":
		service.restoreSiteFromRequest(responseWriter, request, siteID)
	case request.Method == http.MethodGet && action == "logs":
		service.writeSiteLogs(responseWriter, request, siteID)
	case request.Method == http.MethodDelete && action == "":
		service.deleteSiteFromRequest(responseWriter, request, siteID)
	default:
		http.NotFound(responseWriter, request)
	}
}

func splitSitePath(path string) (string, string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return strings.TrimSpace(parts[0]), ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func (service *Service) writeSite(responseWriter http.ResponseWriter, siteID string) {
	site := service.findSiteByID(siteID)
	if site == nil || site.Status == SiteStatusDeleted {
		http.NotFound(responseWriter, nil)
		return
	}
	service.writeJSON(responseWriter, site)
}

func (service *Service) publishSiteFromRequest(responseWriter http.ResponseWriter, request *http.Request, siteID string) {
	payload, errorValue := decodeSitePublishRequest(request.Body)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	payload.SiteID = firstNonEmpty(payload.SiteID, siteID)
	site, errorValue := service.publishSite(request.Context(), payload)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	service.writeJSON(responseWriter, site)
}

func (service *Service) rollbackSiteFromRequest(responseWriter http.ResponseWriter, request *http.Request, siteID string) {
	var payload siteLifecycleRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil && !errors.Is(errorValue, io.EOF) {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	site, errorValue := service.rollbackSite(request.Context(), siteID, payload)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	service.writeJSON(responseWriter, site)
}

func (service *Service) unpublishSiteFromRequest(responseWriter http.ResponseWriter, request *http.Request, siteID string) {
	var payload siteLifecycleRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil && !errors.Is(errorValue, io.EOF) {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	site, errorValue := service.unpublishSite(request.Context(), siteID, payload)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	service.writeJSON(responseWriter, site)
}

func (service *Service) restoreSiteFromRequest(responseWriter http.ResponseWriter, request *http.Request, siteID string) {
	var payload siteLifecycleRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil && !errors.Is(errorValue, io.EOF) {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	site, errorValue := service.restoreSite(request.Context(), siteID, payload)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	service.writeJSON(responseWriter, site)
}

func (service *Service) deleteSiteFromRequest(responseWriter http.ResponseWriter, request *http.Request, siteID string) {
	var payload siteLifecycleRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil && !errors.Is(errorValue, io.EOF) {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	site, errorValue := service.deleteSite(request.Context(), siteID, payload)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	service.writeJSON(responseWriter, site)
}

func (service *Service) writeSiteLogs(responseWriter http.ResponseWriter, request *http.Request, siteID string) {
	site := service.findSiteByID(siteID)
	if site == nil || site.Status == SiteStatusDeleted {
		http.NotFound(responseWriter, request)
		return
	}
	output, errorValue := service.runCommand(request.Context(), "journalctl", "-u", siteServiceName(site.SiteID), "--no-pager", "-n", "120")
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, map[string]string{"logs": string(output)})
}

func decodeSitePublishRequest(reader io.Reader) (sitePublishRequest, error) {
	var payload sitePublishRequest
	if reader == nil {
		return payload, nil
	}
	document, errorValue := io.ReadAll(reader)
	if errorValue != nil {
		return payload, errorValue
	}
	if len(bytes.TrimSpace(document)) == 0 {
		return payload, nil
	}
	return payload, json.Unmarshal(document, &payload)
}

func (service *Service) createSiteRecord(payload siteCreateRequest) (*SiteRecord, error) {
	slug := normalizeSiteSlug(payload.Slug)
	if !isValidSiteSlug(slug) {
		return nil, errors.New("site slug must be a valid DNS label")
	}
	if service.findSiteBySlug(slug) != nil {
		return nil, errors.New("site slug already exists")
	}
	port, errorValue := service.allocateSitePort()
	if errorValue != nil {
		return nil, errorValue
	}
	now := time.Now().UTC()
	siteID := randomHex(12)
	site := &SiteRecord{
		SiteID:              siteID,
		Slug:                slug,
		Title:               firstNonEmpty(strings.TrimSpace(payload.Title), slug),
		Owner:               firstNonEmpty(strings.TrimSpace(payload.Owner), strings.TrimSpace(payload.RequestedBy)),
		Status:              SiteStatusDraft,
		Visibility:          firstNonEmpty(strings.TrimSpace(payload.Visibility), "public"),
		Port:                port,
		PublishedURL:        service.sitePublishedURL(slug),
		TLSStatus:           service.siteTLSStatus(),
		Platform:            strings.TrimSpace(payload.Platform),
		ConversationID:      strings.TrimSpace(payload.ConversationID),
		WorkspacePath:       siteSourceWorkspacePath(siteID, payload.SourceWorkspacePath),
		SourceWorkspacePath: siteSourceWorkspacePath(siteID, payload.SourceWorkspacePath),
		AppWorkspacePath:    filepath.ToSlash(filepath.Join(siteSourceWorkspacePath(siteID, payload.SourceWorkspacePath), "app")),
		HostSourcePath:      service.siteHostWorkspacePath(siteID),
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	return site, service.storeSite(site)
}

func (service *Service) publishSite(ctx context.Context, payload sitePublishRequest) (*SiteRecord, error) {
	site, errorValue := service.siteForPublish(payload)
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := validateWorkspaceOnlyPublish(payload); errorValue != nil {
		return nil, errorValue
	}
	if errorValue := service.updateSiteFromPublishRequest(site, payload); errorValue != nil {
		return nil, errorValue
	}
	if errorValue := service.prepareSiteSourceForPublish(ctx, site, payload); errorValue != nil {
		service.updateSiteStatus(site.SiteID, SiteStatusFailed, errorValue.Error())
		return nil, errorValue
	}
	commitSHA, errorValue := service.commitSiteWorkspace(ctx, site, payload.Message)
	if errorValue != nil {
		service.updateSiteStatus(site.SiteID, SiteStatusFailed, errorValue.Error())
		return nil, errorValue
	}
	versionID := siteVersionID(commitSHA)
	service.updateSiteStatus(site.SiteID, SiteStatusPublishing, "")
	if errorValue := service.prepareSiteVersion(site, versionID, payload); errorValue != nil {
		service.updateSiteStatus(site.SiteID, SiteStatusFailed, errorValue.Error())
		return nil, errorValue
	}
	if errorValue := service.activateSiteVersion(ctx, site, versionID); errorValue != nil {
		service.updateSiteStatus(site.SiteID, SiteStatusFailed, errorValue.Error())
		return nil, errorValue
	}
	now := time.Now().UTC()
	site.PreviousVersionID = site.CurrentVersionID
	site.CurrentVersionID = versionID
	site.LastPublishedCommit = commitSHA
	site.Status = SiteStatusPublished
	site.TLSStatus = service.siteTLSStatus()
	site.UpdatedAt = now
	site.UnpublishedAt = time.Time{}
	site.DeletedAt = time.Time{}
	site.LastError = ""
	return site, service.storeSite(site)
}

func (service *Service) siteForPublish(payload sitePublishRequest) (*SiteRecord, error) {
	if strings.TrimSpace(payload.SiteID) != "" {
		site := service.findSiteByID(payload.SiteID)
		if site == nil || site.Status == SiteStatusDeleted {
			return nil, errors.New("site not found")
		}
		return site, nil
	}
	return nil, errors.New("siteID is required")
}

func validateWorkspaceOnlyPublish(payload sitePublishRequest) error {
	if strings.TrimSpace(payload.FrontendSourcePath) != "" {
		return errors.New("frontendSourcePath is not supported; publish the site workspace by siteID")
	}
	if strings.TrimSpace(payload.PocketBaseMigrationPath) != "" {
		return errors.New("pocketBaseMigrationsPath is not supported; use the site workspace")
	}
	if strings.TrimSpace(payload.PocketBaseHookPath) != "" {
		return errors.New("pocketBaseHooksPath is not supported; use approved workspace hooks")
	}
	return nil
}

func (service *Service) prepareSiteSourceForPublish(ctx context.Context, site *SiteRecord, payload sitePublishRequest) error {
	if strings.TrimSpace(payload.SourceBundleBase64) != "" {
		if errorValue := service.materializeSiteSourceBundle(site, payload); errorValue != nil {
			return errorValue
		}
		return service.initializeSiteGitRepository(ctx, site)
	}
	return errors.New("sourceBundleBase64 is required; publish from the Blueclaw editable source workspace")
}

func (service *Service) updateSiteFromPublishRequest(site *SiteRecord, payload sitePublishRequest) error {
	if payload.Title != "" {
		site.Title = strings.TrimSpace(payload.Title)
	}
	if payload.Visibility != "" {
		site.Visibility = strings.TrimSpace(payload.Visibility)
	}
	if payload.Platform != "" {
		site.Platform = strings.TrimSpace(payload.Platform)
	}
	if payload.ConversationID != "" {
		site.ConversationID = strings.TrimSpace(payload.ConversationID)
	}
	if site.HostSourcePath == "" {
		site.HostSourcePath = service.siteHostWorkspacePath(site.SiteID)
	}
	if site.SourceWorkspacePath == "" {
		site.SourceWorkspacePath = siteSourceWorkspacePath(site.SiteID, "")
	}
	if site.WorkspacePath == "" {
		site.WorkspacePath = site.SourceWorkspacePath
	}
	if site.AppWorkspacePath == "" {
		site.AppWorkspacePath = filepath.ToSlash(filepath.Join(site.SourceWorkspacePath, "app"))
	}
	return service.storeSite(site)
}

func (service *Service) materializeSiteSourceBundle(site *SiteRecord, payload sitePublishRequest) error {
	if strings.TrimSpace(payload.SourceBundleFormat) != "" && strings.TrimSpace(payload.SourceBundleFormat) != "tar.gz" {
		return errors.New("sourceBundleFormat must be tar.gz")
	}
	document, errorValue := base64.StdEncoding.DecodeString(strings.TrimSpace(payload.SourceBundleBase64))
	if errorValue != nil {
		return errorValue
	}
	if errorValue := clearSiteHostSourceWorkspace(site.HostSourcePath); errorValue != nil {
		return errorValue
	}
	return unpackSiteSourceBundle(site.HostSourcePath, document)
}

func clearSiteHostSourceWorkspace(workspacePath string) error {
	if errorValue := os.MkdirAll(workspacePath, 0o700); errorValue != nil {
		return errorValue
	}
	entries, errorValue := os.ReadDir(workspacePath)
	if errorValue != nil {
		return errorValue
	}
	for _, entry := range entries {
		if entry.Name() == ".git" {
			continue
		}
		if errorValue := os.RemoveAll(filepath.Join(workspacePath, entry.Name())); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func unpackSiteSourceBundle(workspacePath string, document []byte) error {
	gzipReader, errorValue := gzip.NewReader(bytes.NewReader(document))
	if errorValue != nil {
		return errorValue
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		header, errorValue := tarReader.Next()
		if errors.Is(errorValue, io.EOF) {
			return nil
		}
		if errorValue != nil {
			return errorValue
		}
		if errorValue := unpackSiteSourceBundleEntry(workspacePath, tarReader, header); errorValue != nil {
			return errorValue
		}
	}
}

func unpackSiteSourceBundleEntry(workspacePath string, reader io.Reader, header *tar.Header) error {
	path, errorValue := safeSiteBundlePath(workspacePath, header.Name)
	if errorValue != nil {
		return errorValue
	}
	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(path, 0o755)
	case tar.TypeReg, tar.TypeRegA:
		if errorValue := os.MkdirAll(filepath.Dir(path), 0o755); errorValue != nil {
			return errorValue
		}
		file, errorValue := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, siteBundleFileMode(header.FileInfo().Mode()))
		if errorValue != nil {
			return errorValue
		}
		_, copyError := io.Copy(file, reader)
		closeError := file.Close()
		if copyError != nil {
			return copyError
		}
		if closeError != nil {
			return closeError
		}
		return os.Chtimes(path, header.ModTime, header.ModTime)
	default:
		return nil
	}
}

func safeSiteBundlePath(workspacePath string, name string) (string, error) {
	if filepath.IsAbs(name) {
		return "", errors.New("source bundle path must be relative")
	}
	cleanName := filepath.Clean(name)
	if cleanName == "." || cleanName == ".." || strings.HasPrefix(cleanName, ".."+string(os.PathSeparator)) {
		return "", errors.New("source bundle path must stay inside workspace")
	}
	path := filepath.Join(workspacePath, cleanName)
	relativePath, errorValue := filepath.Rel(workspacePath, path)
	if errorValue != nil {
		return "", errorValue
	}
	if relativePath == ".." || strings.HasPrefix(relativePath, "../") {
		return "", errors.New("source bundle path must stay inside workspace")
	}
	return path, nil
}

func siteBundleFileMode(mode os.FileMode) os.FileMode {
	if mode&0o111 != 0 {
		return 0o755
	}
	return 0o644
}

func (service *Service) prepareSiteVersion(site *SiteRecord, versionID string, payload sitePublishRequest) error {
	versionPath := service.siteVersionPath(site.SiteID, versionID)
	if errorValue := os.MkdirAll(versionPath, 0o700); errorValue != nil {
		return errorValue
	}
	frontendBuildPath := filepath.Join(site.HostSourcePath, "app", "dist")
	if !isDirectory(frontendBuildPath) {
		return errors.New("site workspace must contain app/dist; build in Blueclaw before publishing")
	}
	if errorValue := ensureSiteFrontendBuildIsFresh(site.HostSourcePath, frontendBuildPath); errorValue != nil {
		return errorValue
	}
	if errorValue := copyDirectory(frontendBuildPath, filepath.Join(versionPath, "frontend", "dist")); errorValue != nil {
		return errorValue
	}
	if errorValue := copyOptionalDirectory(filepath.Join(site.HostSourcePath, "pocketbase", "pb_migrations"), filepath.Join(versionPath, "pb_migrations")); errorValue != nil {
		return errorValue
	}
	if errorValue := service.copyApprovedPocketBaseHooks(site, versionPath, payload); errorValue != nil {
		return errorValue
	}
	return nil
}

func ensureSiteFrontendBuildIsFresh(workspacePath string, frontendBuildPath string) error {
	applicationPath := filepath.Join(workspacePath, "app")
	latestSourceModTime, errorValue := latestFrontendSourceModTime(applicationPath)
	if errorValue != nil {
		return errorValue
	}
	earliestBuildModTime, errorValue := earliestRegularFileModTime(frontendBuildPath)
	if errorValue != nil {
		return errorValue
	}
	if earliestBuildModTime.IsZero() {
		return errors.New("site workspace app/dist must contain build files")
	}
	if latestSourceModTime.After(earliestBuildModTime) {
		return errors.New("site workspace app/dist is stale; run bun run build in app before publishing")
	}
	return nil
}

func latestFrontendSourceModTime(applicationPath string) (time.Time, error) {
	latestModTime := time.Time{}
	errorValue := filepath.Walk(applicationPath, func(path string, information os.FileInfo, walkError error) error {
		if walkError != nil {
			return walkError
		}
		relativePath, errorValue := filepath.Rel(applicationPath, path)
		if errorValue != nil || relativePath == "." {
			return errorValue
		}
		if information.IsDir() && frontendSourcePathIsIgnored(relativePath) {
			return filepath.SkipDir
		}
		if information.Mode().IsRegular() && information.ModTime().After(latestModTime) {
			latestModTime = information.ModTime()
		}
		return nil
	})
	return latestModTime, errorValue
}

func frontendSourcePathIsIgnored(relativePath string) bool {
	for _, component := range strings.Split(filepath.Clean(relativePath), string(os.PathSeparator)) {
		switch component {
		case "dist", "node_modules":
			return true
		}
	}
	return false
}

func earliestRegularFileModTime(rootPath string) (time.Time, error) {
	earliestModTime := time.Time{}
	errorValue := filepath.Walk(rootPath, func(path string, information os.FileInfo, walkError error) error {
		if walkError != nil {
			return walkError
		}
		if !information.Mode().IsRegular() {
			return nil
		}
		if earliestModTime.IsZero() || information.ModTime().Before(earliestModTime) {
			earliestModTime = information.ModTime()
		}
		return nil
	})
	return earliestModTime, errorValue
}

func (service *Service) activateSiteVersion(ctx context.Context, site *SiteRecord, versionID string) error {
	if errorValue := service.ensureSiteRuntimeFiles(site, versionID); errorValue != nil {
		return errorValue
	}
	if errorValue := service.switchCurrentSiteVersion(site.SiteID, versionID); errorValue != nil {
		return errorValue
	}
	_, _ = service.runCommand(ctx, "chown", "-R", "internkim-site:internkim-site", service.sitePath(site.SiteID))
	if _, errorValue := service.runCommand(ctx, "systemctl", "daemon-reload"); errorValue != nil {
		return errorValue
	}
	if _, errorValue := service.runCommand(ctx, "systemctl", "enable", "--now", siteServiceName(site.SiteID)); errorValue != nil {
		return errorValue
	}
	_, _ = service.runCommand(ctx, "systemctl", "restart", siteServiceName(site.SiteID))
	return nil
}

func (service *Service) ensureSiteRuntimeFiles(site *SiteRecord, versionID string) error {
	if errorValue := os.MkdirAll(service.sitePath(site.SiteID), 0o700); errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(service.Configuration.SiteSecretDirectory, 0o700); errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(service.Configuration.SiteSystemdDirectory, 0o755); errorValue != nil {
		return errorValue
	}
	if errorValue := service.ensureSiteEnvironment(site); errorValue != nil {
		return errorValue
	}
	return service.writeSiteSystemdTemplate()
}

func (service *Service) ensureSiteEnvironment(site *SiteRecord) error {
	secretPath := service.siteSecretPath(site.SiteID)
	if errorValue := os.MkdirAll(filepath.Dir(secretPath), 0o700); errorValue != nil {
		return errorValue
	}
	if isRegularFile(secretPath) {
		return nil
	}
	document := "INTERNKIM_SITE_PORT=" + strconv.Itoa(site.Port) + "\nPB_ENCRYPTION_KEY=" + randomHex(16) + "\n"
	return os.WriteFile(secretPath, []byte(document), 0o600)
}

func (service *Service) writeSiteSystemdTemplate() error {
	templatePath := filepath.Join(service.Configuration.SiteSystemdDirectory, "internkim-site@.service")
	document := `[Unit]
Description=InternKim dynamic site %i
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=internkim-site
Group=internkim-site
EnvironmentFile=` + service.Configuration.SiteSecretDirectory + `/%i/environment
WorkingDirectory=` + service.Configuration.SitesRoot + `/%i/current
ExecStart=/usr/local/bin/pocketbase serve --http=127.0.0.1:${INTERNKIM_SITE_PORT} --dir ` + service.Configuration.SitesRoot + `/%i/pb_data --encryptionEnv=PB_ENCRYPTION_KEY
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`
	return os.WriteFile(templatePath, []byte(document), 0o644)
}

func (service *Service) switchCurrentSiteVersion(siteID string, versionID string) error {
	currentPath := filepath.Join(service.sitePath(siteID), "current")
	_ = os.Remove(currentPath)
	return os.Symlink(service.siteVersionPath(siteID, versionID), currentPath)
}

func (service *Service) rollbackSite(ctx context.Context, siteID string, payload siteLifecycleRequest) (*SiteRecord, error) {
	site := service.findSiteByID(siteID)
	if site == nil || site.Status == SiteStatusDeleted {
		return nil, errors.New("site not found")
	}
	if !siteLifecycleAllowed(site, payload) {
		return nil, errors.New("site owner confirmation is required")
	}
	if strings.TrimSpace(site.PreviousVersionID) == "" {
		return nil, errors.New("previous version is not available")
	}
	nextVersionID := site.PreviousVersionID
	site.PreviousVersionID = site.CurrentVersionID
	site.CurrentVersionID = nextVersionID
	if errorValue := service.activateSiteVersion(ctx, site, nextVersionID); errorValue != nil {
		return nil, errorValue
	}
	site.Status = SiteStatusPublished
	site.UpdatedAt = time.Now().UTC()
	site.UnpublishedAt = time.Time{}
	site.LastError = ""
	return site, service.storeSite(site)
}

func (service *Service) unpublishSite(ctx context.Context, siteID string, payload siteLifecycleRequest) (*SiteRecord, error) {
	site := service.findSiteByID(siteID)
	if site == nil || site.Status == SiteStatusDeleted {
		return nil, errors.New("site not found")
	}
	if !siteLifecycleAllowed(site, payload) {
		return nil, errors.New("site owner confirmation is required")
	}
	_, _ = service.runCommand(ctx, "systemctl", "stop", siteServiceName(site.SiteID))
	now := time.Now().UTC()
	site.Status = SiteStatusUnpublished
	site.UpdatedAt = now
	site.UnpublishedAt = now
	site.LastError = strings.TrimSpace(payload.Reason)
	return site, service.storeSite(site)
}

func (service *Service) restoreSite(ctx context.Context, siteID string, payload siteLifecycleRequest) (*SiteRecord, error) {
	site := service.findSiteByID(siteID)
	if site == nil || site.Status == SiteStatusDeleted {
		return nil, errors.New("site not found")
	}
	if !siteLifecycleAllowed(site, payload) {
		return nil, errors.New("site owner confirmation is required")
	}
	if strings.TrimSpace(site.CurrentVersionID) == "" {
		return nil, errors.New("site has no published version")
	}
	if errorValue := service.activateSiteVersion(ctx, site, site.CurrentVersionID); errorValue != nil {
		return nil, errorValue
	}
	site.Status = SiteStatusPublished
	site.UpdatedAt = time.Now().UTC()
	site.UnpublishedAt = time.Time{}
	site.LastError = ""
	return site, service.storeSite(site)
}

func (service *Service) deleteSite(ctx context.Context, siteID string, payload siteLifecycleRequest) (*SiteRecord, error) {
	if payload.Confirm != "DELETE" {
		return nil, errors.New(`confirm must be "DELETE"`)
	}
	if !payload.UserConfirmed {
		return nil, errors.New("user confirmation is required")
	}
	site := service.findSiteByID(siteID)
	if site == nil || site.Status == SiteStatusDeleted {
		return nil, errors.New("site not found")
	}
	if !siteLifecycleAllowed(site, payload) {
		return nil, errors.New("site owner confirmation is required")
	}
	site.Status = SiteStatusDeleting
	site.UpdatedAt = time.Now().UTC()
	if errorValue := service.storeSite(site); errorValue != nil {
		return nil, errorValue
	}
	_, _ = service.runCommand(ctx, "systemctl", "disable", "--now", siteServiceName(site.SiteID))
	_ = os.RemoveAll(service.sitePath(site.SiteID))
	_ = os.RemoveAll(filepath.Dir(service.siteSecretPath(site.SiteID)))
	now := time.Now().UTC()
	site.Status = SiteStatusDeleted
	site.UpdatedAt = now
	site.DeletedAt = now
	return site, service.storeSite(site)
}

func siteLifecycleAllowed(site *SiteRecord, payload siteLifecycleRequest) bool {
	requestedBy := strings.TrimSpace(payload.RequestedBy)
	if strings.TrimSpace(site.Owner) == "" {
		return true
	}
	if requestedBy != "" && strings.EqualFold(requestedBy, site.Owner) {
		return true
	}
	return payload.UserConfirmed && payload.Confirm == "CONFIRM"
}

func (service *Service) prepareSiteWorkspace(ctx context.Context, site *SiteRecord) error {
	if errorValue := os.MkdirAll(filepath.Dir(site.HostSourcePath), 0o777); errorValue != nil {
		return errorValue
	}
	if errorValue := os.Chmod(filepath.Dir(site.HostSourcePath), 0o777); errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(filepath.Join(site.HostSourcePath, "app", "src"), 0o750); errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(filepath.Join(site.HostSourcePath, "pocketbase", "pb_migrations"), 0o750); errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(filepath.Join(site.HostSourcePath, "pocketbase", "pb_hooks"), 0o750); errorValue != nil {
		return errorValue
	}
	if errorValue := service.writeSiteWorkspaceTemplate(site); errorValue != nil {
		return errorValue
	}
	if errorValue := service.initializeSiteGitRepository(ctx, site); errorValue != nil {
		return errorValue
	}
	_, _ = service.runCommand(ctx, "chown", "-R", "blueclaw:blueclaw", site.HostSourcePath)
	return makeSiteWorkspaceCollaborative(site.HostSourcePath)
}

func makeSiteWorkspaceCollaborative(workspacePath string) error {
	return filepath.Walk(workspacePath, func(path string, information os.FileInfo, walkError error) error {
		if walkError != nil {
			return walkError
		}
		mode := os.FileMode(0o666)
		if information.IsDir() || information.Mode()&0o111 != 0 {
			mode = 0o777
		}
		return os.Chmod(path, mode)
	})
}

func (service *Service) writeSiteWorkspaceTemplate(site *SiteRecord) error {
	files := []siteTemplateFile{
		{Path: ".internkim/site.json", Document: service.siteWorkspaceMetadata(site)},
		{Path: "DESIGN.md", Document: siteDesignMD(site)},
		{Path: "app/package.json", Document: sitePackageJSON(site)},
		{Path: "app/index.html", Document: siteIndexHTML(site)},
		{Path: "app/scripts/build.ts", Document: siteBuildTS()},
		{Path: "app/src/content.html", Document: siteContentHTML(site)},
		{Path: "app/src/styles.css", Document: siteStylesCSS()},
		{Path: "app/src/script.js", Document: siteScriptJS()},
		{Path: "app/dist/index.html", Document: siteBuiltIndexHTML(site)},
	}
	for _, file := range files {
		path := filepath.Join(site.HostSourcePath, file.Path)
		if isRegularFile(path) {
			continue
		}
		if errorValue := os.MkdirAll(filepath.Dir(path), 0o750); errorValue != nil {
			return errorValue
		}
		if errorValue := os.WriteFile(path, []byte(file.Document), 0o640); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

type siteTemplateFile struct {
	Path     string
	Document string
}

func (service *Service) siteWorkspaceMetadata(site *SiteRecord) string {
	document, errorValue := json.MarshalIndent(siteWorkspaceMetadata{
		SiteID:         site.SiteID,
		Slug:           site.Slug,
		Title:          site.Title,
		PublishedURL:   site.PublishedURL,
		Platform:       site.Platform,
		ConversationID: site.ConversationID,
		Purpose:        "prototype for idea validation",
		Stack:          "Dependency-free HTML + CSS + JavaScript scaffold with optional PocketBase files",
		DesignDefault:  "starter scaffold only; customize through DESIGN.md before publish",
	}, "", "  ")
	if errorValue != nil {
		return "{}\n"
	}
	return string(document) + "\n"
}

func (service *Service) initializeSiteGitRepository(ctx context.Context, site *SiteRecord) error {
	if isDirectory(filepath.Join(site.HostSourcePath, ".git")) {
		return nil
	}
	if _, errorValue := service.runCommand(ctx, "git", siteGitArguments(site, "init")...); errorValue != nil {
		return errorValue
	}
	_, _ = service.runCommand(ctx, "git", siteGitArguments(site, "config", "user.name", "InternKim")...)
	_, _ = service.runCommand(ctx, "git", siteGitArguments(site, "config", "user.email", "internkim@localhost")...)
	if _, errorValue := service.runCommand(ctx, "git", siteGitArguments(site, "add", ".")...); errorValue != nil {
		return errorValue
	}
	_, _ = service.runCommand(ctx, "git", siteGitArguments(site, "commit", "-m", "Initialize prototype site")...)
	return nil
}

func (service *Service) commitSiteWorkspace(ctx context.Context, site *SiteRecord, message string) (string, error) {
	if errorValue := service.initializeSiteGitRepository(ctx, site); errorValue != nil {
		return "", errorValue
	}
	statusOutput, errorValue := service.runCommand(ctx, "git", siteGitArguments(site, "status", "--porcelain")...)
	if errorValue != nil {
		return "", errorValue
	}
	if strings.TrimSpace(string(statusOutput)) != "" {
		if _, errorValue := service.runCommand(ctx, "git", siteGitArguments(site, "add", ".")...); errorValue != nil {
			return "", errorValue
		}
		if _, errorValue := service.runCommand(ctx, "git", siteGitArguments(site, "commit", "-m", siteCommitMessage(message))...); errorValue != nil {
			return "", errorValue
		}
	}
	commitOutput, errorValue := service.runCommand(ctx, "git", siteGitArguments(site, "rev-parse", "HEAD")...)
	if errorValue != nil {
		return "", errorValue
	}
	return strings.TrimSpace(string(commitOutput)), nil
}

func siteGitArguments(site *SiteRecord, arguments ...string) []string {
	result := []string{"-c", "safe.directory=" + site.HostSourcePath, "-C", site.HostSourcePath}
	return append(result, arguments...)
}

func (service *Service) copyApprovedPocketBaseHooks(site *SiteRecord, versionPath string, payload sitePublishRequest) error {
	hooksPath := filepath.Join(site.HostSourcePath, "pocketbase", "pb_hooks")
	if !directoryHasFiles(hooksPath) {
		return nil
	}
	if !payload.PocketBaseHooksApproved {
		return errors.New("PocketBase hooks require explicit admin approval")
	}
	return copyDirectory(hooksPath, filepath.Join(versionPath, "pb_hooks"))
}

func (service *Service) updateSiteStatus(siteID string, status string, lastError string) {
	site := service.findSiteByID(siteID)
	if site == nil {
		return
	}
	site.Status = status
	site.LastError = lastError
	site.UpdatedAt = time.Now().UTC()
	_ = service.storeSite(site)
}

func (service *Service) siteList() []*SiteRecord {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	sites := []*SiteRecord{}
	for _, site := range service.sites {
		if site.Status != SiteStatusDeleted {
			copiedSite := *site
			sites = append(sites, &copiedSite)
		}
	}
	sort.Slice(sites, func(leftIndex int, rightIndex int) bool {
		return sites[leftIndex].CreatedAt.Before(sites[rightIndex].CreatedAt)
	})
	return sites
}

func (service *Service) findSiteByID(siteID string) *SiteRecord {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	site := service.sites[strings.TrimSpace(siteID)]
	if site == nil {
		return nil
	}
	copiedSite := *site
	return &copiedSite
}

func (service *Service) findSiteBySlug(slug string) *SiteRecord {
	normalizedSlug := normalizeSiteSlug(slug)
	service.mutex.Lock()
	defer service.mutex.Unlock()
	for _, site := range service.sites {
		if site.Slug == normalizedSlug && site.Status != SiteStatusDeleted {
			copiedSite := *site
			return &copiedSite
		}
	}
	return nil
}

func (service *Service) storeSite(site *SiteRecord) error {
	if site == nil {
		return nil
	}
	site.PublishedURL = service.sitePublishedURL(site.Slug)
	site.TLSStatus = service.siteTLSStatus()
	if site.SourceWorkspacePath == "" {
		site.SourceWorkspacePath = siteSourceWorkspacePath(site.SiteID, "")
	}
	if site.WorkspacePath == "" {
		site.WorkspacePath = site.SourceWorkspacePath
	}
	if site.HostSourcePath == "" {
		site.HostSourcePath = service.siteHostWorkspacePath(site.SiteID)
	}
	service.mutex.Lock()
	copiedSite := *site
	service.sites[site.SiteID] = &copiedSite
	service.mutex.Unlock()
	return service.saveSites()
}

func (service *Service) siteTLSStatus() string {
	return strings.TrimSpace(readTrimmedFile("/root/.internkim/env/tls-certificate-status"))
}

func (service *Service) saveSites() error {
	sites := []*SiteRecord{}
	service.mutex.Lock()
	for _, site := range service.sites {
		copiedSite := *site
		sites = append(sites, &copiedSite)
	}
	service.mutex.Unlock()
	sort.Slice(sites, func(leftIndex int, rightIndex int) bool {
		return sites[leftIndex].CreatedAt.Before(sites[rightIndex].CreatedAt)
	})
	document, errorValue := json.MarshalIndent(siteStateDocument{Sites: sites}, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	path := service.siteRegistryPath()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return errorValue
	}
	return os.WriteFile(path, append(document, '\n'), 0o600)
}

func (service *Service) loadSites() {
	document, errorValue := os.ReadFile(service.siteRegistryPath())
	if errorValue != nil {
		return
	}
	var state siteStateDocument
	if errorValue := json.Unmarshal(document, &state); errorValue != nil {
		return
	}
	service.mutex.Lock()
	defer service.mutex.Unlock()
	for _, site := range state.Sites {
		if site != nil && site.SiteID != "" {
			service.sites[site.SiteID] = site
		}
	}
}

func (service *Service) allocateSitePort() (int, error) {
	usedPorts := service.usedSitePorts()
	for port := sitePortStart; port <= sitePortEnd; port++ {
		if usedPorts[port] || !canListenOnSitePort(port) {
			continue
		}
		return port, nil
	}
	return 0, errors.New("no site port is available")
}

func (service *Service) usedSitePorts() map[int]bool {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	usedPorts := map[int]bool{}
	for _, site := range service.sites {
		if site.Status != SiteStatusDeleted {
			usedPorts[site.Port] = true
		}
	}
	return usedPorts
}

func canListenOnSitePort(port int) bool {
	listener, errorValue := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if errorValue != nil {
		return false
	}
	_ = listener.Close()
	return true
}

func (service *Service) siteRegistryPath() string {
	if filepath.Base(service.Configuration.StateDirectory) == "admin" {
		return filepath.Join(filepath.Dir(service.Configuration.StateDirectory), "sites.json")
	}
	return filepath.Join(service.Configuration.StateDirectory, "sites.json")
}

func (service *Service) sitePath(siteID string) string {
	return filepath.Join(service.Configuration.SitesRoot, siteID)
}

func (service *Service) siteVersionPath(siteID string, versionID string) string {
	return filepath.Join(service.sitePath(siteID), "versions", versionID)
}

func (service *Service) siteSecretPath(siteID string) string {
	return filepath.Join(service.Configuration.SiteSecretDirectory, siteID, "environment")
}

func (service *Service) sitePublishedURL(slug string) string {
	deviceHost := service.deviceHost()
	if deviceHost == "" {
		return ""
	}
	return "https://" + normalizeSiteSlug(slug) + "." + deviceHost
}

func siteServiceName(siteID string) string {
	return "internkim-site@" + strings.TrimSpace(siteID) + ".service"
}

func normalizeSiteSlug(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func isValidSiteSlug(value string) bool {
	return siteSlugPattern.MatchString(normalizeSiteSlug(value))
}

func isDirectory(path string) bool {
	information, errorValue := os.Stat(path)
	return errorValue == nil && information.IsDir()
}

func isRegularFile(path string) bool {
	information, errorValue := os.Stat(path)
	return errorValue == nil && !information.IsDir()
}

func copyOptionalDirectory(sourcePath string, targetPath string) error {
	trimmedSourcePath := strings.TrimSpace(sourcePath)
	if trimmedSourcePath == "" {
		return nil
	}
	if !isDirectory(trimmedSourcePath) {
		return nil
	}
	return copyDirectory(trimmedSourcePath, targetPath)
}

func directoryHasFiles(path string) bool {
	entries, errorValue := os.ReadDir(path)
	if errorValue != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			return true
		}
		if directoryHasFiles(filepath.Join(path, entry.Name())) {
			return true
		}
	}
	return false
}

func siteSourceWorkspacePath(siteID string, requestedPath string) string {
	if strings.TrimSpace(requestedPath) != "" {
		return strings.TrimSpace(requestedPath)
	}
	return filepath.ToSlash(filepath.Join("home", "sites", siteID))
}

func (service *Service) siteHostWorkspacePath(siteID string) string {
	return filepath.Join(service.Configuration.BlueclawWorkspacePath, "sites", siteID)
}

func siteVersionID(commitSHA string) string {
	shortCommitSHA := strings.TrimSpace(commitSHA)
	if len(shortCommitSHA) > 12 {
		shortCommitSHA = shortCommitSHA[:12]
	}
	if shortCommitSHA == "" {
		shortCommitSHA = randomHex(4)
	}
	return time.Now().UTC().Format("20060102T150405") + "-" + shortCommitSHA
}

func siteCommitMessage(message string) string {
	cleanMessage := strings.TrimSpace(message)
	if cleanMessage == "" {
		return "Publish prototype site"
	}
	return cleanMessage
}

func sitePackageJSON(site *SiteRecord) string {
	document, errorValue := json.MarshalIndent(map[string]any{
		"scripts": map[string]string{
			"build":   "bun scripts/build.ts",
			"dev":     "bun scripts/build.ts && bun --hot --port 5173 dist/index.html",
			"preview": "bun --port 4173 dist/index.html",
		},
		"dependencies":    map[string]string{},
		"devDependencies": map[string]string{},
		"name":            normalizeSiteSlug(site.Slug),
		"private":         true,
		"type":            "module",
		"version":         "0.0.0",
	}, "", "  ")
	if errorValue != nil {
		return "{}\n"
	}
	return string(document) + "\n"
}

func siteIndexHTML(site *SiteRecord) string {
	title := html.EscapeString(firstNonEmpty(site.Title, site.Slug))
	return "<!doctype html>\n<html lang=\"ko\">\n<head>\n<meta charset=\"UTF-8\" />\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\" />\n<title>" + title + "</title>\n<style>\n__SITE_STYLES__\n</style>\n</head>\n<body>\n__SITE_BODY__\n<script>\n__SITE_SCRIPT__\n</script>\n</body>\n</html>\n"
}

func siteDesignMD(site *SiteRecord) string {
	title := html.EscapeString(firstNonEmpty(site.Title, site.Slug))
	return "# " + title + " DESIGN.md\n\n## Product\n\nEditable scaffold for a website prototype. Replace this file with a request-specific design system before publishing user-facing work.\n\n## Audience\n\nDefine the primary user and what they are trying to accomplish.\n\n## Prototype Scope\n\nDescribe what works in the first publish and what is intentionally deferred.\n\n## Visual Direction\n\nChoose typography, color, spacing, layout density, interaction feel, and responsive behavior for this specific request.\n\n## Screens\n\nList the screens and states included in the prototype.\n\n## Workflows\n\nDescribe the main interaction paths the user can try.\n\n## Data Model\n\nDefine local state, fake data, PocketBase collections, files, or realtime behavior.\n\n## Implemented Now\n\nReplace this scaffold with the implemented feature set before publishing.\n\n## Next Iterations\n\nRecord follow-up work for longer projects.\n\n## Acceptance Criteria\n\nList the checks that must pass before publish.\n"
}

func siteBuiltIndexHTML(site *SiteRecord) string {
	title := html.EscapeString(firstNonEmpty(site.Title, site.Slug))
	return "<!doctype html>\n<html lang=\"ko\">\n<head>\n<meta charset=\"UTF-8\" />\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\" />\n<title>" + title + "</title>\n<style>" + siteStylesCSS() + "</style>\n</head>\n<body>\n" + siteContentHTML(site) + "\n<script>\n" + siteScriptJS() + "\n</script>\n</body>\n</html>\n"
}

func siteBuildTS() string {
	return "import { mkdir, readFile, writeFile } from \"node:fs/promises\";\nimport { join } from \"node:path\";\n\nconst rootPath = process.cwd();\nconst sourcePath = join(rootPath, \"src\");\nconst distPath = join(rootPath, \"dist\");\n\nasync function readOptionalFile(path: string, fallback: string): Promise<string> {\n  try {\n    return await readFile(path, \"utf8\");\n  } catch (error) {\n    if (error && typeof error === \"object\" && \"code\" in error && error.code === \"ENOENT\") {\n      return fallback;\n    }\n    throw error;\n  }\n}\n\nconst template = await readFile(join(rootPath, \"index.html\"), \"utf8\");\nconst body = await readOptionalFile(join(sourcePath, \"content.html\"), \"<main></main>\\n\");\nconst styles = await readOptionalFile(join(sourcePath, \"styles.css\"), \"\");\nconst script = await readOptionalFile(join(sourcePath, \"script.js\"), \"\");\n\nconst document = template\n  .replace(\"__SITE_STYLES__\", styles)\n  .replace(\"__SITE_BODY__\", body)\n  .replace(\"__SITE_SCRIPT__\", script);\n\nawait mkdir(distPath, { recursive: true });\nawait writeFile(join(distPath, \"index.html\"), document, \"utf8\");\n"
}

func siteContentHTML(site *SiteRecord) string {
	title := html.EscapeString(firstNonEmpty(site.Title, site.Slug))
	return "<main class=\"scaffold-shell\">\n  <section class=\"scaffold-panel\">\n    <p class=\"scaffold-label\">Editable scaffold</p>\n    <h1>" + title + "</h1>\n    <p class=\"scaffold-copy\">\n      이 사이트는 아직 사용자 요청에 맞게 제작되기 전의 기본 작업 공간입니다. DESIGN.md를 작성하고 HTML, CSS, JavaScript 소스를 구현한 뒤 빌드해서 배포하세요.\n    </p>\n    <span class=\"scaffold-origin\">Dependency-free site scaffold</span>\n  </section>\n</main>\n"
}

func siteStylesCSS() string {
	return ":root {\n  color: #111827;\n  background: #f8fafc;\n  font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, \"Apple SD Gothic Neo\", \"Segoe UI\", sans-serif;\n}\n\n* {\n  box-sizing: border-box;\n}\n\nbody {\n  margin: 0;\n  min-width: 320px;\n  min-height: 100vh;\n  background: #f8fafc;\n}\n\n.scaffold-shell {\n  display: grid;\n  min-height: 100vh;\n  place-items: center;\n  padding: 24px;\n}\n\n.scaffold-panel {\n  width: min(640px, 100%);\n  border: 1px solid #d1d5db;\n  border-radius: 8px;\n  background: #ffffff;\n  padding: 28px;\n}\n\n.scaffold-label {\n  margin: 0 0 12px;\n  color: #6b7280;\n  font-size: 13px;\n  font-weight: 700;\n}\n\nh1 {\n  margin: 0;\n  font-size: 32px;\n  line-height: 1.15;\n  letter-spacing: 0;\n}\n\n.scaffold-copy {\n  margin: 16px 0 0;\n  color: #4b5563;\n  font-size: 15px;\n  line-height: 1.65;\n}\n\n.scaffold-origin {\n  display: inline-block;\n  margin-top: 18px;\n  color: #6b7280;\n  font-size: 13px;\n}\n\n@media (max-width: 680px) {\n  .scaffold-panel {\n    padding: 22px;\n  }\n\n  h1 {\n    font-size: 28px;\n  }\n}\n"
}

func siteScriptJS() string {
	return "console.info(\"InternKim site prototype loaded\");\n"
}
