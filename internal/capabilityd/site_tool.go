package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type siteAppInput struct {
	SiteID              string          `json:"siteID"`
	Slug                string          `json:"slug"`
	Title               string          `json:"title"`
	Prompt              string          `json:"prompt"`
	DesignBrief         string          `json:"designBrief"`
	PrototypeScope      string          `json:"prototypeScope"`
	Description         string          `json:"description"`
	Idea                string          `json:"idea"`
	Purpose             string          `json:"purpose"`
	Audience            string          `json:"audience"`
	Archetype           string          `json:"archetype"`
	DomainKeywords      []string        `json:"domainKeywords"`
	SourceWorkspacePath string          `json:"sourceWorkspacePath"`
	SourceBundleBase64  string          `json:"sourceBundleBase64"`
	SourceBundleFormat  string          `json:"sourceBundleFormat"`
	PreviewID           string          `json:"previewID"`
	FromRevision        string          `json:"fromRevision"`
	ToRevision          string          `json:"toRevision"`
	Revision            string          `json:"revision"`
	RequestedBy         string          `json:"requestedBy"`
	Requester           siteAppIdentity `json:"requester"`
	CreatedBy           siteAppIdentity `json:"createdBy"`
	OwnerIdentity       siteAppIdentity `json:"ownerIdentity"`
	Platform            string          `json:"platform"`
	ConversationID      string          `json:"conversationID"`
	Scope               string          `json:"scope"`
	CheckLive           bool            `json:"checkLive"`
}

type siteAppRecord struct {
	SiteID           string                `json:"siteID"`
	Slug             string                `json:"slug"`
	Title            string                `json:"title"`
	Description      string                `json:"description"`
	Purpose          string                `json:"purpose"`
	Archetype        string                `json:"archetype"`
	PublishedURL     string                `json:"publishedURL"`
	PreviewURL       string                `json:"previewURL"`
	Owner            string                `json:"owner"`
	OwnerIdentity    siteAppIdentity       `json:"ownerIdentity"`
	CreatedBy        siteAppIdentity       `json:"createdBy"`
	Collaborators    []siteAppCollaborator `json:"collaborators"`
	Platform         string                `json:"platform"`
	ConversationID   string                `json:"conversationID"`
	Status           string                `json:"status"`
	LastError        string                `json:"lastError"`
	WorkspaceHealth  string                `json:"workspaceHealth"`
	DraftPath        string                `json:"draftPath"`
	AppWorkspacePath string                `json:"appWorkspacePath"`
	UpdatedAt        time.Time             `json:"updatedAt"`
	LiveHTTPStatus   int                   `json:"liveHTTPStatus"`
}

type siteAppListResponse struct {
	Sites []siteAppRecord `json:"sites"`
}

type siteAppIdentity struct {
	PersonID       string `json:"personID,omitempty"`
	Platform       string `json:"platform,omitempty"`
	PlatformUserID string `json:"platformUserID,omitempty"`
	DisplayName    string `json:"displayName,omitempty"`
}

type siteAppCollaborator struct {
	PersonID       string    `json:"personID,omitempty"`
	PlatformUserID string    `json:"platformUserID,omitempty"`
	Role           string    `json:"role"`
	GrantedBy      string    `json:"grantedBy,omitempty"`
	GrantedAt      time.Time `json:"grantedAt,omitempty"`
}

func (service Service) invokeSiteAppTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	result, errorValue := service.invokeSiteApp(ctx, request)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        request.ToolName,
		Status:          "ok",
		Result:          result,
	}, nil
}

