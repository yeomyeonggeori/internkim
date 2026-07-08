package admind

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
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

const publishedSiteLimit = 10

const (
	staffCircleSitesWorkspacePath = "/workspace/circles/staff/sites"
	siteIDStorageDirectoryName    = ".ids"
)

var sitePersistenceMutex sync.Mutex

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
	DraftPath           string             `json:"draftPath,omitempty"`
	HostSourcePath      string             `json:"hostSourcePath,omitempty"`
	PreviewID           string             `json:"previewID,omitempty"`
	PreviewURL          string             `json:"previewURL,omitempty"`
	PreviewExpiresAt    time.Time          `json:"previewExpiresAt,omitempty"`
	QualityStatus       string             `json:"qualityStatus,omitempty"`
	QualityIssueCount   int                `json:"qualityIssueCount,omitempty"`
	QualitySummary      []string           `json:"qualitySummary,omitempty"`
	QualityReportPath   string             `json:"qualityReportPath,omitempty"`
	LastPublishedCommit string             `json:"lastPublishedCommit,omitempty"`
	RevisionCount       int                `json:"revisionCount"`
	CreatedAt           time.Time          `json:"createdAt"`
	UpdatedAt           time.Time          `json:"updatedAt"`
	UnpublishedAt       time.Time          `json:"unpublishedAt,omitempty"`
	DeletedAt           time.Time          `json:"deletedAt,omitempty"`
	LastError           string             `json:"lastError,omitempty"`
	LiveHTTPStatus      int                `json:"liveHTTPStatus,omitempty"`
	NextOperation       string             `json:"nextOperation,omitempty"`
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
	Content             *siteContent       `json:"content"`
}

type siteContentSection struct {
	Title string          `json:"title"`
	Body  siteContentText `json:"body"`
}

type siteContentText string

func (text *siteContentText) UnmarshalJSON(document []byte) error {
	var single string
	if errorValue := json.Unmarshal(document, &single); errorValue == nil {
		*text = siteContentText(single)
		return nil
	}
	var lines []string
	if errorValue := json.Unmarshal(document, &lines); errorValue != nil {
		return errors.New("body must be a string or an array of strings")
	}
	*text = siteContentText(strings.Join(lines, "\n\n"))
	return nil
}

type siteContentBlockItem struct {
	Title string          `json:"title"`
	Body  siteContentText `json:"body"`
	Icon  string          `json:"icon,omitempty"`
}

type siteContentBlock struct {
	Variant     string                 `json:"variant"`
	Title       string                 `json:"title,omitempty"`
	Body        siteContentText        `json:"body,omitempty"`
	Items       []siteContentBlockItem `json:"items,omitempty"`
	ActionLabel string                 `json:"actionLabel,omitempty"`
	ActionHref  string                 `json:"actionHref,omitempty"`
	Image       string                 `json:"image,omitempty"`
	ImageAlt    string                 `json:"imageAlt,omitempty"`
	Backdrop    string                 `json:"backdrop,omitempty"`
}

var siteContentKnownBlockVariants = map[string]bool{
	"hero":     true,
	"features": true,
	"prose":    true,
	"cta":      true,
	"faq":      true,
	"contact":  true,
}

type siteContent struct {
	SiteName        string               `json:"siteName"`
	Tagline         string               `json:"tagline,omitempty"`
	HeroActionLabel string               `json:"heroActionLabel,omitempty"`
	HeroActionHref  string               `json:"heroActionHref,omitempty"`
	Sections        []siteContentSection `json:"sections,omitempty"`
	Blocks          []siteContentBlock   `json:"blocks,omitempty"`
	Pages           []siteContentPage    `json:"pages,omitempty"`
}

type siteContentPage struct {
	Path   string             `json:"path"`
	Title  string             `json:"title"`
	Blocks []siteContentBlock `json:"blocks"`
}

var siteContentReservedPathPrefixes = []string{"/api", "/_", "/fonts"}

