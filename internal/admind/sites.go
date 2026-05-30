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
	SiteID              string             `json:"siteID"`
	Slug                string             `json:"slug"`
	Title               string             `json:"title"`
	Owner               string             `json:"owner,omitempty"`
	OwnerIdentity       siteIdentity       `json:"ownerIdentity,omitempty"`
	CreatedBy           siteIdentity       `json:"createdBy,omitempty"`
	Collaborators       []siteCollaborator `json:"collaborators,omitempty"`
	Description         string             `json:"description,omitempty"`
	Idea                string             `json:"idea,omitempty"`
	OriginalPrompt      string             `json:"originalPrompt,omitempty"`
	Purpose             string             `json:"purpose,omitempty"`
	Audience            string             `json:"audience,omitempty"`
	Archetype           string             `json:"archetype,omitempty"`
	DomainKeywords      []string           `json:"domainKeywords,omitempty"`
	Status              string             `json:"status"`
	Visibility          string             `json:"visibility"`
	Port                int                `json:"port"`
	CurrentVersionID    string             `json:"currentVersionID,omitempty"`
	PreviousVersionID   string             `json:"previousVersionID,omitempty"`
	PublishedURL        string             `json:"publishedURL,omitempty"`
	TLSStatus           string             `json:"tlsStatus,omitempty"`
	Platform            string             `json:"platform,omitempty"`
	ConversationID      string             `json:"conversationID,omitempty"`
	WorkspacePath       string             `json:"workspacePath,omitempty"`
	SourceWorkspacePath string             `json:"sourceWorkspacePath,omitempty"`
	AppWorkspacePath    string             `json:"appWorkspacePath,omitempty"`
	HostSourcePath      string             `json:"hostSourcePath,omitempty"`
	LastPublishedCommit string             `json:"lastPublishedCommit,omitempty"`
	RevisionCount       int                `json:"revisionCount"`
	CreatedAt           time.Time          `json:"createdAt"`
	UpdatedAt           time.Time          `json:"updatedAt"`
	UnpublishedAt       time.Time          `json:"unpublishedAt,omitempty"`
	DeletedAt           time.Time          `json:"deletedAt,omitempty"`
	LastError           string             `json:"lastError,omitempty"`
}

type siteCreateRequest struct {
	Slug                string             `json:"slug"`
	Title               string             `json:"title"`
	Prompt              string             `json:"prompt"`
	DesignBrief         string             `json:"designBrief"`
	PrototypeScope      string             `json:"prototypeScope"`
	Description         string             `json:"description"`
	Idea                string             `json:"idea"`
	Purpose             string             `json:"purpose"`
	Audience            string             `json:"audience"`
	Archetype           string             `json:"archetype"`
	DomainKeywords      []string           `json:"domainKeywords"`
	SourceWorkspacePath string             `json:"sourceWorkspacePath"`
	Owner               string             `json:"owner"`
	OwnerIdentity       siteIdentity       `json:"ownerIdentity"`
	CreatedBy           siteIdentity       `json:"createdBy"`
	Collaborators       []siteCollaborator `json:"collaborators"`
	Visibility          string             `json:"visibility"`
	RequestedBy         string             `json:"requestedBy"`
	Requester           siteIdentity       `json:"requester"`
	Platform            string             `json:"platform"`
	ConversationID      string             `json:"conversationID"`
}

type sitePublishRequest struct {
	SiteID                  string       `json:"siteID"`
	Slug                    string       `json:"slug"`
	Title                   string       `json:"title"`
	Owner                   string       `json:"owner"`
	Visibility              string       `json:"visibility"`
	Description             string       `json:"description"`
	Idea                    string       `json:"idea"`
	Purpose                 string       `json:"purpose"`
	Audience                string       `json:"audience"`
	Archetype               string       `json:"archetype"`
	DomainKeywords          []string     `json:"domainKeywords"`
	FrontendSourcePath      string       `json:"frontendSourcePath"`
	PocketBaseMigrationPath string       `json:"pocketBaseMigrationsPath"`
	PocketBaseHookPath      string       `json:"pocketBaseHooksPath"`
	SourceWorkspacePath     string       `json:"sourceWorkspacePath"`
	SourceBundleBase64      string       `json:"sourceBundleBase64"`
	SourceBundleFormat      string       `json:"sourceBundleFormat"`
	RequestedBy             string       `json:"requestedBy"`
	Requester               siteIdentity `json:"requester"`
	Message                 string       `json:"message"`
	Platform                string       `json:"platform"`
	ConversationID          string       `json:"conversationID"`
	PocketBaseHooksApproved bool         `json:"pocketBaseHooksApproved"`
}

type siteLifecycleRequest struct {
	RequestedBy   string       `json:"requestedBy"`
	Requester     siteIdentity `json:"requester"`
	Reason        string       `json:"reason"`
	Revision      string       `json:"revision"`
	Confirm       string       `json:"confirm"`
	UserConfirmed bool         `json:"userConfirmed"`
}

type siteDiffRequest struct {
	FromRevision string `json:"fromRevision"`
	ToRevision   string `json:"toRevision"`
}

type siteStateDocument struct {
	Sites []*SiteRecord `json:"sites"`
}

type siteWorkspaceMetadata struct {
	SiteID         string             `json:"siteID"`
	Slug           string             `json:"slug"`
	Title          string             `json:"title"`
	PublishedURL   string             `json:"publishedURL"`
	Platform       string             `json:"platform,omitempty"`
	ConversationID string             `json:"conversationID,omitempty"`
	Description    string             `json:"description,omitempty"`
	Idea           string             `json:"idea,omitempty"`
	OriginalPrompt string             `json:"originalPrompt,omitempty"`
	Purpose        string             `json:"purpose"`
	Audience       string             `json:"audience,omitempty"`
	Archetype      string             `json:"archetype,omitempty"`
	DomainKeywords []string           `json:"domainKeywords,omitempty"`
	CreatedBy      siteIdentity       `json:"createdBy,omitempty"`
	Owner          siteIdentity       `json:"owner,omitempty"`
	Collaborators  []siteCollaborator `json:"collaborators,omitempty"`
	Stack          string             `json:"stack"`
	DesignDefault  string             `json:"designDefault"`
}

type siteIdentity struct {
	PersonID       string `json:"personID,omitempty"`
	Platform       string `json:"platform,omitempty"`
	PlatformUserID string `json:"platformUserID,omitempty"`
	DisplayName    string `json:"displayName,omitempty"`
}