func (service Service) invokeSiteApp(ctx context.Context, request capabilities.ToolInvokeRequest) (json.RawMessage, error) {
	inputDocument, errorValue := siteAppInputWithContext(request.Input, request.Context)
	if errorValue != nil {
		return nil, errorValue
	}
	switch request.ToolName {
	case "site.create":
		return service.postAdmindSite(ctx, "/admin/api/sites", inputDocument)
	case "site.publish":
		return service.publishAdmindSite(ctx, inputDocument)
	case "site.preview":
		return service.previewAdmindSite(ctx, inputDocument)
	case "site.status":
		input, errorValue := decodeSiteAppInput(inputDocument)
		if errorValue != nil {
			return nil, errorValue
		}
		return service.getAdmindSiteStatus(ctx, input)
	case "site.history":
		input, errorValue := decodeSiteAppInput(inputDocument)
		if errorValue != nil {
			return nil, errorValue
		}
		siteID, errorValue := service.resolveAdmindSiteID(ctx, input)
		if errorValue != nil {
			return nil, errorValue
		}
		return service.getAdmindSite(ctx, "/admin/api/sites/"+url.PathEscape(siteID)+"/history")
	case "site.diff":
		return service.getAdmindSiteDiff(ctx, inputDocument)
	case "site.logs":
		input, errorValue := decodeSiteAppInput(inputDocument)
		if errorValue != nil {
			return nil, errorValue
		}
		siteID, errorValue := service.resolveAdmindSiteID(ctx, input)
		if errorValue != nil {
			return nil, errorValue
		}
		return service.getAdmindSite(ctx, "/admin/api/sites/"+url.PathEscape(siteID)+"/logs")
	case "site.rollback":
		return service.postAdmindSiteAction(ctx, inputDocument, "rollback")
	case "site.unpublish":
		return service.postAdmindSiteAction(ctx, inputDocument, "unpublish")
	case "site.restore":
		return service.postAdmindSiteAction(ctx, inputDocument, "restore")
	case "site.repair":
		return service.postAdmindSiteAction(ctx, inputDocument, "repair")
	case "site.delete":
		return service.deleteAdmindSite(ctx, inputDocument)
	default:
		return nil, errors.New("site app tool is not configured: " + request.ToolName)
	}
}

func (service Service) getAdmindSiteDiff(ctx context.Context, inputDocument json.RawMessage) (json.RawMessage, error) {
	input, errorValue := decodeSiteAppInput(inputDocument)
	if errorValue != nil {
		return nil, errorValue
	}
	siteID, errorValue := service.resolveAdmindSiteID(ctx, input)
	if errorValue != nil {
		return nil, errorValue
	}
	query := url.Values{}
	if input.FromRevision != "" {
		query.Set("fromRevision", input.FromRevision)
	}
	if input.ToRevision != "" {
		query.Set("toRevision", input.ToRevision)
	}
	path := "/admin/api/sites/" + url.PathEscape(siteID) + "/diff"
	if encodedQuery := query.Encode(); encodedQuery != "" {
		path += "?" + encodedQuery
	}
	return service.getAdmindSite(ctx, path)
}

func (service Service) publishAdmindSite(ctx context.Context, inputDocument json.RawMessage) (json.RawMessage, error) {
	input, errorValue := decodeSiteAppInput(inputDocument)
	if errorValue != nil {
		return nil, errorValue
	}
	siteID := input.SiteID
	if siteID == "" {
		resolvedSiteID, errorValue := service.resolveAdmindSiteID(ctx, input)
		if errorValue != nil {
			return nil, errorValue
		}
		siteID = resolvedSiteID
	}
	return service.postAdmindSite(ctx, "/admin/api/sites/"+url.PathEscape(siteID)+"/publish", inputDocument)
}

func (service Service) previewAdmindSite(ctx context.Context, inputDocument json.RawMessage) (json.RawMessage, error) {
	input, errorValue := decodeSiteAppInput(inputDocument)
	if errorValue != nil {
		return nil, errorValue
	}
	siteID := input.SiteID
	if siteID == "" {
		resolvedSiteID, errorValue := service.resolveAdmindSiteID(ctx, input)
		if errorValue != nil {
			return nil, errorValue
		}
		siteID = resolvedSiteID
	}
	return service.postAdmindSite(ctx, "/admin/api/sites/"+url.PathEscape(siteID)+"/preview", inputDocument)
}