func validateSiteContentPages(pages []siteContentPage) error {
	seenPaths := map[string]bool{}
	for pageIndex, page := range pages {
		path := strings.TrimSpace(page.Path)
		if !strings.HasPrefix(path, "/") {
			return fmt.Errorf("pages[%d].path must start with /", pageIndex)
		}
		for _, reservedPrefix := range siteContentReservedPathPrefixes {
			if path == reservedPrefix || strings.HasPrefix(path, reservedPrefix+"/") {
				return fmt.Errorf("pages[%d].path %q is reserved for the site backend", pageIndex, path)
			}
		}
		if seenPaths[path] {
			return fmt.Errorf("pages[%d].path %q is duplicated", pageIndex, path)
		}
		seenPaths[path] = true
		if strings.TrimSpace(page.Title) == "" {
			return fmt.Errorf("pages[%d].title is required", pageIndex)
		}
		if len(page.Blocks) == 0 {
			return fmt.Errorf("pages[%d].blocks must include at least one block", pageIndex)
		}
		if errorValue := validateSiteContentBlocks(page.Blocks); errorValue != nil {
			return fmt.Errorf("pages[%d]: %s", pageIndex, errorValue.Error())
		}
	}
	if !seenPaths["/"] {
		return errors.New("pages must include a / page")
	}
	return nil
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
	PreviewID               string       `json:"previewID"`
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
	PreviewURL     string             `json:"previewURL,omitempty"`
	Platform       string             `json:"platform,omitempty"`
	ConversationID string             `json:"conversationID,omitempty"`
	DraftPath      string             `json:"draftPath,omitempty"`
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

type siteReadinessProbeContextKey struct{}

type siteReadinessProbe struct {
	SiteID    string
	VersionID string
}

type siteReadinessProbeResult struct {
	StatusCode int
	Body       string
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
	if fontPath, isFontRequest := strings.CutPrefix(request.URL.Path, "/fonts/"); isFontRequest {
		service.serveSiteFont(responseWriter, request, fontPath)
		return
	}
	switch site.Status {
	case SiteStatusPublished:
		service.servePublishedSite(responseWriter, request, site)
	case SiteStatusPublishing:
		if probe, isProbe := siteReadinessProbeFromRequest(request); isProbe && probe.SiteID == site.SiteID {
			site.CurrentVersionID = probe.VersionID
			service.servePublishedSite(responseWriter, request, site)
			return
		}
		http.NotFound(responseWriter, request)
	case SiteStatusDraft:
		if isPocketBasePath(request.URL.Path) {
			service.serveDraftSitePocketBase(responseWriter, request, site)
			return
		}
		previewID, _ := sitePreviewRequestPath(request.URL.Path)
		if sitePreviewActive(site, previewID) {
			service.serveSiteFrontend(responseWriter, request, site)
			return
		}
		http.NotFound(responseWriter, request)
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

func siteReadinessProbeFromRequest(request *http.Request) (siteReadinessProbe, bool) {
	probe, isProbe := request.Context().Value(siteReadinessProbeContextKey{}).(siteReadinessProbe)
	if !isProbe || strings.TrimSpace(probe.SiteID) == "" || strings.TrimSpace(probe.VersionID) == "" {
		return siteReadinessProbe{}, false
	}
	return probe, true
}

func (service *Service) servePublishedSite(responseWriter http.ResponseWriter, request *http.Request, site *SiteRecord) {
	if isPocketBasePath(request.URL.Path) {
		if !service.siteVersionHasPocketBaseBackend(site, site.CurrentVersionID) {
			http.NotFound(responseWriter, request)
			return
		}
		service.proxySitePocketBase(responseWriter, request, site)
		return
	}
	service.serveSiteFrontend(responseWriter, request, site)
}

func isPocketBasePath(path string) bool {
	return strings.HasPrefix(path, "/api/") || path == "/api" || strings.HasPrefix(path, "/_/")
}

func (service *Service) proxySitePocketBase(responseWriter http.ResponseWriter, request *http.Request, site *SiteRecord) {
	if errorValue := service.ensureSitePocketBaseRunning(request.Context(), site); errorValue != nil {
		http.Error(responseWriter, "site backend is starting up, retry shortly: "+errorValue.Error(), http.StatusServiceUnavailable)
		return
	}
	defer service.finishSitePocketBaseRequest(site.SiteID)
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
	rootPath := filepath.Join(service.sitePublishedVersionPath(site, site.CurrentVersionID), "frontend", "dist")
	previewID, requestPath := sitePreviewRequestPath(request.URL.Path)
	if previewID != "" {
		if !sitePreviewActive(site, previewID) {
			http.NotFound(responseWriter, request)
			return
		}
		rootPath = filepath.Join(service.sitePreviewPath(site, previewID), "frontend", "dist")
	}
	relativePath, errorValue := cleanSiteFrontendPath(requestPath)
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

func sitePreviewRequestPath(path string) (string, string) {
	cleanPath := "/" + strings.TrimLeft(path, "/")
	if !strings.HasPrefix(cleanPath, "/__preview/") {
		return "", path
	}
	rest := strings.TrimPrefix(cleanPath, "/__preview/")
	previewID, suffix, _ := strings.Cut(rest, "/")
	previewID = strings.TrimSpace(previewID)
	if previewID == "" {
		return "", path
	}
	if strings.TrimSpace(suffix) == "" {
		suffix = "index.html"
	}
	return previewID, "/" + suffix
}

func sitePreviewActive(site *SiteRecord, previewID string) bool {
	if site == nil || strings.TrimSpace(site.PreviewID) == "" || site.PreviewID != strings.TrimSpace(previewID) {
		return false
	}
	return site.PreviewExpiresAt.IsZero() || time.Now().UTC().Before(site.PreviewExpiresAt)
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

// deviceHost falls back to <fleetID>.example.test when the device-url file is
// absent, matching publicDeviceURL and mattermostFlowBaseURL — a bare fleet ID
// is not a resolvable host and would publish sites at unreachable URLs.
func (service *Service) deviceHost() string {
	deviceURL := strings.TrimSpace(readTrimmedFile(service.Configuration.DeviceURLPath))
	if deviceURL != "" {
		parsedURL, errorValue := url.Parse(deviceURL)
		if errorValue == nil && parsedURL.Host != "" {
			return normalizeHTTPHost(parsedURL.Host)
		}
	}
	fleetID := normalizeHTTPHost(strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)))
	if fleetID == "" {
		return ""
	}
	if strings.Contains(fleetID, ".") {
		return fleetID
	}
	return fleetID + ".example.test"
}

func normalizeHTTPHost(host string) string {
	trimmedHost := strings.ToLower(strings.TrimSpace(host))
	hostWithoutPort, _, errorValue := net.SplitHostPort(trimmedHost)
	if errorValue == nil {
		return hostWithoutPort
	}
	return strings.Trim(trimmedHost, "[]")
}

func (service *Service) listSites(responseWriter http.ResponseWriter, request *http.Request) {
	service.writeJSON(responseWriter, map[string]any{"sites": service.siteListResponse(request.Context(), siteListShouldCheckLive(request))})
}

func (service *Service) createSite(responseWriter http.ResponseWriter, request *http.Request) {
	var payload siteCreateRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	site := service.reusableSiteForCreate(payload)
	if site == nil {
		createdSite, errorValue := service.createSiteRecord(payload)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
			return
		}
		site = createdSite
	}
	if errorValue := service.materializeSiteSourceWorkspace(request.Context(), site, payload.Content); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeSiteCreateRecord(responseWriter, site, payload.Content)
}

// reusableSiteForCreate returns the conversation's existing site so repeated
// create requests update it in place instead of spawning duplicates. A new site
// is created only for a conversation that does not yet have one; deleting the
// existing site is the explicit way to start over.
func (service *Service) reusableSiteForCreate(payload siteCreateRequest) *SiteRecord {
	conversationID := strings.TrimSpace(payload.ConversationID)
	if conversationID == "" {
		return nil
	}
	for _, site := range service.siteList() {
		if site != nil && strings.EqualFold(strings.TrimSpace(site.ConversationID), conversationID) {
			return site
		}
	}
	return nil
}

// materializeSiteSourceWorkspace provisions the managed scaffold and the editable
// staff-circle workspace for a site. Blueclaw no longer writes site source; the
// admin service owns the scaffold and the editable workspace. content is only
// applied when explicitly provided (create); repair passes nil so existing
// edits are never overwritten.
func (service *Service) materializeSiteSourceWorkspace(ctx context.Context, site *SiteRecord, content *siteContent) error {
	if errorValue := service.prepareSiteWorkspace(ctx, site, content); errorValue != nil {
		return errorValue
	}
	return service.migrateSiteSourceToStaffCircle(site)
}

// repairSiteFromRequest re-materializes any missing managed scaffold files into
// the editable workspace without overwriting existing edits.
func (service *Service) repairSiteFromRequest(responseWriter http.ResponseWriter, request *http.Request, siteID string) {
	site := service.findSiteByID(siteID)
	if site == nil {
		http.NotFound(responseWriter, request)
		return
	}
	if errorValue := service.materializeSiteSourceWorkspace(request.Context(), site, nil); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeSiteRecord(responseWriter, site)
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
	case request.Method == http.MethodPost && action == "preview":
		service.previewSiteFromRequest(responseWriter, request, siteID)
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
	case request.Method == http.MethodPost && action == "repair":
		service.repairSiteFromRequest(responseWriter, request, siteID)
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
	preparedSite, errorValue := service.prepareSiteWorkspaceForResponse(site)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeSiteRecord(responseWriter, service.siteWithRevisionMetadata(preparedSite))
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
	service.writeSiteRecord(responseWriter, site)
}

func (service *Service) previewSiteFromRequest(responseWriter http.ResponseWriter, request *http.Request, siteID string) {
	payload, errorValue := decodeSitePublishRequest(request.Body)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	payload.SiteID = siteID
	site, errorValue := service.previewSite(request.Context(), payload)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	service.writeSiteRecord(responseWriter, site)
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
	service.writeSiteRecord(responseWriter, site)
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
	service.writeSiteRecord(responseWriter, site)
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
	service.writeSiteRecord(responseWriter, site)
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
	service.writeSiteRecord(responseWriter, site)
}

func (service *Service) writeSiteRecord(responseWriter http.ResponseWriter, site *SiteRecord) {
	service.writeJSON(responseWriter, siteAPIResponse(site))
}

func siteAPIResponse(site *SiteRecord) *SiteRecord {
	if site == nil {
		return nil
	}
	copiedSite := *site
	if copiedSite.Status != SiteStatusPublished {
		copiedSite.PublishedURL = ""
	}
	if personalDraftPath := siteOwnerSourceWorkspacePath(&copiedSite); personalDraftPath != "" {
		copiedSite.SourceWorkspacePath = personalDraftPath
		copiedSite.DraftPath = personalDraftPath
		copiedSite.AppWorkspacePath = filepath.ToSlash(filepath.Join(personalDraftPath, "app"))
		copiedSite.WorkspacePath = siteOwnerProjectWorkspacePath(&copiedSite)
	}
	copiedSite.NextOperation = siteNextOperation(&copiedSite)
	return &copiedSite
}

// siteNextOperation gives the agent a deterministic, machine-actionable next
// step for the site's current lifecycle status. It carries no user-facing
// prose; it is a routing hint the model can act on directly.
func siteNextOperation(site *SiteRecord) string {
	switch site.Status {
	case SiteStatusDraft:
		return "site.create succeeded and is not published yet. Compose app/public/site-content.json, then call site.publish with siteID=" + site.SiteID + " when the content is ready. Do not call site.create again for this site."
	case SiteStatusPublishing:
		return "publish is in progress. Call site.status with siteID=" + site.SiteID + " again to check for completion."
	case SiteStatusUnpublished:
		return "site is unpublished. Call site.publish with siteID=" + site.SiteID + " to republish, or site.restore if this was not intended."
	case SiteStatusFailed:
		return "last operation failed (see lastError). Call site.repair with siteID=" + site.SiteID + " to re-materialize the workspace, or fix the reported cause and retry the same operation."
	default:
		return ""
	}
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

func deriveSiteSlug(slug string, title string) string {
	for _, candidate := range []string{slug, title} {
		if normalized := normalizeSiteSlug(candidate); isValidSiteSlug(normalized) {
			return normalized
		}
	}
	return "site-" + randomHex(6)
}

func (service *Service) createSiteRecord(payload siteCreateRequest) (*SiteRecord, error) {
	baseSlug := deriveSiteSlug(payload.Slug, payload.Title)
	port, errorValue := service.allocateSitePort()
	if errorValue != nil {
		return nil, errorValue
	}
	now := time.Now().UTC()
	siteID := randomHex(12)
	createdBy := siteCreatorIdentity(payload)
	ownerIdentity := siteOwnerIdentity(payload, createdBy)
	slug, errorValue := service.availableSiteSlug(baseSlug, siteID, siteCreationSlugSuffix(payload, createdBy, now))
	if errorValue != nil {
		return nil, errorValue
	}
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
		WorkspacePath:       siteProjectAliasWorkspacePath(siteID, slug),
		SourceWorkspacePath: siteDraftWorkspacePath(siteID, slug, payload.SourceWorkspacePath),
		AppWorkspacePath:    filepath.ToSlash(filepath.Join(siteDraftWorkspacePath(siteID, slug, payload.SourceWorkspacePath), "app")),
		DraftPath:           siteDraftWorkspacePath(siteID, slug, payload.SourceWorkspacePath),
		HostSourcePath:      service.siteSourceLedgerPath(siteID),
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	return site, service.storeSite(site)
}

func (service *Service) availableSiteSlug(baseSlug string, siteID string, preferredSuffix string) (string, error) {
	if service.findSiteBySlug(baseSlug) == nil {
		return baseSlug, nil
	}
	for _, candidateSuffix := range siteSlugCandidateSuffixes(siteID, preferredSuffix) {
		candidateSlug := siteSlugWithSuffix(baseSlug, candidateSuffix)
		if isValidSiteSlug(candidateSlug) && service.findSiteBySlug(candidateSlug) == nil {
			return candidateSlug, nil
		}
	}
	return "", errors.New("site slug already exists")
}

func siteSlugCandidateSuffixes(siteID string, preferredSuffix string) []string {
	suffixes := []string{}
	if strings.TrimSpace(preferredSuffix) != "" {
		suffixes = append(suffixes, preferredSuffix)
		if len(siteID) >= 6 {
			suffixes = append(suffixes, siteSlugWithSuffix(preferredSuffix, siteID[:6]))
		}
	}
	for _, suffixLength := range []int{6, 8, 12} {
		if len(siteID) >= suffixLength {
			suffixes = append(suffixes, siteID[:suffixLength])
		}
	}
	return suffixes
}

func siteCreationSlugSuffix(payload siteCreateRequest, createdBy siteIdentity, createdAt time.Time) string {
	requesterToken := siteRequesterSlugToken(payload, createdBy)
	timestampToken := createdAt.UTC().Format("20060102t150405z")
	return siteSlugWithSuffix(requesterToken, timestampToken)
}

func siteRequesterSlugToken(payload siteCreateRequest, createdBy siteIdentity) string {
	for _, value := range []string{
		createdBy.PersonID,
		createdBy.PlatformUserID,
		createdBy.DisplayName,
		payload.RequestedBy,
		payload.Owner,
	} {
		token := normalizeSiteSlugToken(value)
		if token != "" {
			return token
		}
	}
	return "requester"
}

func normalizeSiteSlugToken(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	builder := strings.Builder{}
	previousHyphen := false
	for _, character := range normalized {
		isAllowedLetter := character >= 'a' && character <= 'z'
		isAllowedDigit := character >= '0' && character <= '9'
		if isAllowedLetter || isAllowedDigit {
			builder.WriteRune(character)
			previousHyphen = false
			continue
		}
		if !previousHyphen && builder.Len() > 0 {
			builder.WriteByte('-')
			previousHyphen = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func (service *Service) publishSite(ctx context.Context, payload sitePublishRequest) (*SiteRecord, error) {
	site, errorValue := service.siteForPublish(payload)
	if errorValue != nil {
		return nil, errorValue
	}
	if !siteModificationAllowed(site, payload.RequestedBy, payload.Requester, false) {
		return nil, errors.New("site editor permission is required")
	}
	if errorValue := service.enforcePublishedSiteLimit(site); errorValue != nil {
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
	if errorValue := service.ensureSitePortNotTakenByAnotherSite(site); errorValue != nil {
		service.updateSiteStatus(site.SiteID, SiteStatusFailed, errorValue.Error())
		return nil, errorValue
	}
	service.updateSiteStatus(site.SiteID, SiteStatusPublishing, "")
	if errorValue := service.prepareSiteVersion(site, versionID, payload); errorValue != nil {
		service.updateSiteStatus(site.SiteID, SiteStatusFailed, errorValue.Error())
		return nil, errorValue
	}
	if errorValue := service.activateSiteVersion(ctx, site, versionID); errorValue != nil {
		service.updateSiteStatus(site.SiteID, SiteStatusFailed, errorValue.Error())
		return nil, errorValue
	}
	if errorValue := service.probeSiteReadiness(ctx, site, versionID); errorValue != nil {
		service.updateSiteStatus(site.SiteID, SiteStatusFailed, errorValue.Error())
		return nil, errorValue
	}
	service.clearSitePreview(site)
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

func (service *Service) enforcePublishedSiteLimit(site *SiteRecord) error {
	if site.Status == SiteStatusPublished {
		return nil
	}
	publishedSites := service.publishedSites()
	if len(publishedSites) < publishedSiteLimit {
		return nil
	}
	sort.Slice(publishedSites, func(leftIndex int, rightIndex int) bool {
		return publishedSites[leftIndex].UpdatedAt.After(publishedSites[rightIndex].UpdatedAt)
	})
	descriptions := make([]string, 0, len(publishedSites))
	for _, publishedSite := range publishedSites {
		descriptions = append(descriptions, describePublishedSiteForLimitError(publishedSite))
	}
	return fmt.Errorf(
		"site publish limit reached (%d/%d); unpublish one of the published sites first: %s",
		publishedSiteLimit,
		publishedSiteLimit,
		strings.Join(descriptions, "; "),
	)
}

func (service *Service) publishedSites() []*SiteRecord {
	publishedSites := []*SiteRecord{}
	for _, site := range service.siteList() {
		if site.Status == SiteStatusPublished {
			publishedSites = append(publishedSites, site)
		}
	}
	return publishedSites
}

func describePublishedSiteForLimitError(site *SiteRecord) string {
	displayName := firstNonEmpty(strings.TrimSpace(site.Title), site.Slug)
	publishedURL := firstNonEmpty(strings.TrimSpace(site.PublishedURL), site.Slug)
	return fmt.Sprintf(
		"%s (siteID=%s, url=%s, publishedAt=%s)",
		displayName,
		site.SiteID,
		publishedURL,
		site.UpdatedAt.Format(time.RFC3339),
	)
}

func (service *Service) previewSite(ctx context.Context, payload sitePublishRequest) (*SiteRecord, error) {
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
		return nil, errorValue
	}
	previewID := firstNonEmpty(strings.TrimSpace(payload.PreviewID), "preview-"+randomHex(4))
	if errorValue := service.prepareSitePreview(site, previewID); errorValue != nil {
		return nil, errorValue
	}
	site.PreviewID = previewID
	site.PreviewURL = service.sitePreviewURL(site, previewID)
	site.PreviewExpiresAt = time.Now().UTC().Add(24 * time.Hour)
	site.UpdatedAt = time.Now().UTC()
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
	} else {
		editWorkspaceDraftPath := filepath.Join(service.siteProjectStorageHostPath(site.SiteID), "draft")
		if !directoryHasEntries(editWorkspaceDraftPath) {
			return errors.New("site source workspace is empty; create or repair the site before publishing")
		}
		if errorValue := clearSiteHostSourceWorkspace(site.HostSourcePath); errorValue != nil {
			return errorValue
		}
		if errorValue := copySiteSourceTree(editWorkspaceDraftPath, site.HostSourcePath); errorValue != nil {
			return errorValue
		}
	}
	if errorValue := service.writeSiteMetadataMirror(site); errorValue != nil {
		return errorValue
	}
	return service.initializeSiteGitRepository(ctx, site)
}

// copySiteSourceTree mirrors a site source tree into the publish ledger, skipping
// version-control state so the ledger's own git history is preserved.
func copySiteSourceTree(sourceRoot string, targetRoot string) error {
	return filepath.Walk(sourceRoot, func(sourcePath string, information os.FileInfo, walkError error) error {
		if walkError != nil {
			return walkError
		}
		relativePath, errorValue := filepath.Rel(sourceRoot, sourcePath)
		if errorValue != nil || relativePath == "." {
			return errorValue
		}
		if relativePath == ".git" {
			if information.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		targetPath := filepath.Join(targetRoot, relativePath)
		if information.IsDir() {
			return os.MkdirAll(targetPath, information.Mode())
		}
		if !information.Mode().IsRegular() {
			return nil
		}
		if errorValue := os.MkdirAll(filepath.Dir(targetPath), 0o750); errorValue != nil {
			return errorValue
		}
		return copyFile(sourcePath, targetPath, information.Mode())
	})
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
		site.HostSourcePath = service.siteSourceLedgerPath(site.SiteID)
	}
	if site.SourceWorkspacePath == "" {
		site.SourceWorkspacePath = siteDraftWorkspacePath(site.SiteID, site.Slug, "")
	}
	if site.WorkspacePath == "" {
		site.WorkspacePath = siteProjectAliasWorkspacePath(site.SiteID, site.Slug)
	}
	if site.DraftPath == "" {
		site.DraftPath = site.SourceWorkspacePath
	}
	if site.AppWorkspacePath == "" {
		site.AppWorkspacePath = filepath.ToSlash(filepath.Join(site.SourceWorkspacePath, "app"))
	}
	if errorValue := service.ensureSiteWorkspaceAlias(site); errorValue != nil {
		return errorValue
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
	versionPath := service.sitePublishedVersionPath(site, versionID)
	if errorValue := os.MkdirAll(versionPath, 0o700); errorValue != nil {
		return errorValue
	}
	if errorValue := service.materializeSiteFrontendDist(site, filepath.Join(versionPath, "frontend", "dist"), "publishing"); errorValue != nil {
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

func (service *Service) prepareSitePreview(site *SiteRecord, previewID string) error {
	previewPath := service.sitePreviewPath(site, previewID)
	if errorValue := os.RemoveAll(previewPath); errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(previewPath, 0o700); errorValue != nil {
		return errorValue
	}
	return service.materializeSiteFrontendDist(site, filepath.Join(previewPath, "frontend", "dist"), "preview")
}

// materializeSiteFrontendDist fills frontendDistPath with the site's published
// frontend. When the editable app/ source still matches the scaffold-app
// manifest (i.e. only content was edited, never app/src or the build config),
// it uses the canonical embedded dist directly and skips the build-freshness
// check entirely — basic sites publish without a Blueclaw build. Otherwise it
// falls back to the legacy fresh-build requirement. Both paths validate and
// overlay app/public/** (including site-content.json) onto the result, so
// content-only edits always take effect regardless of which path was used.
func (service *Service) materializeSiteFrontendDist(site *SiteRecord, frontendDistPath string, actionLabel string) error {
	if errorValue := validateSiteApplicationContentFile(site.HostSourcePath); errorValue != nil {
		return errorValue
	}
	if siteAppMatchesScaffoldManifest(site.HostSourcePath) {
		if errorValue := service.materializeCanonicalSiteFrontendDist(site, frontendDistPath); errorValue != nil {
			return errorValue
		}
		service.markSiteBuildQualityPassedPrebuilt(site)
	} else {
		frontendBuildPath := filepath.Join(site.HostSourcePath, "app", "dist")
		if !isDirectory(frontendBuildPath) {
			return fmt.Errorf("site workspace must contain app/dist; content-only edits via %s never need a build — a build (bun scripts/build.ts in the app workspace) is needed only after app/src or app config changes; run it in Blueclaw before %s", siteApplicationContentPath, actionLabel)
		}
		if errorValue := ensureSiteFrontendBuildIsFresh(site.HostSourcePath, frontendBuildPath); errorValue != nil {
			return errorValue
		}
		service.updateSiteBuildQualitySummary(site)
		if errorValue := materializeDirectory(frontendBuildPath, frontendDistPath); errorValue != nil {
			return errorValue
		}
	}
	if errorValue := service.overlaySiteApplicationPublicDirectory(site, frontendDistPath); errorValue != nil {
		return errorValue
	}
	return applySiteDesignTheme(site.HostSourcePath, frontendDistPath)
}

func (service *Service) materializeCanonicalSiteFrontendDist(site *SiteRecord, frontendDistPath string) error {
	if errorValue := os.RemoveAll(frontendDistPath); errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(frontendDistPath, 0o755); errorValue != nil {
		return errorValue
	}
	for _, file := range siteScaffoldCanonicalDistFiles(site) {
		path := filepath.Join(frontendDistPath, filepath.FromSlash(file.Path))
		if errorValue := os.MkdirAll(filepath.Dir(path), 0o755); errorValue != nil {
			return errorValue
		}
		if errorValue := os.WriteFile(path, []byte(file.Document), 0o644); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func (service *Service) overlaySiteApplicationPublicDirectory(site *SiteRecord, frontendDistPath string) error {
	return copyOptionalDirectory(filepath.Join(site.HostSourcePath, "app", "public"), frontendDistPath)
}

func (service *Service) markSiteBuildQualityPassedPrebuilt(site *SiteRecord) {
	site.QualityStatus = "passed_prebuilt"
	site.QualityIssueCount = 0
	site.QualitySummary = []string{}
	site.QualityReportPath = filepath.Join(site.HostSourcePath, ".internkim", "build-quality.json")
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
		return fmt.Errorf("site workspace app/dist is stale; content-only edits via %s never need a build — a build (bun scripts/build.ts in the app workspace) is needed only after app/src or app config changes; run it before publishing", siteApplicationContentPath)
	}
	return nil
}

type siteBuildQualityIssue struct {
	Severity     string `json:"severity"`
	Category     string `json:"category"`
	Target       string `json:"target"`
	Message      string `json:"message"`
	SuggestedFix string `json:"suggestedFix"`
}

type siteBuildQualityDocument struct {
	GeneratedAt         string                  `json:"generatedAt"`
	BlockingIssueCount  int                     `json:"blockingIssueCount"`
	Issues              []siteBuildQualityIssue `json:"issues"`
	PostBuildNormalized bool                    `json:"postBuildNormalized"`
}

type siteBuildQualitySummary struct {
	Status     string
	IssueCount int
	Lines      []string
	Path       string
}

func (service *Service) updateSiteBuildQualitySummary(site *SiteRecord) {
	summary := summarizeSiteBuildQuality(site.HostSourcePath)
	site.QualityStatus = summary.Status
	site.QualityIssueCount = summary.IssueCount
	site.QualitySummary = summary.Lines
	site.QualityReportPath = summary.Path
}

func summarizeSiteBuildQuality(workspacePath string) siteBuildQualitySummary {
	path := filepath.Join(workspacePath, ".internkim", "build-quality.json")
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return siteBuildQualitySummary{
			Status: "missing_report",
			Lines:  []string{"No build-quality.json was found; publish used the latest fresh dist snapshot."},
			Path:   path,
		}
	}
	var quality siteBuildQualityDocument
	if errorValue := json.Unmarshal(document, &quality); errorValue != nil {
		return siteBuildQualitySummary{
			Status: "invalid_report",
			Lines:  []string{"build-quality.json could not be parsed; publish used the latest fresh dist snapshot."},
			Path:   path,
		}
	}
	qualityInformation, errorValue := os.Stat(path)
	if errorValue != nil {
		return siteBuildQualitySummary{
			Status: "missing_report",
			Lines:  []string{"No build-quality.json was found; publish used the latest fresh dist snapshot."},
			Path:   path,
		}
	}
	latestSourceModTime, errorValue := latestFrontendSourceModTime(filepath.Join(workspacePath, "app"))
	if errorValue == nil && latestSourceModTime.After(qualityInformation.ModTime()) {
		return siteBuildQualitySummary{
			Status:     "stale_report",
			IssueCount: len(quality.Issues),
			Lines:      siteBuildQualityLines(quality.Issues),
			Path:       path,
		}
	}
	if len(quality.Issues) > 0 {
		return siteBuildQualitySummary{
			Status:     "needs_improvement",
			IssueCount: len(quality.Issues),
			Lines:      siteBuildQualityLines(quality.Issues),
			Path:       path,
		}
	}
	return siteBuildQualitySummary{
		Status: "passed",
		Lines:  []string{},
		Path:   path,
	}
}

func siteBuildQualityLines(issues []siteBuildQualityIssue) []string {
	lines := []string{}
	for _, issue := range issues {
		target := firstNonEmpty(strings.TrimSpace(issue.Target), "site")
		message := strings.TrimSpace(issue.Message)
		if message == "" {
			message = strings.TrimSpace(issue.SuggestedFix)
		}
		if message == "" {
			message = strings.TrimSpace(issue.Category)
		}
		if message == "" {
			continue
		}
		lines = append(lines, target+": "+message)
		if len(lines) >= 3 {
			return lines
		}
	}
	return lines
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
	if frontendSourceTopLevelPathIsIgnored(relativePath) {
		return true
	}
	for _, component := range strings.Split(filepath.Clean(relativePath), string(os.PathSeparator)) {
		switch component {
		case "dist", "node_modules":
			return true
		}
	}
	return false
}

// frontendSourceTopLevelPathIsIgnored excludes the top-level app/public
// directory and app/DESIGN.md from staleness checks — content-only edits
// there (e.g. site-content.json, DESIGN.md) never require a rebuild.
// DESIGN.md is applied to theme.css at publish time (see
// applySiteDesignTheme), not baked into the Vite build, matching Blueclaw's
// own pathIsSiteDesignOrControlFile classification of DESIGN.md as a
// no-rebuild-needed control file. This is a prefix check rather than an
// any-component match so a nested directory named "public" deeper in app/src
// is still tracked.
func frontendSourceTopLevelPathIsIgnored(relativePath string) bool {
	cleanRelativePath := filepath.ToSlash(filepath.Clean(relativePath))
	if cleanRelativePath == siteDesignDocumentPath {
		return true
	}
	return cleanRelativePath == "public" || strings.HasPrefix(cleanRelativePath, "public/")
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
	if errorValue := service.switchCurrentSiteVersion(site, versionID); errorValue != nil {
		return errorValue
	}
	if _, errorValue := service.runCommand(ctx, "chown", "-R", "internkim-site:internkim-site", service.sitePath(site.SiteID)); errorValue != nil {
		return errorValue
	}
	if _, errorValue := service.runCommand(ctx, "systemctl", "daemon-reload"); errorValue != nil {
		return errorValue
	}
	return service.reconcileSitePocketBaseRuntime(ctx, site, versionID)
}

func (service *Service) reconcileSitePocketBaseRuntime(ctx context.Context, site *SiteRecord, versionID string) error {
	if !service.siteVersionHasPocketBaseBackend(site, versionID) {
		_ = service.stopSitePocketBaseRuntime(ctx, site)
		return nil
	}
	if _, errorValue := service.runCommand(ctx, "systemctl", "disable", siteServiceName(site.SiteID)); errorValue != nil {
		return errorValue
	}
	if _, errorValue := service.runCommand(ctx, "systemctl", "restart", siteServiceName(site.SiteID)); errorValue != nil {
		return fmt.Errorf("systemctl restart %s failed: %w", siteServiceName(site.SiteID), errorValue)
	}
	service.markSitePocketBaseRunning(site.SiteID, true)
	return nil
}

func (service *Service) stopSitePocketBaseRuntime(ctx context.Context, site *SiteRecord) error {
	if _, errorValue := service.runCommand(ctx, "systemctl", "disable", "--now", siteServiceName(site.SiteID)); errorValue != nil {
		log.Printf("disable static site %s pocketbase runtime failed: %v", site.SiteID, errorValue)
		return errorValue
	}
	service.markSitePocketBaseRunning(site.SiteID, false)
	return nil
}

func (service *Service) siteVersionHasPocketBaseBackend(site *SiteRecord, versionID string) bool {
	versionPath := service.sitePublishedVersionPath(site, versionID)
	return directoryHasOperationalFiles(filepath.Join(versionPath, "pb_migrations")) ||
		directoryHasOperationalFiles(filepath.Join(versionPath, "pb_hooks"))
}

func (service *Service) reconcilePublishedSitePocketBaseRuntimes(ctx context.Context) {
	disabledCount := 0
	for _, site := range service.sites {
		if site.Status != SiteStatusPublished || strings.TrimSpace(site.CurrentVersionID) == "" {
			continue
		}
		if service.siteVersionHasPocketBaseBackend(site, site.CurrentVersionID) {
			continue
		}
		if service.stopSitePocketBaseRuntime(ctx, site) == nil {
			disabledCount++
		}
	}
	if disabledCount > 0 {
		log.Printf("reconciled static site runtimes: disabled pocketbase for %d site(s)", disabledCount)
	}
}

func (service *Service) probeSiteReadiness(ctx context.Context, site *SiteRecord, versionID string) error {
	deadline := time.Now().Add(3 * time.Second)
	lastResult := siteReadinessProbeResult{}
	for {
		lastResult = service.probeSiteIndex(ctx, site, versionID)
		if lastResult.isReady() {
			return nil
		}
		if time.Now().After(deadline) {
			return siteReadinessProbeError(lastResult)
		}
		if errorValue := waitForSiteReadinessRetry(ctx, 100*time.Millisecond); errorValue != nil {
			return errorValue
		}
	}
}

func (service *Service) probeSiteIndex(ctx context.Context, site *SiteRecord, versionID string) siteReadinessProbeResult {
	host := service.siteReadinessProbeHost(site)
	request := httptest.NewRequest(http.MethodGet, "https://"+host+"/", nil)
	request.Host = host
	probe := siteReadinessProbe{SiteID: site.SiteID, VersionID: versionID}
	request = request.WithContext(context.WithValue(ctx, siteReadinessProbeContextKey{}, probe))
	response := httptest.NewRecorder()
	service.withSiteGateway(http.NotFoundHandler()).ServeHTTP(response, request)
	return siteReadinessProbeResult{StatusCode: response.Code, Body: response.Body.String()}
}

func (service *Service) siteReadinessProbeHost(site *SiteRecord) string {
	parsedURL, errorValue := url.Parse(site.PublishedURL)
	if errorValue == nil && strings.TrimSpace(parsedURL.Host) != "" {
		return parsedURL.Host
	}
	return site.Slug + "." + service.deviceHost()
}

func (result siteReadinessProbeResult) isReady() bool {
	return result.StatusCode == http.StatusOK && strings.TrimSpace(result.Body) != ""
}

func siteReadinessProbeError(result siteReadinessProbeResult) error {
	return fmt.Errorf("site readiness probe failed: observed status %d with index body length %d", result.StatusCode, len(strings.TrimSpace(result.Body)))
}

func waitForSiteReadinessRetry(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
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

func (service *Service) switchCurrentSiteVersion(site *SiteRecord, versionID string) error {
	currentPath := filepath.Join(service.sitePath(site.SiteID), "current")
	versionPath := service.sitePublishedVersionPath(site, versionID)
	return replaceSymlink(currentPath, versionPath)
}

func replaceSymlink(path string, target string) error {
	_ = os.Remove(path)
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return errorValue
	}
	return os.Symlink(target, path)
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
	_ = os.RemoveAll(service.siteOwnerProjectPath(site))
	_ = os.RemoveAll(service.siteProjectAliasHostPath(site))
	_ = os.RemoveAll(service.siteProjectStorageHostPath(site.SiteID))
	_ = os.RemoveAll(site.HostSourcePath)
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

func (service *Service) prepareSiteWorkspace(ctx context.Context, site *SiteRecord, content *siteContent) error {
	if errorValue := os.MkdirAll(filepath.Dir(site.HostSourcePath), 0o770); errorValue != nil {
		return errorValue
	}
	if errorValue := os.Chmod(filepath.Dir(site.HostSourcePath), 0o770); errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(filepath.Join(site.HostSourcePath, "app", "src"), 0o770); errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(filepath.Join(site.HostSourcePath, "pocketbase", "pb_migrations"), 0o770); errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(filepath.Join(site.HostSourcePath, "pocketbase", "pb_hooks"), 0o770); errorValue != nil {
		return errorValue
	}
	if errorValue := service.writeSiteWorkspaceTemplate(site, content); errorValue != nil {
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
		mode := os.FileMode(0o660)
		if information.IsDir() || information.Mode()&0o111 != 0 {
			mode = 0o770
		}
		return os.Chmod(path, mode)
	})
}

func materializeDirectory(sourceRoot string, targetRoot string) error {
	return filepath.Walk(sourceRoot, func(sourcePath string, information os.FileInfo, walkError error) error {
		if walkError != nil {
			return walkError
		}
		relativePath, errorValue := filepath.Rel(sourceRoot, sourcePath)
		if errorValue != nil || relativePath == "." {
			return errorValue
		}
		targetPath := filepath.Join(targetRoot, relativePath)
		if information.IsDir() {
			return os.MkdirAll(targetPath, information.Mode())
		}
		if !information.Mode().IsRegular() {
			return nil
		}
		if errorValue := os.MkdirAll(filepath.Dir(targetPath), 0o755); errorValue != nil {
			return errorValue
		}
		return copyRegularFile(sourcePath, targetPath)
	})
}

func (service *Service) writeSiteWorkspaceTemplate(site *SiteRecord, content *siteContent) error {
	files := service.siteScaffoldFiles(site, content)
	for _, file := range files {
		path := filepath.Join(site.HostSourcePath, file.Path)
		if isRegularFile(path) && !file.Overwrite {
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
	Path      string
	Document  string
	Overwrite bool
}

func (service *Service) siteWorkspaceMetadata(site *SiteRecord) string {
	document, errorValue := json.MarshalIndent(siteWorkspaceMetadata{
		SiteID:         site.SiteID,
		Slug:           site.Slug,
		Title:          site.Title,
		PublishedURL:   site.PublishedURL,
		PreviewURL:     site.PreviewURL,
		Platform:       site.Platform,
		ConversationID: site.ConversationID,
		DraftPath:      site.DraftPath,
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
	PublishedURL string              `json:"publishedURL,omitempty"`
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
	PublishedURL string `json:"publishedURL,omitempty"`
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
		PublishedURL: siteVisiblePublishedURL(site),
		Revisions:    service.siteRevisionEntries(site, string(output)),
	}, nil
}

func (service *Service) siteRevisionEntries(site *SiteRecord, output string) []siteRevisionEntry {
	versionIDs := service.siteVersionIDs(site)
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
		PublishedURL: siteVisiblePublishedURL(site),
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
	if isDirectory(service.sitePublishedVersionPath(site, revision)) || isDirectory(service.siteVersionPath(site.SiteID, revision)) {
		return revision, nil
	}
	shortCommit := shortSiteCommit(revision)
	for _, versionID := range service.siteVersionIDs(site) {
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

func (service *Service) siteVersionIDs(site *SiteRecord) []string {
	entries, errorValue := os.ReadDir(filepath.Join(service.sitePublishedRootPath(site), "versions"))
	if errorValue != nil {
		entries, errorValue = os.ReadDir(filepath.Join(service.sitePath(site.SiteID), "versions"))
	}
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

func (service *Service) siteListResponse(ctx context.Context, checkLive bool) []*SiteRecord {
	sites := service.siteList()
	probeCount := 0
	for index, site := range sites {
		preparedSite, errorValue := service.prepareSiteWorkspaceForResponse(site)
		if errorValue != nil {
			log.Printf("site response workspace repair failed for %s: %v", site.SiteID, errorValue)
			preparedSite = site
		}
		responseSite := siteAPIResponse(preparedSite)
		if checkLive && responseSite.Status == SiteStatusPublished && probeCount < 20 {
			responseSite.LiveHTTPStatus = service.probePublishedSiteHTTPStatus(ctx, responseSite)
			probeCount++
		}
		sites[index] = responseSite
	}
	return sites
}

func siteListShouldCheckLive(request *http.Request) bool {
	if request == nil {
		return false
	}
	value := strings.ToLower(strings.TrimSpace(request.URL.Query().Get("checkLive")))
	return value == "true" || value == "1"
}

func (service *Service) probePublishedSiteHTTPStatus(ctx context.Context, site *SiteRecord) int {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	host := service.siteReadinessProbeHost(site)
	request := httptest.NewRequest(http.MethodGet, "https://"+host+"/", nil)
	request.Host = host
	request = request.WithContext(ctx)
	response := httptest.NewRecorder()
	service.withSiteGateway(http.NotFoundHandler()).ServeHTTP(response, request)
	return response.Code
}

func siteVisiblePublishedURL(site *SiteRecord) string {
	if site == nil || site.Status != SiteStatusPublished {
		return ""
	}
	return site.PublishedURL
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

func (service *Service) prepareSiteWorkspaceForResponse(site *SiteRecord) (*SiteRecord, error) {
	if site == nil {
		return nil, nil
	}
	if errorValue := service.migrateSiteSourceToStaffCircle(site); errorValue != nil {
		return nil, errorValue
	}
	refreshedSite := service.findSiteByID(site.SiteID)
	if refreshedSite != nil {
		return refreshedSite, nil
	}
	return site, nil
}

func (service *Service) storeSite(site *SiteRecord) error {
	if site == nil {
		return nil
	}
	site.PublishedURL = service.sitePublishedURL(site.Slug)
	site.TLSStatus = service.siteTLSStatus()
	if site.SourceWorkspacePath == "" {
		site.SourceWorkspacePath = siteDraftWorkspacePath(site.SiteID, site.Slug, "")
	}
	if site.WorkspacePath == "" {
		site.WorkspacePath = siteProjectAliasWorkspacePath(site.SiteID, site.Slug)
	}
	if site.HostSourcePath == "" {
		site.HostSourcePath = service.siteSourceLedgerPath(site.SiteID)
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
	if strings.TrimSpace(site.WorkspacePath) == "" || isLegacySiteWorkspacePath(site.WorkspacePath, site.SiteID) {
		site.WorkspacePath = siteProjectAliasWorkspacePath(site.SiteID, site.Slug)
	}
	if strings.TrimSpace(site.SourceWorkspacePath) == "" || isLegacySiteDraftWorkspacePath(site.SourceWorkspacePath, site.SiteID) {
		site.SourceWorkspacePath = siteDraftWorkspacePath(site.SiteID, site.Slug, "")
	}
	if strings.TrimSpace(site.DraftPath) == "" || isLegacySiteDraftWorkspacePath(site.DraftPath, site.SiteID) {
		site.DraftPath = site.SourceWorkspacePath
	}
	if strings.TrimSpace(site.AppWorkspacePath) == "" || isLegacySiteAppWorkspacePath(site.AppWorkspacePath, site.SiteID) {
		site.AppWorkspacePath = filepath.ToSlash(filepath.Join(site.SourceWorkspacePath, "app"))
	}
	if strings.TrimSpace(site.HostSourcePath) == "" || strings.HasPrefix(strings.TrimSpace(site.HostSourcePath), service.Configuration.BlueclawWorkspacePath) {
		site.HostSourcePath = service.siteSourceLedgerPath(site.SiteID)
	}
	if errorValue := service.ensureSiteWorkspaceAlias(site); errorValue != nil {
		return errorValue
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
	sitePersistenceMutex.Lock()
	defer sitePersistenceMutex.Unlock()
	document, errorValue := service.sitesDocument()
	if errorValue != nil {
		return errorValue
	}
	path := service.siteRegistryPath()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return errorValue
	}
	return os.WriteFile(path, append(document, '\n'), 0o600)
}

func (service *Service) sitesDocument() ([]byte, error) {
	sites := []*SiteRecord{}
	service.mutex.Lock()
	defer service.mutex.Unlock()
	for _, site := range service.sites {
		copiedSite := *site
		sites = append(sites, &copiedSite)
	}
	sort.Slice(sites, func(leftIndex int, rightIndex int) bool {
		return sites[leftIndex].CreatedAt.Before(sites[rightIndex].CreatedAt)
	})
	return json.MarshalIndent(siteStateDocument{Sites: sites}, "", "  ")
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
			reconcileInterruptedSiteStatus(site)
			service.sites[site.SiteID] = site
		}
	}
}

func reconcileInterruptedSiteStatus(site *SiteRecord) {
	if site.Status != SiteStatusPublishing {
		return
	}
	site.Status = SiteStatusFailed
	site.LastError = "publish interrupted before completion; republish to recover"
	site.UpdatedAt = time.Now().UTC()
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
		if siteHoldsSitePort(site) {
			usedPorts[site.Port] = true
		}
	}
	return usedPorts
}

func (service *Service) ensureSitePortNotTakenByAnotherSite(site *SiteRecord) error {
	if site.Port != 0 && !service.sitePortHeldByAnotherSite(site) {
		return nil
	}
	port, errorValue := service.allocateSitePort()
	if errorValue != nil {
		return errorValue
	}
	site.Port = port
	return nil
}

func (service *Service) sitePortHeldByAnotherSite(site *SiteRecord) bool {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	for _, otherSite := range service.sites {
		if otherSite.SiteID == site.SiteID {
			continue
		}
		if otherSite.Port == site.Port && siteHoldsSitePort(otherSite) {
			return true
		}
	}
	return false
}

func siteHoldsSitePort(site *SiteRecord) bool {
	if site.Status == SiteStatusDeleted {
		return false
	}
	if site.Status == SiteStatusFailed && strings.TrimSpace(site.CurrentVersionID) == "" {
		return false
	}
	return true
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

func (service *Service) siteOwnerProjectPath(site *SiteRecord) string {
	if site != nil && strings.TrimSpace(site.OwnerIdentity.PersonID) != "" {
		return filepath.Join(service.Configuration.BlueclawWorkspacePath, "private", "people", site.OwnerIdentity.PersonID, "sites", site.SiteID)
	}
	if site != nil && strings.TrimSpace(site.CreatedBy.PersonID) != "" {
		return filepath.Join(service.Configuration.BlueclawWorkspacePath, "private", "people", site.CreatedBy.PersonID, "sites", site.SiteID)
	}
	if site != nil && strings.TrimSpace(site.SiteID) != "" {
		return filepath.Join(service.Configuration.BlueclawWorkspacePath, "sites", site.SiteID)
	}
	return filepath.Join(service.Configuration.BlueclawWorkspacePath, "sites", "unknown")
}

func (service *Service) siteProjectStorageHostPath(siteID string) string {
	if strings.TrimSpace(siteID) == "" {
		return ""
	}
	return filepath.Join(service.Configuration.BlueclawWorkspacePath, "circles", "staff", "sites", siteIDStorageDirectoryName, strings.TrimSpace(siteID))
}

func (service *Service) siteProjectAliasHostPath(site *SiteRecord) string {
	if site == nil {
		return ""
	}
	aliasName := siteWorkspaceAliasName(site.SiteID, site.Slug)
	if aliasName == "" {
		return ""
	}
	return filepath.Join(service.Configuration.BlueclawWorkspacePath, "circles", "staff", "sites", aliasName)
}

func (service *Service) ensureSiteWorkspaceAlias(site *SiteRecord) error {
	if site == nil || strings.TrimSpace(site.SiteID) == "" {
		return nil
	}
	storagePath := service.siteProjectStorageHostPath(site.SiteID)
	aliasPath := service.siteProjectAliasHostPath(site)
	if storagePath == "" || aliasPath == "" || storagePath == aliasPath {
		return nil
	}
	if errorValue := service.ensureSiteWorkspaceStoragePath(site.SiteID); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureStaffCircleDirectory(filepath.Dir(aliasPath)); errorValue != nil {
		return errorValue
	}
	if targetPath, errorValue := os.Readlink(aliasPath); errorValue == nil {
		if filepath.Clean(filepath.Join(filepath.Dir(aliasPath), targetPath)) == filepath.Clean(storagePath) || filepath.Clean(targetPath) == filepath.Clean(storagePath) {
			return nil
		}
		return fmt.Errorf("site workspace alias already points elsewhere: %s", aliasPath)
	}
	if information, errorValue := os.Lstat(aliasPath); errorValue == nil {
		if information.IsDir() {
			if errorValue := replaceSiteWorkspaceAliasDirectory(aliasPath, storagePath); errorValue != nil {
				return errorValue
			}
		} else {
			return fmt.Errorf("site workspace alias path already exists: %s", aliasPath)
		}
	}
	relativeTargetPath, errorValue := filepath.Rel(filepath.Dir(aliasPath), storagePath)
	if errorValue != nil {
		return errorValue
	}
	return os.Symlink(relativeTargetPath, aliasPath)
}

func (service *Service) ensureSiteWorkspaceStoragePath(siteID string) error {
	staffSitesPath := filepath.Join(service.Configuration.BlueclawWorkspacePath, "circles", "staff", "sites")
	for _, directoryPath := range []string{
		staffSitesPath,
		filepath.Join(staffSitesPath, siteIDStorageDirectoryName),
		service.siteProjectStorageHostPath(siteID),
	} {
		if errorValue := ensureStaffCircleDirectory(directoryPath); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func replaceSiteWorkspaceAliasDirectory(aliasPath string, storagePath string) error {
	_ = os.Chmod(aliasPath, os.ModeSetgid|0o770)
	healStaffCirclePermissions(aliasPath)
	if directoryHasEntries(aliasPath) {
		if errorValue := copyDirectory(aliasPath, storagePath); errorValue != nil {
			return errorValue
		}
		return os.RemoveAll(aliasPath)
	}
	return os.Remove(aliasPath)
}

func (service *Service) sitePublishedRootPath(site *SiteRecord) string {
	return service.sitePath(site.SiteID)
}

func (service *Service) sitePublishedVersionPath(site *SiteRecord, versionID string) string {
	return filepath.Join(service.sitePublishedRootPath(site), "versions", versionID)
}

func (service *Service) sitePreviewPath(site *SiteRecord, previewID string) string {
	return filepath.Join(service.sitePublishedRootPath(site), "previews", strings.TrimSpace(previewID))
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

func (service *Service) sitePreviewURL(site *SiteRecord, previewID string) string {
	if site == nil || strings.TrimSpace(site.PublishedURL) == "" || strings.TrimSpace(previewID) == "" {
		return ""
	}
	return strings.TrimRight(site.PublishedURL, "/") + "/__preview/" + url.PathEscape(strings.TrimSpace(previewID))
}

func (service *Service) clearSitePreview(site *SiteRecord) {
	if site == nil || strings.TrimSpace(site.PreviewID) == "" {
		return
	}
	_ = os.RemoveAll(filepath.Join(service.sitePublishedRootPath(site), "previews"))
	site.PreviewID = ""
	site.PreviewURL = ""
	site.PreviewExpiresAt = time.Time{}
}

func siteServiceName(siteID string) string {
	return "internkim-site@" + strings.TrimSpace(siteID) + ".service"
}

func normalizeSiteSlug(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func siteSlugWithSuffix(baseSlug string, suffix string) string {
	cleanSuffix := strings.Trim(strings.ToLower(strings.TrimSpace(suffix)), "-")
	if cleanSuffix == "" {
		return strings.Trim(baseSlug, "-")
	}
	maxBaseLength := 63 - len(cleanSuffix) - 1
	if maxBaseLength < 1 {
		return cleanSuffix
	}
	cleanBase := strings.Trim(baseSlug, "-")
	if len(cleanBase) > maxBaseLength {
		cleanBase = strings.Trim(cleanBase[:maxBaseLength], "-")
	}
	if cleanBase == "" {
		return cleanSuffix
	}
	return cleanBase + "-" + cleanSuffix
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

func siteProjectStorageWorkspacePath(siteID string) string {
	if strings.TrimSpace(siteID) == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Join(staffCircleSitesWorkspacePath, siteIDStorageDirectoryName, strings.TrimSpace(siteID)))
}

func siteProjectAliasWorkspacePath(siteID string, slug string) string {
	aliasName := siteWorkspaceAliasName(siteID, slug)
	if aliasName == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Join(staffCircleSitesWorkspacePath, aliasName))
}

func siteDraftWorkspacePath(siteID string, slug string, requestedPath string) string {
	canonicalPath := filepath.ToSlash(filepath.Join(siteProjectAliasWorkspacePath(siteID, slug), "draft"))
	cleanRequestedPath := filepath.ToSlash(strings.TrimSpace(requestedPath))
	if cleanRequestedPath == canonicalPath {
		return cleanRequestedPath
	}
	return canonicalPath
}

func siteWorkspaceAliasName(siteID string, slug string) string {
	aliasName := normalizeSiteSlug(slug)
	if aliasName != "" {
		return aliasName
	}
	return strings.TrimSpace(siteID)
}

func siteOwnerProjectWorkspacePath(site *SiteRecord) string {
	if site == nil {
		return ""
	}
	siteID := strings.TrimSpace(site.SiteID)
	if siteID == "" {
		return ""
	}
	return "home/sites/" + siteID
}

func siteOwnerSourceWorkspacePath(site *SiteRecord) string {
	projectPath := siteOwnerProjectWorkspacePath(site)
	if projectPath == "" {
		return ""
	}
	return projectPath + "/draft"
}

type siteSourceFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type siteCreateAPIResponse struct {
	*SiteRecord
	SourceFiles []siteSourceFile `json:"sourceFiles,omitempty"`
}

const siteScaffoldAppManifestPath = ".internkim/scaffold-app-manifest.json"
const siteApplicationContentPath = "app/public/site-content.json"
const siteApplicationContentDistPath = "site-content.json"

func (service *Service) siteScaffoldFiles(site *SiteRecord, content *siteContent) []siteTemplateFile {
	renderedContent := siteContentOrDefault(content, site)
	contentDocument := siteContentJSONDocument(renderedContent)
	appSourceFiles := siteAppScaffoldTemplateFiles(site)
	files := []siteTemplateFile{
		{Path: ".internkim/site.json", Document: service.siteWorkspaceMetadata(site)},
		{Path: ".internkim/idea.md", Document: siteIdeaMarkdown(site)},
		{Path: "DESIGN.md", Document: siteDesignMD(site)},
		{Path: siteApplicationContentPath, Document: contentDocument, Overwrite: content != nil},
		{Path: siteScaffoldAppManifestPath, Document: siteScaffoldAppManifestDocument(appSourceFiles)},
	}
	files = append(files, appSourceFiles...)
	files = append(files, siteScaffoldDistFilesWithRenderedContent(site, contentDocument)...)
	return files
}

// siteScaffoldDistFilesWithRenderedContent mirrors the embedded canonical dist
// into the workspace, overwriting dist/site-content.json with the rendered
// content when the dist ships one so the freshly-created workspace already
// reflects the requested content.
func siteScaffoldDistFilesWithRenderedContent(site *SiteRecord, contentDocument string) []siteTemplateFile {
	distFiles := siteScaffoldDistTemplateFiles(site)
	distContentPath := filepath.ToSlash(filepath.Join("app", "dist", siteApplicationContentDistPath))
	for index, file := range distFiles {
		if file.Path == distContentPath {
			distFiles[index].Document = contentDocument
		}
	}
	return distFiles
}

func siteContentOrDefault(content *siteContent, site *SiteRecord) *siteContent {
	if content != nil {
		return content
	}
	return defaultSiteContent(site)
}

func defaultSiteContent(site *SiteRecord) *siteContent {
	title := firstNonEmpty(strings.TrimSpace(site.Title), site.Slug)
	tagline := strings.TrimSpace(site.Description)
	return &siteContent{
		SiteName: title,
		Tagline:  tagline,
		Sections: []siteContentSection{
			{Title: title, Body: siteContentText(firstNonEmpty(tagline, "This site is ready to customize."))},
		},
	}
}

func siteContentJSONDocument(content *siteContent) string {
	document, errorValue := json.MarshalIndent(content, "", "  ")
	if errorValue != nil {
		return "{}\n"
	}
	return string(document) + "\n"
}

func validateSiteContent(content *siteContent) error {
	if content == nil {
		return errors.New("content is required")
	}
	if strings.TrimSpace(content.SiteName) == "" {
		return errors.New("siteName is required")
	}
	if len(content.Pages) > 0 {
		return validateSiteContentPages(content.Pages)
	}
	if len(content.Blocks) > 0 {
		return validateSiteContentBlocks(content.Blocks)
	}
	if len(content.Sections) == 0 {
		return errors.New("sections or blocks must include at least one entry")
	}
	for index, section := range content.Sections {
		if strings.TrimSpace(section.Title) == "" {
			return fmt.Errorf("sections[%d].title is required", index)
		}
		if strings.TrimSpace(string(section.Body)) == "" {
			return fmt.Errorf("sections[%d].body is required", index)
		}
	}
	return nil
}

var siteContentKnownBackdrops = map[string]bool{"mesh": true, "aurora": true, "grain": true, "grid": true, "dots": true}

func validateSiteContentBlocks(blocks []siteContentBlock) error {
	for blockIndex, block := range blocks {
		variant := strings.TrimSpace(block.Variant)
		if variant == "" {
			return fmt.Errorf("blocks[%d].variant is required", blockIndex)
		}
		if !siteContentKnownBlockVariants[variant] {
			return fmt.Errorf("blocks[%d].variant %q is not a known block variant", blockIndex, variant)
		}
		if backdrop := strings.TrimSpace(block.Backdrop); backdrop != "" && !siteContentKnownBackdrops[backdrop] {
			return fmt.Errorf("blocks[%d].backdrop %q does not exist; choose mesh, aurora, grain, grid, or dots", blockIndex, backdrop)
		}
		if image := strings.TrimSpace(block.Image); strings.HasPrefix(image, "http://") || strings.HasPrefix(image, "https://") {
			return fmt.Errorf("blocks[%d].image hotlinks an external URL; fetch the photo into app/public/images/ with scripts/fetch_image.py and reference /images/<name>", blockIndex)
		}
		for itemIndex, item := range block.Items {
			if strings.TrimSpace(item.Title) == "" {
				return fmt.Errorf("blocks[%d].items[%d].title is required", blockIndex, itemIndex)
			}
			if strings.TrimSpace(string(item.Body)) == "" {
				return fmt.Errorf("blocks[%d].items[%d].body is required", blockIndex, itemIndex)
			}
		}
	}
	return nil
}

// validateSiteApplicationContentFile validates app/public/site-content.json
// against the shared content schema when present. A missing file is not an
// error — legacy sites and sites that only edit app/src never write one.
func validateSiteApplicationContentFile(hostSourcePath string) error {
	document, errorValue := os.ReadFile(filepath.Join(hostSourcePath, siteApplicationContentPath))
	if errorValue != nil {
		return nil
	}
	var content siteContent
	if errorValue := json.Unmarshal(document, &content); errorValue != nil {
		return fmt.Errorf("%s is invalid: %s; fix the JSON and publish again — no build is needed", siteApplicationContentPath, errorValue.Error())
	}
	if errorValue := validateSiteContent(&content); errorValue != nil {
		return fmt.Errorf("%s is invalid: %s; fix the JSON and publish again — no build is needed", siteApplicationContentPath, errorValue.Error())
	}
	return nil
}

func siteScaffoldAppManifestDocument(appSourceFiles []siteTemplateFile) string {
	manifest := map[string]string{}
	for _, file := range appSourceFiles {
		if siteApplicationTopLevelDirectoryIsExcluded(file.Path) {
			continue
		}
		manifest[file.Path] = sha256Hex(file.Document)
	}
	document, errorValue := json.MarshalIndent(manifest, "", "  ")
	if errorValue != nil {
		return "{}\n"
	}
	return string(document) + "\n"
}

// siteAppMatchesScaffoldManifest reports whether a site's editable app/ source
// still matches the scaffold-app-manifest.json recorded at create time — i.e.
// the requester never touched app/src or the build configuration. A pristine
// match lets publish/preview use the canonical embedded dist directly instead
// of requiring a Blueclaw build.
func siteAppMatchesScaffoldManifest(hostSourcePath string) bool {
	manifest, isPresent := readSiteScaffoldAppManifest(hostSourcePath)
	if !isPresent {
		return false
	}
	for relativePath, expectedSHA256 := range manifest {
		actualSHA256, errorValue := fileSHA256Hex(filepath.Join(hostSourcePath, relativePath))
		if errorValue != nil || actualSHA256 != expectedSHA256 {
			return false
		}
	}
	return !siteApplicationHasUntrackedFiles(hostSourcePath, manifest)
}

func readSiteScaffoldAppManifest(hostSourcePath string) (map[string]string, bool) {
	document, errorValue := os.ReadFile(filepath.Join(hostSourcePath, siteScaffoldAppManifestPath))
	if errorValue != nil {
		return nil, false
	}
	var manifest map[string]string
	if errorValue := json.Unmarshal(document, &manifest); errorValue != nil {
		return nil, false
	}
	return manifest, true
}

func siteApplicationHasUntrackedFiles(hostSourcePath string, manifest map[string]string) bool {
	applicationPath := filepath.Join(hostSourcePath, "app")
	foundUntrackedFile := false
	_ = filepath.Walk(applicationPath, func(path string, information os.FileInfo, walkError error) error {
		if walkError != nil || foundUntrackedFile {
			return walkError
		}
		relativePath, relativeError := filepath.Rel(hostSourcePath, path)
		if relativeError != nil || relativePath == "app" {
			return relativeError
		}
		slashRelativePath := filepath.ToSlash(relativePath)
		if information.IsDir() {
			if siteApplicationTopLevelDirectoryIsExcluded(slashRelativePath) {
				return filepath.SkipDir
			}
			return nil
		}
		if _, isTracked := manifest[slashRelativePath]; !isTracked {
			foundUntrackedFile = true
		}
		return nil
	})
	return foundUntrackedFile
}

func siteApplicationTopLevelDirectoryIsExcluded(relativePath string) bool {
	components := strings.SplitN(relativePath, "/", 3)
	if len(components) < 2 {
		return false
	}
	switch components[1] {
	case "dist", "public", "node_modules":
		return true
	default:
		return false
	}
}

func sha256Hex(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func fileSHA256Hex(path string) (string, error) {
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return "", errorValue
	}
	return sha256Hex(string(document)), nil
}

func (service *Service) siteScaffoldSourceFiles(site *SiteRecord, content *siteContent) []siteSourceFile {
	templateFiles := service.siteScaffoldFiles(site, content)
	sourceFiles := make([]siteSourceFile, 0, len(templateFiles))
	for _, file := range templateFiles {
		sourceFiles = append(sourceFiles, siteSourceFile{Path: file.Path, Content: file.Document})
	}
	return sourceFiles
}

func (service *Service) writeSiteCreateRecord(responseWriter http.ResponseWriter, site *SiteRecord, content *siteContent) {
	service.writeJSON(responseWriter, siteCreateAPIResponse{
		SiteRecord:  siteAPIResponse(site),
		SourceFiles: service.siteScaffoldSourceFiles(site, content),
	})
}

func isLegacySiteWorkspacePath(path string, siteID string) bool {
	cleanPath := filepath.ToSlash(strings.TrimSpace(path))
	return strings.HasPrefix(cleanPath, "/workspace/sites/") ||
		cleanPath == filepath.ToSlash(filepath.Join(staffCircleSitesWorkspacePath, strings.TrimSpace(siteID))) ||
		cleanPath == siteProjectStorageWorkspacePath(siteID)
}

func isLegacySiteDraftWorkspacePath(path string, siteID string) bool {
	cleanPath := filepath.ToSlash(strings.TrimSpace(path))
	return strings.HasPrefix(cleanPath, "/workspace/sites/") ||
		cleanPath == filepath.ToSlash(filepath.Join(staffCircleSitesWorkspacePath, strings.TrimSpace(siteID), "draft")) ||
		cleanPath == filepath.ToSlash(filepath.Join(siteProjectStorageWorkspacePath(siteID), "draft"))
}

func isLegacySiteAppWorkspacePath(path string, siteID string) bool {
	cleanPath := filepath.ToSlash(strings.TrimSpace(path))
	return strings.HasPrefix(cleanPath, "/workspace/sites/") ||
		cleanPath == filepath.ToSlash(filepath.Join(staffCircleSitesWorkspacePath, strings.TrimSpace(siteID), "draft", "app")) ||
		cleanPath == filepath.ToSlash(filepath.Join(siteProjectStorageWorkspacePath(siteID), "draft", "app"))
}

func (service *Service) siteSourceLedgerPath(siteID string) string {
	return filepath.Join(filepath.Dir(service.Configuration.SitesRoot), "site-sources", siteID)
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