type siteCollaborator struct {
	PersonID       string    `json:"personID,omitempty"`
	PlatformUserID string    `json:"platformUserID,omitempty"`
	Role           string    `json:"role"`
	GrantedBy      string    `json:"grantedBy,omitempty"`
	GrantedAt      time.Time `json:"grantedAt,omitempty"`
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
	case request.Method == http.MethodGet && action == "history":
		service.writeSiteHistory(responseWriter, request, siteID)
	case request.Method == http.MethodGet && action == "diff":
		service.writeSiteDiff(responseWriter, request, siteID)
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
	service.writeJSON(responseWriter, service.siteWithRevisionMetadata(site))
}

func (service *Service) writeSiteHistory(responseWriter http.ResponseWriter, request *http.Request, siteID string) {
	site := service.findSiteByID(siteID)
	if site == nil || site.Status == SiteStatusDeleted {
		http.NotFound(responseWriter, request)
		return
	}
	history, errorValue := service.siteGitHistory(request.Context(), site)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, history)
}

func (service *Service) writeSiteDiff(responseWriter http.ResponseWriter, request *http.Request, siteID string) {
	site := service.findSiteByID(siteID)
	if site == nil || site.Status == SiteStatusDeleted {
		http.NotFound(responseWriter, request)
		return
	}
	diffRequest := siteDiffRequest{
		FromRevision: request.URL.Query().Get("fromRevision"),
		ToRevision:   request.URL.Query().Get("toRevision"),
	}
	diff, errorValue := service.siteGitDiff(request.Context(), site, diffRequest)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, diff)
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
	createdBy := siteCreatorIdentity(payload)
	ownerIdentity := siteOwnerIdentity(payload, createdBy)
	site := &SiteRecord{
		SiteID:              siteID,
		Slug:                slug,
		Title:               firstNonEmpty(strings.TrimSpace(payload.Title), slug),
		Owner:               firstNonEmpty(strings.TrimSpace(payload.Owner), strings.TrimSpace(payload.RequestedBy)),
		OwnerIdentity:       ownerIdentity,
		CreatedBy:           createdBy,
		Collaborators:       normalizeSiteCollaborators(payload.Collaborators),
		Description:         siteDescriptionFromCreateRequest(payload),
		Idea:                firstNonEmpty(strings.TrimSpace(payload.Idea), strings.TrimSpace(payload.Prompt)),
		OriginalPrompt:      strings.TrimSpace(payload.Prompt),
		Purpose:             firstNonEmpty(strings.TrimSpace(payload.Purpose), inferSitePurpose(payload)),
		Audience:            strings.TrimSpace(payload.Audience),
		Archetype:           firstNonEmpty(strings.TrimSpace(payload.Archetype), inferSiteArchetype(payload)),
		DomainKeywords:      normalizeSiteKeywords(payload.DomainKeywords, payload),
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
	if !siteModificationAllowed(site, payload.RequestedBy, payload.Requester, false) {
		return nil, errors.New("site editor permission is required")
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
	site.RevisionCount = service.siteRevisionCount(site)
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

func siteCreatorIdentity(payload siteCreateRequest) siteIdentity {
	for _, identity := range []siteIdentity{payload.CreatedBy, payload.Requester, payload.OwnerIdentity} {
		if !siteIdentityEmpty(identity) {
			return normalizeSiteIdentity(identity)
		}
	}
	return siteIdentity{
		Platform:       strings.TrimSpace(payload.Platform),
		PlatformUserID: strings.TrimSpace(payload.RequestedBy),
		DisplayName:    strings.TrimSpace(payload.RequestedBy),
	}
}

func siteOwnerIdentity(payload siteCreateRequest, createdBy siteIdentity) siteIdentity {
	if !siteIdentityEmpty(payload.OwnerIdentity) {
		return normalizeSiteIdentity(payload.OwnerIdentity)
	}
	return createdBy
}

func siteDescriptionFromCreateRequest(payload siteCreateRequest) string {
	return firstNonEmpty(
		strings.TrimSpace(payload.Description),
		strings.TrimSpace(payload.PrototypeScope),
		strings.TrimSpace(payload.DesignBrief),
		firstSentence(payload.Prompt),
		firstNonEmpty(strings.TrimSpace(payload.Title), normalizeSiteSlug(payload.Slug)),
	)
}

func inferSitePurpose(payload siteCreateRequest) string {
	text := strings.ToLower(strings.Join([]string{payload.Title, payload.Prompt, payload.DesignBrief, payload.PrototypeScope}, " "))
	switch {
	case strings.Contains(text, "portfolio") || strings.Contains(text, "포트폴리오"):
		return "portfolio"
	case strings.Contains(text, "booking") || strings.Contains(text, "예약"):
		return "booking"
	case strings.Contains(text, "dashboard") || strings.Contains(text, "대시보드"):
		return "dashboard"
	case strings.Contains(text, "marketplace") || strings.Contains(text, "마켓"):
		return "marketplace"
	case strings.Contains(text, "admin") || strings.Contains(text, "관리"):
		return "admin tool"
	default:
		return "prototype"
	}
}

func inferSiteArchetype(payload siteCreateRequest) string {
	purpose := inferSitePurpose(payload)
	switch purpose {
	case "portfolio", "booking", "dashboard", "marketplace":
		return purpose
	case "admin tool":
		return "admin tool"
	default:
		return "landing"
	}
}

func normalizeSiteKeywords(keywords []string, payload siteCreateRequest) []string {
	seenKeywords := map[string]bool{}
	result := []string{}
	for _, keyword := range append(keywords, payload.Purpose, payload.Archetype, payload.Title) {
		normalizedKeyword := strings.ToLower(strings.TrimSpace(keyword))
		if normalizedKeyword == "" || seenKeywords[normalizedKeyword] {
			continue
		}
		seenKeywords[normalizedKeyword] = true
		result = append(result, normalizedKeyword)
	}
	return result
}

func firstSentence(value string) string {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return ""
	}
	for _, separator := range []string{".", "\n", "。", "!", "?"} {
		if index := strings.Index(trimmedValue, separator); index > 0 {
			return strings.TrimSpace(trimmedValue[:index])
		}
	}
	return trimmedValue
}

func normalizeSiteIdentity(identity siteIdentity) siteIdentity {
	return siteIdentity{
		PersonID:       strings.TrimSpace(identity.PersonID),
		Platform:       strings.TrimSpace(identity.Platform),
		PlatformUserID: strings.TrimSpace(identity.PlatformUserID),
		DisplayName:    strings.TrimSpace(identity.DisplayName),
	}
}

func siteIdentityEmpty(identity siteIdentity) bool {
	return strings.TrimSpace(identity.PersonID) == "" &&
		strings.TrimSpace(identity.PlatformUserID) == "" &&
		strings.TrimSpace(identity.DisplayName) == ""
}

func siteIdentityMatches(owner siteIdentity, requester siteIdentity) bool {
	owner = normalizeSiteIdentity(owner)
	requester = normalizeSiteIdentity(requester)
	if owner.PersonID != "" && requester.PersonID != "" && owner.PersonID == requester.PersonID {
		return true
	}
	if owner.PlatformUserID != "" && requester.PlatformUserID != "" && owner.PlatformUserID == requester.PlatformUserID {
		return owner.Platform == "" || requester.Platform == "" || strings.EqualFold(owner.Platform, requester.Platform)
	}
	return false
}

func identityStringMatches(left string, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	return left != "" && right != "" && strings.EqualFold(left, right)
}

func siteCollaboratorCanEdit(collaborators []siteCollaborator, requester siteIdentity, requestedBy string) bool {
	for _, collaborator := range collaborators {
		if !siteCollaboratorRoleCanEdit(collaborator.Role) {
			continue
		}
		if collaborator.PersonID != "" && collaborator.PersonID == requester.PersonID {
			return true
		}
		if collaborator.PlatformUserID != "" && collaborator.PlatformUserID == requester.PlatformUserID {
			return true
		}
		if identityStringMatches(requestedBy, collaborator.PersonID) || identityStringMatches(requestedBy, collaborator.PlatformUserID) {
			return true
		}
	}
	return false
}

func siteCollaboratorRoleCanEdit(role string) bool {
	normalizedRole := strings.ToLower(strings.TrimSpace(role))
	return normalizedRole == "editor" || normalizedRole == "owner"
}

func normalizeSiteCollaborators(collaborators []siteCollaborator) []siteCollaborator {
	result := []siteCollaborator{}
	for _, collaborator := range collaborators {
		normalizedCollaborator := siteCollaborator{
			PersonID:       strings.TrimSpace(collaborator.PersonID),
			PlatformUserID: strings.TrimSpace(collaborator.PlatformUserID),
			Role:           strings.ToLower(strings.TrimSpace(collaborator.Role)),
			GrantedBy:      strings.TrimSpace(collaborator.GrantedBy),
			GrantedAt:      collaborator.GrantedAt,
		}
		if normalizedCollaborator.Role == "" {
			normalizedCollaborator.Role = "viewer"
		}
		if normalizedCollaborator.PersonID == "" && normalizedCollaborator.PlatformUserID == "" {
			continue
		}
		result = append(result, normalizedCollaborator)
	}
	return result
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
		if errorValue := service.writeSiteMetadataMirror(site); errorValue != nil {
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
	if payload.Description != "" {
		site.Description = strings.TrimSpace(payload.Description)
	}
	if payload.Idea != "" {
		site.Idea = strings.TrimSpace(payload.Idea)
	}
	if payload.Purpose != "" {
		site.Purpose = strings.TrimSpace(payload.Purpose)
	}
	if payload.Audience != "" {
		site.Audience = strings.TrimSpace(payload.Audience)
	}
	if payload.Archetype != "" {
		site.Archetype = strings.TrimSpace(payload.Archetype)
	}
	if len(payload.DomainKeywords) > 0 {
		site.DomainKeywords = normalizeSiteKeywords(payload.DomainKeywords, siteCreateRequest{Title: site.Title, Purpose: site.Purpose, Archetype: site.Archetype})
	}
	if payload.Owner != "" {
		site.Owner = strings.TrimSpace(payload.Owner)
	}
	if !siteIdentityEmpty(payload.Requester) && siteIdentityEmpty(site.OwnerIdentity) {
		site.OwnerIdentity = payload.Requester
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
	if errorValue := ensureSiteBuildQualityPassed(site.HostSourcePath); errorValue != nil {
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
		return errors.New("site workspace app/dist is stale; run bun scripts/build.ts in app before publishing")
	}
	return nil
}

func ensureSiteBuildQualityPassed(workspacePath string) error {
	path := filepath.Join(workspacePath, ".internkim", "build-quality.json")
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return errors.New("site workspace is missing .internkim/build-quality.json; run bun scripts/build.ts in app before publishing")
	}
	var quality struct {
		BlockingIssueCount int `json:"blockingIssueCount"`
	}
	if errorValue := json.Unmarshal(document, &quality); errorValue != nil {
		return errorValue
	}
	if quality.BlockingIssueCount > 0 {
		return errors.New("site workspace has blocking quality issues; see .internkim/build-quality.json")
	}
	qualityInformation, errorValue := os.Stat(path)
	if errorValue != nil {
		return errorValue
	}
	latestSourceModTime, errorValue := latestFrontendSourceModTime(filepath.Join(workspacePath, "app"))
	if errorValue != nil {
		return errorValue
	}
	if latestSourceModTime.After(qualityInformation.ModTime()) {
		return errors.New("site workspace build-quality.json is stale; run bun scripts/build.ts in app before publishing")
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
	if strings.TrimSpace(site.PreviousVersionID) == "" && strings.TrimSpace(payload.Revision) == "" {
		return nil, errors.New("previous version is not available")
	}
	nextVersionID, errorValue := service.siteRollbackVersionID(site, payload)
	if errorValue != nil {
		return nil, errorValue
	}
	site.PreviousVersionID = site.CurrentVersionID
	site.CurrentVersionID = nextVersionID
	if errorValue := service.activateSiteVersion(ctx, site, nextVersionID); errorValue != nil {
		return nil, errorValue
	}
	site.Status = SiteStatusPublished
	site.LastPublishedCommit = siteCommitFromVersionID(nextVersionID)
	site.RevisionCount = service.siteRevisionCount(site)
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
	return siteModificationAllowed(site, payload.RequestedBy, payload.Requester, payload.UserConfirmed && payload.Confirm == "CONFIRM")
}

func siteModificationAllowed(site *SiteRecord, requestedBy string, requester siteIdentity, hasAdminConfirmation bool) bool {
	if site == nil {
		return false
	}
	if strings.TrimSpace(site.Owner) == "" && siteIdentityEmpty(site.OwnerIdentity) {
		return true
	}
	if siteIdentityMatches(site.OwnerIdentity, requester) {
		return true
	}
	if identityStringMatches(requestedBy, site.Owner) {
		return true
	}
	if siteCollaboratorCanEdit(site.Collaborators, requester, requestedBy) {
		return true
	}
	return hasAdminConfirmation
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
		{Path: ".internkim/idea.md", Document: siteIdeaMarkdown(site)},
		{Path: "DESIGN.md", Document: siteDesignMD(site)},
		{Path: "app/package.json", Document: sitePackageJSON(site)},
		{Path: "app/index.html", Document: siteIndexHTML(site)},
		{Path: "app/scripts/build.ts", Document: siteBuildTS()},
		{Path: "app/tsconfig.json", Document: siteTSConfigJSON()},
		{Path: "app/vite.config.ts", Document: siteViteConfigTS()},
		{Path: "app/src/App.tsx", Document: siteAppTSX(site)},
		{Path: "app/src/main.tsx", Document: siteMainTSX()},
		{Path: "app/src/index.css", Document: siteIndexCSS()},
		{Path: "app/src/lib/utils.ts", Document: siteUtilsTS()},
		{Path: "app/src/components/ui/badge.tsx", Document: siteBadgeTSX()},
		{Path: "app/src/components/ui/button.tsx", Document: siteButtonTSX()},
		{Path: "app/src/components/ui/card.tsx", Document: siteCardTSX()},
		{Path: "app/src/components/ui/dialog.tsx", Document: siteDialogTSX()},
		{Path: "app/src/components/ui/input.tsx", Document: siteInputTSX()},
		{Path: "app/src/components/ui/label.tsx", Document: siteLabelTSX()},
		{Path: "app/src/components/ui/separator.tsx", Document: siteSeparatorTSX()},
		{Path: "app/src/components/ui/table.tsx", Document: siteTableTSX()},
		{Path: "app/src/components/ui/tabs.tsx", Document: siteTabsTSX()},
		{Path: "app/src/components/ui/textarea.tsx", Document: siteTextareaTSX()},
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

func (service *Service) writeSiteMetadataMirror(site *SiteRecord) error {
	files := []siteTemplateFile{
		{Path: ".internkim/site.json", Document: service.siteWorkspaceMetadata(site)},
		{Path: ".internkim/idea.md", Document: siteIdeaMarkdown(site)},
	}
	for _, file := range files {
		path := filepath.Join(site.HostSourcePath, file.Path)
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
		Description:    site.Description,
		Idea:           site.Idea,
		OriginalPrompt: site.OriginalPrompt,
		Purpose:        site.Purpose,
		Audience:       site.Audience,
		Archetype:      site.Archetype,
		DomainKeywords: site.DomainKeywords,
		CreatedBy:      site.CreatedBy,
		Owner:          site.OwnerIdentity,
		Collaborators:  site.Collaborators,
		Stack:          "React + Vite + TypeScript + Tailwind + shadcn/ui scaffold with optional PocketBase files",
		DesignDefault:  "Stitch-compatible DESIGN.md with beautiful shadcn prototype defaults; customize before publish",
	}, "", "  ")
	if errorValue != nil {
		return "{}\n"
	}
	return string(document) + "\n"
}

func siteIdeaMarkdown(site *SiteRecord) string {
	lines := []string{
		"# Site Idea",
		"",
		"## Summary",
		firstNonEmpty(site.Description, site.Title),
		"",
		"## Original Idea",
		firstNonEmpty(site.Idea, site.OriginalPrompt, site.Description),
		"",
		"## Audience",
		firstNonEmpty(site.Audience, "Unspecified"),
		"",
		"## Purpose",
		firstNonEmpty(site.Purpose, "prototype"),
		"",
		"## Archetype",
		firstNonEmpty(site.Archetype, "landing"),
		"",
	}
	return strings.Join(lines, "\n")
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

type siteHistoryResponse struct {
	SiteID       string              `json:"siteID"`
	Slug         string              `json:"slug"`
	PublishedURL string              `json:"publishedURL"`
	Revisions    []siteRevisionEntry `json:"revisions"`
}

type siteRevisionEntry struct {
	Commit           string `json:"commit"`
	ShortCommit      string `json:"shortCommit"`
	VersionID        string `json:"versionID,omitempty"`
	Message          string `json:"message"`
	TimestampUnix    int64  `json:"timestampUnix"`
	IsCurrent        bool   `json:"isCurrent"`
	IsPrevious       bool   `json:"isPrevious"`
	IsPublishedBuild bool   `json:"isPublishedBuild"`
}

type siteDiffResponse struct {
	SiteID       string `json:"siteID"`
	Slug         string `json:"slug"`
	PublishedURL string `json:"publishedURL"`
	FromRevision string `json:"fromRevision"`
	ToRevision   string `json:"toRevision"`
	Summary      string `json:"summary"`
}

func (service *Service) siteWithRevisionMetadata(site *SiteRecord) *SiteRecord {
	copiedSite := *site
	copiedSite.RevisionCount = service.siteRevisionCount(site)
	return &copiedSite
}

func (service *Service) siteGitHistory(ctx context.Context, site *SiteRecord) (siteHistoryResponse, error) {
	output, errorValue := service.runCommand(ctx, "git", siteGitArguments(site, "log", "--date=unix", "--pretty=format:%H%x1f%ct%x1f%s", "-n", "30")...)
	if errorValue != nil {
		return siteHistoryResponse{}, errorValue
	}
	return siteHistoryResponse{
		SiteID:       site.SiteID,
		Slug:         site.Slug,
		PublishedURL: site.PublishedURL,
		Revisions:    service.siteRevisionEntries(site, string(output)),
	}, nil
}

func (service *Service) siteRevisionEntries(site *SiteRecord, output string) []siteRevisionEntry {
	versionIDs := service.siteVersionIDs(site.SiteID)
	entries := []siteRevisionEntry{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		parts := strings.SplitN(line, "\x1f", 3)
		if len(parts) != 3 {
			continue
		}
		commit := strings.TrimSpace(parts[0])
		shortCommit := shortSiteCommit(commit)
		versionID := versionIDForShortCommit(versionIDs, shortCommit)
		timestampUnix, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		entries = append(entries, siteRevisionEntry{
			Commit:           commit,
			ShortCommit:      shortCommit,
			VersionID:        versionID,
			Message:          strings.TrimSpace(parts[2]),
			TimestampUnix:    timestampUnix,
			IsCurrent:        commitMatchesVersion(commit, site.CurrentVersionID),
			IsPrevious:       commitMatchesVersion(commit, site.PreviousVersionID),
			IsPublishedBuild: versionID != "",
		})
	}
	return entries
}

func (service *Service) siteGitDiff(ctx context.Context, site *SiteRecord, request siteDiffRequest) (siteDiffResponse, error) {
	fromRevision := firstNonEmpty(strings.TrimSpace(request.FromRevision), siteCommitFromVersionID(site.PreviousVersionID))
	toRevision := firstNonEmpty(strings.TrimSpace(request.ToRevision), strings.TrimSpace(site.LastPublishedCommit), "HEAD")
	if fromRevision == "" {
		fromRevision = "HEAD^"
	}
	output, errorValue := service.runCommand(ctx, "git", siteGitArguments(site, "diff", "--stat", "--summary", fromRevision, toRevision)...)
	if errorValue != nil {
		return siteDiffResponse{}, errorValue
	}
	return siteDiffResponse{
		SiteID:       site.SiteID,
		Slug:         site.Slug,
		PublishedURL: site.PublishedURL,
		FromRevision: fromRevision,
		ToRevision:   toRevision,
		Summary:      strings.TrimSpace(string(output)),
	}, nil
}

func (service *Service) siteRollbackVersionID(site *SiteRecord, payload siteLifecycleRequest) (string, error) {
	revision := strings.TrimSpace(payload.Revision)
	if revision == "" {
		return site.PreviousVersionID, nil
	}
	if isDirectory(service.siteVersionPath(site.SiteID, revision)) {
		return revision, nil
	}
	shortCommit := shortSiteCommit(revision)
	for _, versionID := range service.siteVersionIDs(site.SiteID) {
		if strings.HasSuffix(versionID, "-"+shortCommit) {
			return versionID, nil
		}
	}
	return "", errors.New("requested site revision is not available as a published version")
}

func (service *Service) siteRevisionCount(site *SiteRecord) int {
	if !isDirectory(filepath.Join(site.HostSourcePath, ".git")) {
		return 0
	}
	output, errorValue := service.runCommand(context.Background(), "git", siteGitArguments(site, "rev-list", "--count", "HEAD")...)
	if errorValue != nil {
		return 0
	}
	count, errorValue := strconv.Atoi(strings.TrimSpace(string(output)))
	if errorValue != nil {
		return 0
	}
	return count
}

func (service *Service) siteVersionIDs(siteID string) []string {
	entries, errorValue := os.ReadDir(filepath.Join(service.sitePath(siteID), "versions"))
	if errorValue != nil {
		return nil
	}
	versionIDs := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			versionIDs = append(versionIDs, entry.Name())
		}
	}
	sort.Strings(versionIDs)
	return versionIDs
}

func versionIDForShortCommit(versionIDs []string, shortCommit string) string {
	for _, versionID := range versionIDs {
		if strings.HasSuffix(versionID, "-"+shortCommit) {
			return versionID
		}
	}
	return ""
}

func commitMatchesVersion(commit string, versionID string) bool {
	if strings.TrimSpace(versionID) == "" {
		return false
	}
	return strings.HasSuffix(versionID, "-"+shortSiteCommit(commit))
}

func siteCommitFromVersionID(versionID string) string {
	parts := strings.Split(strings.TrimSpace(versionID), "-")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[len(parts)-1])
}

func shortSiteCommit(commit string) string {
	shortCommit := strings.TrimSpace(commit)
	if len(shortCommit) > 12 {
		return shortCommit[:12]
	}
	return shortCommit
}

func siteGitArguments(site *SiteRecord, arguments ...string) []string {
	result := []string{"-c", "safe.directory=" + site.HostSourcePath, "-C", site.HostSourcePath}
	return append(result, arguments...)
}

func (service *Service) copyApprovedPocketBaseHooks(site *SiteRecord, versionPath string, payload sitePublishRequest) error {
	hooksPath := filepath.Join(site.HostSourcePath, "pocketbase", "pb_hooks")
	if !directoryHasOperationalFiles(hooksPath) {
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
	if site.Description == "" {
		site.Description = firstNonEmpty(site.Idea, site.OriginalPrompt, site.Title)
	}
	if site.Idea == "" {
		site.Idea = firstNonEmpty(site.OriginalPrompt, site.Description)
	}
	if site.Purpose == "" {
		site.Purpose = "prototype"
	}
	if site.Archetype == "" {
		site.Archetype = "landing"
	}
	if siteIdentityEmpty(site.OwnerIdentity) && strings.TrimSpace(site.Owner) != "" {
		site.OwnerIdentity = siteIdentity{DisplayName: strings.TrimSpace(site.Owner)}
	}
	if siteIdentityEmpty(site.CreatedBy) {
		site.CreatedBy = site.OwnerIdentity
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

func directoryHasOperationalFiles(path string) bool {
	entries, errorValue := os.ReadDir(path)
	if errorValue != nil {
		return false
	}
	for _, entry := range entries {
		entryPath := filepath.Join(path, entry.Name())
		if entry.IsDir() {
			if directoryHasOperationalFiles(entryPath) {
				return true
			}
			continue
		}
		if siteOperationalFileName(entry.Name()) {
			return true
		}
	}
	return false
}

func siteOperationalFileName(name string) bool {
	switch strings.TrimSpace(name) {
	case "", ".gitkeep", ".DS_Store":
		return false
	default:
		return true
	}
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
			"dev":     "vite --host 0.0.0.0 --port 5173",
			"preview": "vite preview --host 0.0.0.0 --port 4173",
		},
		"dependencies": map[string]string{
			"@radix-ui/react-dialog":   "^1.1.15",
			"@radix-ui/react-tabs":     "^1.1.13",
			"class-variance-authority": "^0.7.1",
			"clsx":                     "^2.1.1",
			"lucide-react":             "^0.468.0",
			"react":                    "^19.2.0",
			"react-dom":                "^19.2.0",
			"tailwind-merge":           "^3.5.0",
		},
		"devDependencies": map[string]string{
			"@google/design.md":    "^0.1.0",
			"@tailwindcss/vite":    "^4.2.2",
			"@types/react":         "^19.2.0",
			"@types/react-dom":     "^19.2.0",
			"@vitejs/plugin-react": "^5.1.1",
			"tailwindcss":          "^4.2.2",
			"typescript":           "^5.9.3",
			"vite":                 "^7.3.1",
		},
		"name":    normalizeSiteSlug(site.Slug),
		"private": true,
		"type":    "module",
		"version": "0.0.0",
	}, "", "  ")
	if errorValue != nil {
		return "{}\n"
	}
	return string(document) + "\n"
}

func siteIndexHTML(site *SiteRecord) string {
	title := html.EscapeString(firstNonEmpty(site.Title, site.Slug))
	return "<!doctype html>\n<html lang=\"ko\">\n<head>\n<meta charset=\"UTF-8\" />\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\" />\n<title>" + title + "</title>\n</head>\n<body>\n<div id=\"root\"></div>\n<script type=\"module\" src=\"/src/main.tsx\"></script>\n</body>\n</html>\n"
}

func siteDesignMD(site *SiteRecord) string {
	title := firstNonEmpty(strings.TrimSpace(site.Title), site.Slug)
	quotedTitle := strconv.Quote(title)
	return `---
version: alpha
name: ` + quotedTitle + `
description: Beautiful default prototype design system for a shadcn React site.
colors:
  primary: "#0F172A"
  primary-foreground: "#F8FAFC"
  secondary: "#2F6B5F"
  tertiary: "#D97706"
  neutral: "#64748B"
  background: "#F7F5EF"
  surface: "#FFFFFF"
  surface-muted: "#EEF2F0"
  border: "#D6D3C9"
  destructive: "#B42318"
typography:
  headline-display:
    fontFamily: ui-serif
    fontSize: 56px
    fontWeight: 650
    lineHeight: 1.02
    letterSpacing: 0px
  headline-lg:
    fontFamily: ui-serif
    fontSize: 38px
    fontWeight: 650
    lineHeight: 1.08
    letterSpacing: 0px
  body-md:
    fontFamily: ui-sans-serif
    fontSize: 16px
    fontWeight: 400
    lineHeight: 1.6
    letterSpacing: 0px
  label-md:
    fontFamily: ui-sans-serif
    fontSize: 13px
    fontWeight: 650
    lineHeight: 1.1
    letterSpacing: 0px
rounded:
  sm: 4px
  md: 8px
  lg: 12px
  full: 9999px
spacing:
  xs: 4px
  sm: 8px
  md: 16px
  lg: 24px
  xl: 40px
  page: 32px
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.primary-foreground}"
    typography: "{typography.label-md}"
    rounded: "{rounded.md}"
    padding: 12px
  button-primary-hover:
    backgroundColor: "{colors.secondary}"
  card:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.primary}"
    rounded: "{rounded.lg}"
    padding: 24px
  input:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.primary}"
    rounded: "{rounded.md}"
    padding: 12px
---

# ` + title + ` DESIGN.md

## Overview

The interface should feel like a polished prototype made for immediate idea validation: useful on the first screen, composed with confident spacing, and refined without looking like a generic SaaS landing page. Default to a calm editorial utility style unless the user request clearly calls for another archetype.

## Colors

The palette uses warm limestone background, crisp white surfaces, slate text, green secondary accents, and amber tertiary highlights. Primary actions use deep slate for contrast. Do not let a single hue dominate the whole page.

## Typography

Use a serif display voice for high-level narrative headings and a clean system sans for product UI, labels, forms, and dense data. Keep letter spacing at 0px unless a specific brand direction requires otherwise.

## Layout

Start from the requested workflow instead of a decorative introduction. App-like requests should open with a usable shell, dashboard, form, board, or editor. Landing requests may use a hero, but the next section must be visible in the first viewport.

## Elevation & Depth

Prefer tonal layers, borders, and restrained shadows. Cards should frame repeated items or tools only; avoid nesting cards inside cards.

## Shapes

Use 8px as the default radius for controls and cards. Use full rounding only for avatars, pills, meters, and compact status indicators.

## Components

Build with shadcn-style primitives: buttons, inputs, labels, cards, badges, tabs, dialogs, tables, and separators. Use lucide icons in icon buttons and compact actions when the meaning is familiar.

## Do's and Don'ts

- Do make the first screen functional for the user's actual request.
- Do include realistic fake data where it helps the workflow feel usable.
- Do verify desktop and mobile layouts before publishing.
- Don't publish placeholder feature-card pages.
- Don't use meaningless gradient blobs, empty hero sections, or decorative filler.
- Don't allow text, buttons, or cards to overlap at mobile widths.
`
}

func siteBuiltIndexHTML(site *SiteRecord) string {
	title := html.EscapeString(firstNonEmpty(site.Title, site.Slug))
	return "<!doctype html>\n<html lang=\"ko\">\n<head>\n<meta charset=\"UTF-8\" />\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\" />\n<title>" + title + "</title>\n<style>body{margin:0;font-family:ui-sans-serif,system-ui;background:#fff;color:#111827}.shell{display:grid;min-height:100vh;place-items:center;padding:24px}</style>\n</head>\n<body><main class=\"shell\" data-starter-marker=\"INTERNKIM_SITE_STARTER_REPLACE_ME\"><h1>" + title + "</h1></main></body>\n</html>\n"
}

func siteBuildTS() string {
	return `import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";

type Command = {
	name: string;
	arguments: string[];
};

type QualityIssue = {
	severity: "blocking" | "warning";
	category: string;
	target: string;
	message: string;
};

async function runCommand(command: Command): Promise<void> {
	const commandProcess = Bun.spawn([command.name, ...command.arguments], {
		stdout: "inherit",
		stderr: "inherit",
	});
	const exitCode = await commandProcess.exited;
	if (exitCode !== 0) {
		throw new Error(command.name + " " + command.arguments.join(" ") + " failed with exit code " + exitCode);
	}
}

function readSource(path: string): string {
	if (!existsSync(path)) return "";
	return readFileSync(path, "utf8");
}

function sourceContainsAny(source: string, values: string[]): boolean {
	return values.some((value) => source.includes(value));
}

function collectQualityIssues(): QualityIssue[] {
	const appSource = readSource("src/App.tsx");
	const styleSource = readSource("src/index.css");
	const issues: QualityIssue[] = [];
	if (!existsSync("src/prototype-data.ts")) {
		issues.push({
			severity: "blocking",
			category: "contentModel",
			target: "src/prototype-data.ts",
			message: "Create domain-specific prototype data before building the site.",
		});
	}
	if (sourceContainsAny(appSource + styleSource, [
		"INTERNKIM_SITE_STARTER_REPLACE_ME",
		"InternKim React prototype",
		"Beautiful default scaffold",
		"Replace this starter",
		"workflowItems",
	])) {
		issues.push({
			severity: "blocking",
			category: "templateSmell",
			target: "src/App.tsx",
			message: "Replace the scaffold starter instead of editing its copy or card-grid structure.",
		});
	}
	return issues;
}

function writeBuildQuality(issues: QualityIssue[]): void {
	mkdirSync("../.internkim", { recursive: true });
	writeFileSync("../.internkim/build-quality.json", JSON.stringify({
		generatedAt: new Date().toISOString(),
		blockingIssueCount: issues.filter((issue) => issue.severity === "blocking").length,
		issues,
	}, null, 2) + "\n");
}

if (!existsSync("../DESIGN.md")) {
	throw new Error("DESIGN.md is required at the site workspace root");
}

const qualityIssues = collectQualityIssues();
if (qualityIssues.some((issue) => issue.severity === "blocking")) {
	writeBuildQuality(qualityIssues);
	throw new Error("site quality gate failed; see ../.internkim/build-quality.json");
}

if (!existsSync("node_modules")) {
	await runCommand({ name: "bun", arguments: ["install"] });
}

await runCommand({ name: "bunx", arguments: ["@google/design.md", "lint", "../DESIGN.md"] });
await runCommand({ name: "bunx", arguments: ["vite", "build"] });
writeBuildQuality(qualityIssues);
`
}

func siteTSConfigJSON() string {
	return `{
  "compilerOptions": {
    "target": "ES2022",
    "useDefineForClassFields": true,
    "lib": ["DOM", "DOM.Iterable", "ES2022"],
    "allowJs": false,
    "skipLibCheck": true,
    "esModuleInterop": true,
    "allowSyntheticDefaultImports": true,
    "strict": true,
    "forceConsistentCasingInFileNames": true,
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx"
  },
  "include": ["src"],
  "references": []
}
`
}

func siteViteConfigTS() string {
	return `import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "vite";

export default defineConfig({
	plugins: [react(), tailwindcss()],
});
`
}

func siteMainTSX() string {
	return `import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import "./index.css";

const rootElement = document.getElementById("root");

if (!rootElement) {
	throw new Error("React root element is missing");
}

createRoot(rootElement).render(
	<StrictMode>
		<App />
	</StrictMode>,
);
`
}

func siteAppTSX(site *SiteRecord) string {
	_ = site
	return `const starterMarker = "INTERNKIM_SITE_STARTER_REPLACE_ME";

function App() {
	return (
		<main data-starter-marker={starterMarker}>
			<h1>Replace this starter with the requested site.</h1>
		</main>
	);
}

export default App;
`
}

func siteIndexCSS() string {
	return `@import "tailwindcss";

:root {
	--background: #f7f5ef;
	--foreground: #0f172a;
	--card: #ffffff;
	--card-foreground: #0f172a;
	--popover: #ffffff;
	--popover-foreground: #0f172a;
	--primary: #0f172a;
	--primary-foreground: #f8fafc;
	--secondary: #2f6b5f;
	--secondary-foreground: #f8fafc;
	--muted: #eef2f0;
	--muted-foreground: #64748b;
	--accent: #d97706;
	--accent-foreground: #fff7ed;
	--destructive: #b42318;
	--destructive-foreground: #fff7ed;
	--border: #d6d3c9;
	--input: #d6d3c9;
	--ring: #2f6b5f;
	--radius: 0.5rem;
	font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Apple SD Gothic Neo", "Segoe UI", sans-serif;
	color: var(--foreground);
	background: var(--background);
}

@theme inline {
	--color-background: var(--background);
	--color-foreground: var(--foreground);
	--color-card: var(--card);
	--color-card-foreground: var(--card-foreground);
	--color-popover: var(--popover);
	--color-popover-foreground: var(--popover-foreground);
	--color-primary: var(--primary);
	--color-primary-foreground: var(--primary-foreground);
	--color-secondary: var(--secondary);
	--color-secondary-foreground: var(--secondary-foreground);
	--color-muted: var(--muted);
	--color-muted-foreground: var(--muted-foreground);
	--color-accent: var(--accent);
	--color-accent-foreground: var(--accent-foreground);
	--color-destructive: var(--destructive);
	--color-destructive-foreground: var(--destructive-foreground);
	--color-border: var(--border);
	--color-input: var(--input);
	--color-ring: var(--ring);
	--radius-sm: calc(var(--radius) - 4px);
	--radius-md: var(--radius);
	--radius-lg: calc(var(--radius) + 4px);
}

* {
	box-sizing: border-box;
	border-color: var(--border);
}

body {
	margin: 0;
	min-width: 320px;
	min-height: 100vh;
}

button,
input,
textarea,
select {
	font: inherit;
}
`
}

func siteUtilsTS() string {
	return `import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
	return twMerge(clsx(inputs));
}
`
}

func siteBadgeTSX() string {
	return `import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "../../lib/utils";

const badgeVariants = cva(
	"inline-flex items-center rounded-md border px-2.5 py-0.5 text-xs font-semibold transition-colors",
	{
		variants: {
			variant: {
				default: "border-transparent bg-primary text-primary-foreground",
				secondary: "border-transparent bg-secondary text-secondary-foreground",
				outline: "text-foreground",
			},
		},
		defaultVariants: {
			variant: "default",
		},
	},
);

export function Badge({
	className,
	variant,
	...properties
}: React.HTMLAttributes<HTMLDivElement> & VariantProps<typeof badgeVariants>) {
	return <div className={cn(badgeVariants({ variant }), className)} {...properties} />;
}
`
}

func siteButtonTSX() string {
	return `import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "../../lib/utils";

const buttonVariants = cva(
	"inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-semibold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50",
	{
		variants: {
			variant: {
				default: "bg-primary text-primary-foreground hover:bg-secondary",
				secondary: "bg-secondary text-secondary-foreground hover:bg-secondary/90",
				outline: "border border-input bg-background hover:bg-muted",
				ghost: "hover:bg-muted",
				destructive: "bg-destructive text-destructive-foreground hover:bg-destructive/90",
			},
			size: {
				default: "h-10 px-4 py-2",
				sm: "h-9 rounded-md px-3",
				lg: "h-11 rounded-md px-8",
				icon: "h-10 w-10",
			},
		},
		defaultVariants: {
			variant: "default",
			size: "default",
		},
	},
);

export function Button({
	className,
	variant,
	size,
	...properties
}: React.ComponentProps<"button"> & VariantProps<typeof buttonVariants>) {
	return <button className={cn(buttonVariants({ variant, size }), className)} {...properties} />;
}
`
}

func siteCardTSX() string {
	return `import * as React from "react";
import { cn } from "../../lib/utils";

export function Card({ className, ...properties }: React.ComponentProps<"div">) {
	return <div className={cn("rounded-lg border bg-card text-card-foreground shadow-sm", className)} {...properties} />;
}

export function CardHeader({ className, ...properties }: React.ComponentProps<"div">) {
	return <div className={cn("flex flex-col space-y-1.5 p-6", className)} {...properties} />;
}

export function CardTitle({ className, ...properties }: React.ComponentProps<"h3">) {
	return <h3 className={cn("text-2xl font-semibold leading-none tracking-[0px]", className)} {...properties} />;
}

export function CardDescription({ className, ...properties }: React.ComponentProps<"p">) {
	return <p className={cn("text-sm text-muted-foreground", className)} {...properties} />;
}

export function CardContent({ className, ...properties }: React.ComponentProps<"div">) {
	return <div className={cn("p-6 pt-0", className)} {...properties} />;
}

export function CardFooter({ className, ...properties }: React.ComponentProps<"div">) {
	return <div className={cn("flex items-center p-6 pt-0", className)} {...properties} />;
}
`
}

func siteDialogTSX() string {
	return `import * as React from "react";
import * as DialogPrimitive from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import { cn } from "../../lib/utils";

export const Dialog = DialogPrimitive.Root;
export const DialogTrigger = DialogPrimitive.Trigger;
export const DialogPortal = DialogPrimitive.Portal;
export const DialogClose = DialogPrimitive.Close;

export function DialogOverlay({ className, ...properties }: React.ComponentProps<typeof DialogPrimitive.Overlay>) {
	return (
		<DialogPrimitive.Overlay
			className={cn("fixed inset-0 z-50 bg-primary/35 backdrop-blur-sm", className)}
			{...properties}
		/>
	);
}

export function DialogContent({ className, children, ...properties }: React.ComponentProps<typeof DialogPrimitive.Content>) {
	return (
		<DialogPortal>
			<DialogOverlay />
			<DialogPrimitive.Content
				className={cn("fixed left-1/2 top-1/2 z-50 grid w-[calc(100%-2rem)] max-w-lg -translate-x-1/2 -translate-y-1/2 gap-4 rounded-lg border bg-popover p-6 text-popover-foreground shadow-lg", className)}
				{...properties}
			>
				{children}
				<DialogPrimitive.Close className="absolute right-4 top-4 rounded-sm opacity-70 transition-opacity hover:opacity-100 focus:outline-none focus:ring-2 focus:ring-ring">
					<X className="h-4 w-4" />
					<span className="sr-only">Close</span>
				</DialogPrimitive.Close>
			</DialogPrimitive.Content>
		</DialogPortal>
	);
}

export function DialogHeader({ className, ...properties }: React.ComponentProps<"div">) {
	return <div className={cn("flex flex-col space-y-1.5 text-left", className)} {...properties} />;
}

export function DialogFooter({ className, ...properties }: React.ComponentProps<"div">) {
	return <div className={cn("flex flex-col-reverse gap-2 sm:flex-row sm:justify-end", className)} {...properties} />;
}

export const DialogTitle = DialogPrimitive.Title;
export const DialogDescription = DialogPrimitive.Description;
`
}

func siteInputTSX() string {
	return `import * as React from "react";
import { cn } from "../../lib/utils";

export function Input({ className, type, ...properties }: React.ComponentProps<"input">) {
	return (
		<input
			type={type}
			className={cn("flex h-10 w-full rounded-md border border-input bg-card px-3 py-2 text-sm ring-offset-background transition-colors file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50", className)}
			{...properties}
		/>
	);
}
`
}

func siteLabelTSX() string {
	return `import * as React from "react";
import { cn } from "../../lib/utils";

export function Label({ className, ...properties }: React.ComponentProps<"label">) {
	return <label className={cn("text-sm font-semibold leading-none", className)} {...properties} />;
}
`
}

func siteSeparatorTSX() string {
	return `import * as React from "react";
import { cn } from "../../lib/utils";

export function Separator({ className, orientation = "horizontal", ...properties }: React.ComponentProps<"div"> & { orientation?: "horizontal" | "vertical" }) {
	return (
		<div
			className={cn(orientation === "horizontal" ? "h-px w-full" : "h-full w-px", "shrink-0 bg-border", className)}
			{...properties}
		/>
	);
}
`
}

func siteTableTSX() string {
	return `import * as React from "react";
import { cn } from "../../lib/utils";

export function Table({ className, ...properties }: React.ComponentProps<"table">) {
	return (
		<div className="w-full overflow-auto">
			<table className={cn("w-full caption-bottom text-sm", className)} {...properties} />
		</div>
	);
}

export function TableHeader({ className, ...properties }: React.ComponentProps<"thead">) {
	return <thead className={cn("[&_tr]:border-b", className)} {...properties} />;
}

export function TableBody({ className, ...properties }: React.ComponentProps<"tbody">) {
	return <tbody className={cn("[&_tr:last-child]:border-0", className)} {...properties} />;
}

export function TableRow({ className, ...properties }: React.ComponentProps<"tr">) {
	return <tr className={cn("border-b transition-colors hover:bg-muted/60", className)} {...properties} />;
}

export function TableHead({ className, ...properties }: React.ComponentProps<"th">) {
	return <th className={cn("h-10 px-2 text-left align-middle font-semibold text-muted-foreground", className)} {...properties} />;
}

export function TableCell({ className, ...properties }: React.ComponentProps<"td">) {
	return <td className={cn("p-2 align-middle", className)} {...properties} />;
}

export function TableCaption({ className, ...properties }: React.ComponentProps<"caption">) {
	return <caption className={cn("mt-4 text-sm text-muted-foreground", className)} {...properties} />;
}
`
}

func siteTabsTSX() string {
	return `import * as React from "react";
import * as TabsPrimitive from "@radix-ui/react-tabs";
import { cn } from "../../lib/utils";

export const Tabs = TabsPrimitive.Root;

export function TabsList({ className, ...properties }: React.ComponentProps<typeof TabsPrimitive.List>) {
	return (
		<TabsPrimitive.List
			className={cn("inline-flex h-10 items-center justify-center rounded-md bg-muted p-1 text-muted-foreground", className)}
			{...properties}
		/>
	);
}

export function TabsTrigger({ className, ...properties }: React.ComponentProps<typeof TabsPrimitive.Trigger>) {
	return (
		<TabsPrimitive.Trigger
			className={cn("inline-flex items-center justify-center whitespace-nowrap rounded-sm px-3 py-1.5 text-sm font-semibold transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50 data-[state=active]:bg-card data-[state=active]:text-foreground data-[state=active]:shadow-sm", className)}
			{...properties}
		/>
	);
}

export function TabsContent({ className, ...properties }: React.ComponentProps<typeof TabsPrimitive.Content>) {
	return (
		<TabsPrimitive.Content
			className={cn("ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring", className)}
			{...properties}
		/>
	);
}
`
}

func siteTextareaTSX() string {
	return `import * as React from "react";
import { cn } from "../../lib/utils";

export function Textarea({ className, ...properties }: React.ComponentProps<"textarea">) {
	return (
		<textarea
			className={cn("flex min-h-24 w-full rounded-md border border-input bg-card px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50", className)}
			{...properties}
		/>
	);
}
`
}