func (service Service) getAdmindSiteStatus(ctx context.Context, input siteAppInput) (json.RawMessage, error) {
	if input.Scope == "mine" {
		sites, errorValue := service.listMatchingSiteAppRecords(ctx, input)
		if errorValue != nil {
			return nil, errorValue
		}
		return json.Marshal(map[string]any{"status": "ok", "sites": siteAppCompactRecords(sites)})
	}
	if input.SiteID != "" {
		return service.getAdmindSiteStatusByID(ctx, strings.TrimSpace(input.SiteID), input)
	}
	if input.Slug != "" {
		sites, errorValue := service.listMatchingSiteAppRecords(ctx, input)
		if errorValue != nil {
			return nil, errorValue
		}
		switch len(sites) {
		case 0:
			return json.Marshal(map[string]any{"status": "not_found", "slug": input.Slug, "candidates": []siteAppRecord{}, "nextOperation": siteStatusNotFoundNextOperation})
		case 1:
			return service.getAdmindSiteStatusByID(ctx, sites[0].SiteID, input)
		default:
			return json.Marshal(map[string]any{"status": "ambiguous", "candidates": siteAppCandidateSummaries(sites), "nextOperation": siteStatusAmbiguousNextOperation})
		}
	}
	sites, errorValue := service.listMatchingSiteAppRecords(ctx, input)
	if errorValue != nil {
		return nil, errorValue
	}
	switch len(sites) {
	case 0:
		return json.Marshal(map[string]any{"status": "not_found", "candidates": []siteAppRecord{}, "nextOperation": siteStatusNotFoundNextOperation})
	case 1:
		return service.getAdmindSite(ctx, "/admin/api/sites/"+url.PathEscape(sites[0].SiteID))
	default:
		return json.Marshal(map[string]any{"status": "ambiguous", "candidates": siteAppCandidateSummaries(sites), "nextOperation": siteStatusAmbiguousNextOperation})
	}
}

// siteStatusNotFoundNextOperation and siteStatusAmbiguousNextOperation are
// deterministic routing hints, not user-facing prose: they tell the model
// what to do next instead of leaving it to re-poll site.status with no new
// information.
const (
	siteStatusNotFoundNextOperation  = "no site matched. Call site.create with a slug and the other descriptive fields to create one. Calling site.status again first will not change this result."
	siteStatusAmbiguousNextOperation = "multiple sites matched. Ask the user which one, or resolve using slug or siteID from the candidates, then retry with siteID set."
)

func (service Service) getAdmindSiteStatusByID(ctx context.Context, siteID string, input siteAppInput) (json.RawMessage, error) {
	document, errorValue := service.getAdmindSite(ctx, "/admin/api/sites/"+url.PathEscape(siteID))
	if errorValue != nil {
		return nil, errorValue
	}
	var site siteAppRecord
	if errorValue := json.Unmarshal(document, &site); errorValue != nil {
		return nil, errorValue
	}
	if siteAppRecordCanEdit(site, input) {
		return document, nil
	}
	publicDocument := map[string]any{}
	if errorValue := json.Unmarshal(document, &publicDocument); errorValue != nil {
		return nil, errorValue
	}
	for _, field := range []string{"workspacePath", "sourceWorkspacePath", "appWorkspacePath", "draftPath", "hostSourcePath"} {
		delete(publicDocument, field)
	}
	publicDocument["canEdit"] = false
	publicDocument["access"] = "viewer"
	return json.Marshal(publicDocument)
}

func (service Service) postAdmindSiteAction(ctx context.Context, inputDocument json.RawMessage, action string) (json.RawMessage, error) {
	input, errorValue := decodeSiteAppInput(inputDocument)
	if errorValue != nil {
		return nil, errorValue
	}
	siteID, errorValue := service.resolveAdmindSiteID(ctx, input)
	if errorValue != nil {
		return nil, errorValue
	}
	return service.postAdmindSite(ctx, "/admin/api/sites/"+url.PathEscape(siteID)+"/"+action, inputDocument)
}

func (service Service) deleteAdmindSite(ctx context.Context, inputDocument json.RawMessage) (json.RawMessage, error) {
	input, errorValue := decodeSiteAppInput(inputDocument)
	if errorValue != nil {
		return nil, errorValue
	}
	siteID, errorValue := service.resolveAdmindSiteID(ctx, input)
	if errorValue != nil {
		return nil, errorValue
	}
	return service.admindSiteRequest(ctx, http.MethodDelete, "/admin/api/sites/"+url.PathEscape(siteID), inputDocument)
}

func (service Service) getAdmindSite(ctx context.Context, path string) (json.RawMessage, error) {
	return service.admindSiteRequest(ctx, http.MethodGet, path, nil)
}

func (service Service) postAdmindSite(ctx context.Context, path string, inputDocument json.RawMessage) (json.RawMessage, error) {
	return service.admindSiteRequest(ctx, http.MethodPost, path, inputDocument)
}

func (service Service) admindSiteRequest(ctx context.Context, method string, path string, inputDocument json.RawMessage) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	requestURL := strings.TrimRight(service.Configuration.AdmindBaseURL, "/") + path
	httpRequest, errorValue := http.NewRequestWithContext(ctx, method, requestURL, bytes.NewReader(inputDocument))
	if errorValue != nil {
		return nil, errorValue
	}
	if len(bytes.TrimSpace(inputDocument)) != 0 {
		httpRequest.Header.Set("Content-Type", "application/json")
	}
	httpResponse, errorValue := service.httpClient().Do(httpRequest)
	if errorValue != nil {
		return nil, errorValue
	}
	defer httpResponse.Body.Close()
	body, readError := io.ReadAll(httpResponse.Body)
	if readError != nil {
		return nil, readError
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("site app request failed: %s", strings.TrimSpace(string(body)))
	}
	return json.RawMessage(body), nil
}

func decodeSiteAppInput(document json.RawMessage) (siteAppInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return siteAppInput{}, errors.New("site app input is required")
	}
	var input siteAppInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return siteAppInput{}, errorValue
	}
	input.SiteID = strings.TrimSpace(input.SiteID)
	input.Slug = strings.TrimSpace(input.Slug)
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	input.Idea = strings.TrimSpace(input.Idea)
	input.Purpose = strings.TrimSpace(input.Purpose)
	input.Audience = strings.TrimSpace(input.Audience)
	input.Archetype = strings.TrimSpace(input.Archetype)
	input.Platform = strings.TrimSpace(input.Platform)
	input.ConversationID = strings.TrimSpace(input.ConversationID)
	input.Scope = strings.TrimSpace(input.Scope)
	input.FromRevision = strings.TrimSpace(input.FromRevision)
	input.ToRevision = strings.TrimSpace(input.ToRevision)
	input.Revision = strings.TrimSpace(input.Revision)
	if input.Scope == "" {
		input.Scope = "conversation"
	}
	if input.SiteID == "" && input.Slug == "" && input.ConversationID == "" && input.Scope != "mine" {
		return siteAppInput{}, errors.New("siteID, slug, or conversationID is required")
	}
	return input, nil
}

func (service Service) resolveAdmindSiteID(ctx context.Context, input siteAppInput) (string, error) {
	if strings.TrimSpace(input.SiteID) != "" {
		return strings.TrimSpace(input.SiteID), nil
	}
	sites, errorValue := service.listMatchingSiteAppRecords(ctx, input)
	if errorValue != nil {
		return "", errorValue
	}
	switch len(sites) {
	case 1:
		return sites[0].SiteID, nil
	case 0:
		return "", errors.New("siteID could not be resolved")
	default:
		return "", errors.New("siteID could not be resolved unambiguously")
	}
}

func (service Service) listMatchingSiteAppRecords(ctx context.Context, input siteAppInput) ([]siteAppRecord, error) {
	path := "/admin/api/sites"
	query := url.Values{}
	if input.CheckLive {
		query.Set("checkLive", "true")
	}
	if encodedQuery := query.Encode(); encodedQuery != "" {
		path += "?" + encodedQuery
	}
	listDocument, errorValue := service.getAdmindSite(ctx, path)
	if errorValue != nil {
		return nil, errorValue
	}
	var listResponse siteAppListResponse
	if errorValue := json.Unmarshal(listDocument, &listResponse); errorValue != nil {
		return nil, errorValue
	}
	return matchSiteAppRecords(listResponse.Sites, input), nil
}

func matchSiteAppRecords(sites []siteAppRecord, input siteAppInput) []siteAppRecord {
	matches := []siteAppRecord{}
	if input.Slug != "" {
		for _, site := range sites {
			if strings.EqualFold(site.Slug, input.Slug) {
				return []siteAppRecord{site}
			}
		}
		return matches
	}
	for _, site := range sites {
		if input.Scope == "mine" {
			if siteAppRecordCanEdit(site, input) {
				matches = append(matches, site)
			}
			continue
		}
		if input.ConversationID == "" || site.ConversationID != input.ConversationID {
			continue
		}
		if input.Platform != "" && site.Platform != "" && !strings.EqualFold(site.Platform, input.Platform) {
			continue
		}
		if !siteAppRecordCanEdit(site, input) {
			continue
		}
		matches = append(matches, site)
	}
	return matches
}

func siteAppCompactRecords(sites []siteAppRecord) []map[string]any {
	records := []map[string]any{}
	for _, site := range sites {
		record := map[string]any{
			"siteID":          site.SiteID,
			"slug":            site.Slug,
			"title":           site.Title,
			"status":          site.Status,
			"lastError":       site.LastError,
			"workspaceHealth": site.WorkspaceHealth,
			"updatedAt":       site.UpdatedAt,
		}
		if site.Status == "published" && strings.TrimSpace(site.PublishedURL) != "" {
			record["publishedURL"] = site.PublishedURL
		}
		if site.LiveHTTPStatus != 0 {
			record["liveHTTPStatus"] = site.LiveHTTPStatus
		}
		records = append(records, record)
	}
	return records
}

func siteAppCandidateSummaries(sites []siteAppRecord) []map[string]any {
	summaries := []map[string]any{}
	for _, site := range sites {
		summaries = append(summaries, map[string]any{
			"siteID":       site.SiteID,
			"slug":         site.Slug,
			"title":        site.Title,
			"description":  site.Description,
			"purpose":      site.Purpose,
			"archetype":    site.Archetype,
			"owner":        firstNonEmptySiteString(site.OwnerIdentity.DisplayName, site.Owner),
			"publishedURL": site.PublishedURL,
			"previewURL":   site.PreviewURL,
			"updatedAt":    site.UpdatedAt,
		})
	}
	return summaries
}

func siteAppRecordCanEdit(site siteAppRecord, input siteAppInput) bool {
	if siteAppIdentityEmpty(site.OwnerIdentity) && strings.TrimSpace(site.Owner) == "" {
		return true
	}
	if siteAppIdentityMatches(site.OwnerIdentity, input.Requester) {
		return true
	}
	if siteAppIdentityStringMatches(input.RequestedBy, site.Owner) {
		return true
	}
	for _, collaborator := range site.Collaborators {
		if !siteAppCollaboratorCanEdit(collaborator.Role) {
			continue
		}
		if collaborator.PersonID != "" && collaborator.PersonID == input.Requester.PersonID {
			return true
		}
		if collaborator.PlatformUserID != "" && collaborator.PlatformUserID == input.Requester.PlatformUserID {
			return true
		}
		if siteAppIdentityStringMatches(input.RequestedBy, collaborator.PersonID) || siteAppIdentityStringMatches(input.RequestedBy, collaborator.PlatformUserID) {
			return true
		}
	}
	return false
}

func siteAppIdentityEmpty(identity siteAppIdentity) bool {
	return strings.TrimSpace(identity.PersonID) == "" &&
		strings.TrimSpace(identity.PlatformUserID) == "" &&
		strings.TrimSpace(identity.DisplayName) == ""
}

func siteAppIdentityMatches(owner siteAppIdentity, requester siteAppIdentity) bool {
	if owner.PersonID != "" && requester.PersonID != "" && owner.PersonID == requester.PersonID {
		return true
	}
	if owner.PlatformUserID != "" && requester.PlatformUserID != "" && owner.PlatformUserID == requester.PlatformUserID {
		return owner.Platform == "" || requester.Platform == "" || strings.EqualFold(owner.Platform, requester.Platform)
	}
	return false
}

func siteAppCollaboratorCanEdit(role string) bool {
	normalizedRole := strings.ToLower(strings.TrimSpace(role))
	return normalizedRole == "editor" || normalizedRole == "owner"
}

func siteAppIdentityStringMatches(left string, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	return left != "" && right != "" && strings.EqualFold(left, right)
}

func firstNonEmptySiteString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func siteAppInputWithContext(document json.RawMessage, toolContext capabilities.ToolInvokeContext) (json.RawMessage, error) {
	payload := map[string]any{}
	if len(bytes.TrimSpace(document)) != 0 {
		if errorValue := json.Unmarshal(document, &payload); errorValue != nil {
			return nil, errorValue
		}
	}
	setSiteAppDefault(payload, "requestedBy", siteRequester(toolContext))
	setSiteAppDefault(payload, "platform", strings.TrimSpace(toolContext.Platform))
	setSiteAppDefault(payload, "conversationID", strings.TrimSpace(toolContext.ConversationID))
	setSiteAppDefaultDocument(payload, "requester", siteRequesterIdentity(toolContext))
	setSiteAppDefaultDocument(payload, "createdBy", siteRequesterIdentity(toolContext))
	setSiteAppDefaultDocument(payload, "ownerIdentity", siteRequesterIdentity(toolContext))
	enrichedDocument, errorValue := json.Marshal(payload)
	if errorValue != nil {
		return nil, errorValue
	}
	return enrichedDocument, nil
}

func setSiteAppDefault(payload map[string]any, key string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	currentValue, exists := payload[key]
	if exists && strings.TrimSpace(fmt.Sprint(currentValue)) != "" {
		return
	}
	payload[key] = value
}

func setSiteAppDefaultDocument(payload map[string]any, key string, value map[string]string) {
	if len(value) == 0 {
		return
	}
	currentValue, exists := payload[key]
	if exists && fmt.Sprint(currentValue) != "" {
		return
	}
	payload[key] = value
}

func siteRequester(toolContext capabilities.ToolInvokeContext) string {
	for _, value := range []string{
		toolContext.RequesterEmail,
		toolContext.RequesterPersonID,
		toolContext.RequesterPlatformUserID,
		toolContext.RequesterName,
	} {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func siteRequesterIdentity(toolContext capabilities.ToolInvokeContext) map[string]string {
	identity := map[string]string{}
	if strings.TrimSpace(toolContext.RequesterPersonID) != "" {
		identity["personID"] = strings.TrimSpace(toolContext.RequesterPersonID)
	}
	if strings.TrimSpace(toolContext.Platform) != "" {
		identity["platform"] = strings.TrimSpace(toolContext.Platform)
	}
	if strings.TrimSpace(toolContext.RequesterPlatformUserID) != "" {
		identity["platformUserID"] = strings.TrimSpace(toolContext.RequesterPlatformUserID)
	}
	if strings.TrimSpace(toolContext.RequesterName) != "" {
		identity["displayName"] = strings.TrimSpace(toolContext.RequesterName)
	}
	return identity
}
