package capabilityd

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type siteAppInput struct {
	SiteID        string          `json:"siteID"`
	SiteReference string          `json:"siteReference"`
	RequestedBy   string          `json:"requestedBy"`
	Requester     siteAppIdentity `json:"requester"`
	CheckLive     bool            `json:"checkLive"`
}

type siteAppRecord struct {
	SiteID              string                `json:"siteID"`
	Slug                string                `json:"slug"`
	Title               string                `json:"title"`
	Description         string                `json:"description"`
	Purpose             string                `json:"purpose"`
	Archetype           string                `json:"archetype"`
	PublishedURL        string                `json:"publishedURL"`
	PreviewURL          string                `json:"previewURL"`
	PreviewID           string                `json:"previewID"`
	PreviewExpiresAt    time.Time             `json:"previewExpiresAt"`
	CurrentVersionID    string                `json:"currentVersionID"`
	Owner               string                `json:"owner"`
	OwnerIdentity       siteAppIdentity       `json:"ownerIdentity"`
	Collaborators       []siteAppCollaborator `json:"collaborators"`
	Status              string                `json:"status"`
	LastError           string                `json:"lastError"`
	WorkspaceHealth     string                `json:"workspaceHealth"`
	SourceWorkspacePath string                `json:"sourceWorkspacePath"`
	AppWorkspacePath    string                `json:"appWorkspacePath"`
	UpdatedAt           time.Time             `json:"updatedAt"`
	LiveHTTPStatus      int                   `json:"liveHTTPStatus"`
	SourceFiles         []siteAppSourceFile   `json:"sourceFiles"`
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

type siteAppSourceFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type siteAppRequestError struct {
	statusCode int
	message    string
}

func (requestError *siteAppRequestError) Error() string {
	return requestError.message
}

func (service Service) invokeSiteAppTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	result, errorValue := service.invokeSiteApp(ctx, request)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if response, isResolutionFailure := siteAppResolutionFailureResponse(request.ToolName, result); isResolutionFailure {
		return response, nil
	}
	projectedResult, errorValue := projectCanonicalSiteAppResult(request, result)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	_, isRegistered := siteAppDescriptor(request.ToolName)
	if !isRegistered {
		return capabilities.ToolInvokeResponse{}, errors.New("site tool descriptor is missing")
	}
	response, errorValue := capabilitySuccessResponse(request.ToolName, "ok", projectedResult)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if len(response.Effects) > 0 {
		siteID, errorValue := siteAppResultID(projectedResult)
		if errorValue != nil {
			return capabilities.ToolInvokeResponse{}, errorValue
		}
		if request.ToolName != "site.create" {
			if errorValue := validateSiteAppResultID(request.Input, siteID); errorValue != nil {
				return capabilities.ToolInvokeResponse{}, errorValue
			}
		}
		response.Status = response.Effects[0].Effect
	}
	return response, nil
}

func siteAppResolutionFailureResponse(toolName string, result json.RawMessage) (capabilities.ToolInvokeResponse, bool) {
	var document struct {
		Status string `json:"status"`
	}
	if json.Unmarshal(result, &document) != nil {
		return capabilities.ToolInvokeResponse{}, false
	}
	status := strings.TrimSpace(document.Status)
	if status != "not_found" && status != "ambiguous" {
		return capabilities.ToolInvokeResponse{}, false
	}
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        toolName,
		Outcome:         capabilities.ToolOutcomeFailed,
		Status:          status,
		IsError:         true,
		ErrorCode:       "site_" + status,
		FailureStage:    "resolution",
		Result:          result,
	}, true
}

func siteAppDescriptor(toolName string) (capabilities.Descriptor, bool) {
	for _, descriptor := range capabilities.SiteAppDescriptors() {
		if descriptor.Name == toolName {
			return descriptor, true
		}
	}
	return capabilities.Descriptor{}, false
}

func projectCanonicalSiteAppResult(request capabilities.ToolInvokeRequest, result json.RawMessage) (json.RawMessage, error) {
	var record siteAppRecord
	if errorValue := json.Unmarshal(result, &record); errorValue != nil {
		return nil, errors.New("site app returned an invalid result")
	}
	if record.SiteID != strings.TrimSpace(record.SiteID) {
		return nil, errors.New("site app returned an invalid siteID")
	}
	switch request.ToolName {
	case "site.create":
		return projectSiteCreateResult(record)
	case "site.status":
		return projectSiteStatusResult(record)
	case "site.preview":
		return projectSitePreviewResult(request.Input, record)
	case "site.publish":
		return projectSitePublishResult(request.Input, request.Transport.SiteSourceBundle, record)
	case "site.delete":
		return projectSiteDeleteResult(request.Input, record)
	default:
		return nil, errors.New("site app tool is not configured: " + request.ToolName)
	}
}

func projectSiteCreateResult(record siteAppRecord) (json.RawMessage, error) {
	if record.SiteID == "" || strings.TrimSpace(record.Slug) == "" || record.Status != "draft" || strings.TrimSpace(record.SourceWorkspacePath) == "" || strings.TrimSpace(record.AppWorkspacePath) == "" || len(record.SourceFiles) == 0 {
		return nil, errors.New("site.create result is missing canonical fields")
	}
	return marshalSiteAppResult(map[string]any{
		"siteID":              record.SiteID,
		"slug":                strings.TrimSpace(record.Slug),
		"title":               record.Title,
		"status":              record.Status,
		"sourceWorkspacePath": strings.TrimSpace(record.SourceWorkspacePath),
		"appWorkspacePath":    strings.TrimSpace(record.AppWorkspacePath),
		"sourceFiles":         record.SourceFiles,
	})
}

func projectSiteStatusResult(record siteAppRecord) (json.RawMessage, error) {
	if record.SiteID == "" || strings.TrimSpace(record.Slug) == "" || !isCanonicalSiteStatus(record.Status) || strings.TrimSpace(record.SourceWorkspacePath) == "" {
		return nil, errors.New("site.status result is missing canonical fields")
	}
	result := map[string]any{
		"siteID":              record.SiteID,
		"slug":                strings.TrimSpace(record.Slug),
		"title":               record.Title,
		"status":              record.Status,
		"sourceWorkspacePath": strings.TrimSpace(record.SourceWorkspacePath),
	}
	setSiteAppResultString(result, "appWorkspacePath", record.AppWorkspacePath)
	setSiteAppResultString(result, "workspaceHealth", record.WorkspaceHealth)
	setSiteAppResultString(result, "lastError", record.LastError)
	setSiteAppResultString(result, "publishedURL", record.PublishedURL)
	setSiteAppResultString(result, "previewURL", record.PreviewURL)
	if !record.UpdatedAt.IsZero() {
		result["updatedAt"] = record.UpdatedAt.UTC().Format(time.RFC3339Nano)
	}
	if record.LiveHTTPStatus != 0 {
		result["liveHTTPStatus"] = record.LiveHTTPStatus
	}
	return marshalSiteAppResult(result)
}

func projectSitePreviewResult(inputDocument json.RawMessage, record siteAppRecord) (json.RawMessage, error) {
	if errorValue := validateSiteAppResultID(inputDocument, record.SiteID); errorValue != nil {
		return nil, errorValue
	}
	if !isCanonicalSiteStatus(record.Status) || strings.TrimSpace(record.SourceWorkspacePath) == "" || strings.TrimSpace(record.PreviewID) == "" || strings.TrimSpace(record.PreviewURL) == "" || record.PreviewExpiresAt.IsZero() {
		return nil, errors.New("site.preview result is missing canonical fields")
	}
	return marshalSiteAppResult(map[string]any{
		"siteID":              record.SiteID,
		"status":              record.Status,
		"sourceWorkspacePath": strings.TrimSpace(record.SourceWorkspacePath),
		"previewID":           strings.TrimSpace(record.PreviewID),
		"previewURL":          strings.TrimSpace(record.PreviewURL),
		"previewExpiresAt":    record.PreviewExpiresAt.UTC().Format(time.RFC3339Nano),
	})
}

func projectSitePublishResult(inputDocument json.RawMessage, sourceBundle *capabilities.SiteSourceBundle, record siteAppRecord) (json.RawMessage, error) {
	if errorValue := validateSiteAppResultID(inputDocument, record.SiteID); errorValue != nil {
		return nil, errorValue
	}
	if sourceBundle == nil {
		return nil, errors.New("site.publish transport is missing source bundle")
	}
	sourceSHA256 := sourceBundle.SHA256
	if !isLowercaseSHA256(sourceSHA256) {
		return nil, errors.New("site.publish source bundle must include a lowercase SHA-256 hash")
	}
	if record.Status != "published" || strings.TrimSpace(record.SourceWorkspacePath) == "" || strings.TrimSpace(record.SourceWorkspacePath) != sourceBundle.WorkspacePath || strings.TrimSpace(record.PublishedURL) == "" || strings.TrimSpace(record.CurrentVersionID) == "" {
		return nil, errors.New("site.publish result is missing canonical fields")
	}
	return marshalSiteAppResult(map[string]any{
		"siteID":              record.SiteID,
		"status":              record.Status,
		"sourceWorkspacePath": strings.TrimSpace(record.SourceWorkspacePath),
		"sourceSHA256":        sourceSHA256,
		"publishedURL":        strings.TrimSpace(record.PublishedURL),
		"currentVersionID":    strings.TrimSpace(record.CurrentVersionID),
	})
}

func projectSiteDeleteResult(inputDocument json.RawMessage, record siteAppRecord) (json.RawMessage, error) {
	if errorValue := validateSiteAppResultID(inputDocument, record.SiteID); errorValue != nil {
		return nil, errorValue
	}
	return marshalSiteAppResult(map[string]any{"siteID": record.SiteID, "deleted": true})
}

func validateSiteAppResultID(inputDocument json.RawMessage, actualSiteID string) error {
	expectedSiteID, errorValue := decodeExactSiteID(inputDocument)
	if errorValue != nil {
		return errorValue
	}
	if strings.TrimSpace(actualSiteID) == "" || strings.TrimSpace(actualSiteID) != expectedSiteID {
		return errors.New("site app result siteID does not match input")
	}
	return nil
}

func siteAppResultID(result json.RawMessage) (string, error) {
	var document struct {
		SiteID string `json:"siteID"`
	}
	if json.Unmarshal(result, &document) != nil || strings.TrimSpace(document.SiteID) == "" {
		return "", errors.New("site app mutation result must include siteID")
	}
	return strings.TrimSpace(document.SiteID), nil
}

func marshalSiteAppResult(result map[string]any) (json.RawMessage, error) {
	document, errorValue := json.Marshal(result)
	return document, errorValue
}

func setSiteAppResultString(result map[string]any, field string, value string) {
	if normalizedValue := strings.TrimSpace(value); normalizedValue != "" {
		result[field] = normalizedValue
	}
}

func isLowercaseSHA256(value string) bool {
	if len(value) != 64 || value != strings.ToLower(value) {
		return false
	}
	_, errorValue := hex.DecodeString(value)
	return errorValue == nil
}

func isCanonicalSiteStatus(status string) bool {
	switch status {
	case "draft", "publishing", "published", "unpublished", "failed":
		return true
	default:
		return false
	}
}

func (service Service) invokeSiteApp(ctx context.Context, request capabilities.ToolInvokeRequest) (json.RawMessage, error) {
	inputDocument, errorValue := siteAppInputWithContext(request.Input, request.Context)
	if errorValue != nil {
		return nil, errorValue
	}
	if siteToolNeedsSourceBundle(request.ToolName) {
		inputDocument, errorValue = siteAppInputWithSourceBundle(inputDocument, request.Transport.SiteSourceBundle)
		if errorValue != nil {
			return nil, errorValue
		}
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
	case "site.delete":
		return service.deleteAdmindSite(ctx, inputDocument, request.Context.IsApprovalContinuation)
	default:
		return nil, errors.New("site app tool is not configured: " + request.ToolName)
	}
}

func siteToolNeedsSourceBundle(toolName string) bool {
	return toolName == "site.preview" || toolName == "site.publish"
}

func siteAppInputWithSourceBundle(document json.RawMessage, sourceBundle *capabilities.SiteSourceBundle) (json.RawMessage, error) {
	if errorValue := validateSiteSourceBundle(sourceBundle); errorValue != nil {
		return nil, errorValue
	}
	payload := map[string]any{}
	if errorValue := json.Unmarshal(document, &payload); errorValue != nil {
		return nil, errorValue
	}
	payload["sourceWorkspacePath"] = sourceBundle.WorkspacePath
	payload["sourceBundleBase64"] = sourceBundle.ContentBase64
	payload["sourceBundleFormat"] = sourceBundle.Format
	payload["sourceSHA256"] = sourceBundle.SHA256
	return json.Marshal(payload)
}

func validateSiteSourceBundle(sourceBundle *capabilities.SiteSourceBundle) error {
	if sourceBundle == nil {
		return errors.New("site source bundle is required")
	}
	if sourceBundle.WorkspacePath == "" || sourceBundle.WorkspacePath != strings.TrimSpace(sourceBundle.WorkspacePath) {
		return errors.New("site source bundle workspacePath is invalid")
	}
	if sourceBundle.Format != "tar.gz" {
		return errors.New("site source bundle format must be tar.gz")
	}
	document, errorValue := base64.StdEncoding.DecodeString(sourceBundle.ContentBase64)
	if errorValue != nil || len(document) == 0 {
		return errors.New("site source bundle content is invalid")
	}
	digest := sha256.Sum256(document)
	if hex.EncodeToString(digest[:]) != sourceBundle.SHA256 {
		return errors.New("site source bundle SHA-256 does not match its content")
	}
	return nil
}

func (service Service) publishAdmindSite(ctx context.Context, inputDocument json.RawMessage) (json.RawMessage, error) {
	siteID, errorValue := decodeExactSiteID(inputDocument)
	if errorValue != nil {
		return nil, errorValue
	}
	return service.postAdmindSite(ctx, "/admin/api/sites/"+url.PathEscape(siteID)+"/publish", inputDocument)
}

func (service Service) previewAdmindSite(ctx context.Context, inputDocument json.RawMessage) (json.RawMessage, error) {
	siteID, errorValue := decodeExactSiteID(inputDocument)
	if errorValue != nil {
		return nil, errorValue
	}
	return service.postAdmindSite(ctx, "/admin/api/sites/"+url.PathEscape(siteID)+"/preview", inputDocument)
}

func (service Service) getAdmindSiteStatus(ctx context.Context, input siteAppInput) (json.RawMessage, error) {
	reference := strings.TrimSpace(input.SiteReference)
	if reference == "" {
		return nil, errors.New("siteReference is required")
	}
	sites, errorValue := service.listMatchingSiteAppRecords(ctx, input)
	if errorValue != nil {
		return nil, errorValue
	}
	return service.siteStatusForMatches(ctx, input, sites)
}

func (service Service) siteStatusForMatches(ctx context.Context, input siteAppInput, sites []siteAppRecord) (json.RawMessage, error) {
	switch len(sites) {
	case 0:
		return json.Marshal(map[string]any{"status": "not_found", "siteReference": input.SiteReference, "candidates": []siteAppRecord{}})
	case 1:
		return service.getAdmindSiteStatusByID(ctx, sites[0].SiteID, input)
	default:
		return json.Marshal(map[string]any{"status": "ambiguous", "candidates": siteAppCandidateSummaries(sites)})
	}
}

func (service Service) getAdmindSiteStatusByID(ctx context.Context, siteID string, input siteAppInput) (json.RawMessage, error) {
	document, errorValue := service.getAdmindSite(ctx, "/admin/api/sites/"+url.PathEscape(siteID))
	if errorValue != nil {
		var requestError *siteAppRequestError
		if errors.As(errorValue, &requestError) && requestError.statusCode == http.StatusNotFound {
			return json.Marshal(map[string]any{"status": "not_found", "siteID": siteID, "candidates": []siteAppRecord{}})
		}
		return nil, errorValue
	}
	var site siteAppRecord
	if errorValue := json.Unmarshal(document, &site); errorValue != nil {
		return nil, errorValue
	}
	if siteAppRecordCanEdit(site, input) {
		return document, nil
	}
	return json.Marshal(map[string]any{"status": "not_found", "siteID": siteID, "candidates": []siteAppRecord{}})
}

func (service Service) deleteAdmindSite(ctx context.Context, inputDocument json.RawMessage, isApprovalContinuation bool) (json.RawMessage, error) {
	siteID, errorValue := decodeExactSiteID(inputDocument)
	if errorValue != nil {
		return nil, errorValue
	}
	admindInput, errorValue := siteDeleteAdmindInput(inputDocument, isApprovalContinuation)
	if errorValue != nil {
		return nil, errorValue
	}
	return service.admindSiteRequest(ctx, http.MethodDelete, "/admin/api/sites/"+url.PathEscape(siteID), admindInput)
}

func siteDeleteAdmindInput(inputDocument json.RawMessage, isApprovalContinuation bool) (json.RawMessage, error) {
	input := map[string]any{}
	if errorValue := json.Unmarshal(inputDocument, &input); errorValue != nil {
		return nil, errorValue
	}
	delete(input, "confirm")
	delete(input, "userConfirmed")
	if isApprovalContinuation {
		input["confirm"] = "DELETE"
		input["userConfirmed"] = true
	}
	return json.Marshal(input)
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
		return nil, &siteAppRequestError{
			statusCode: httpResponse.StatusCode,
			message:    "site app request failed: " + strings.TrimSpace(string(body)),
		}
	}
	var document map[string]json.RawMessage
	if errorValue := json.Unmarshal(body, &document); errorValue != nil || len(document) == 0 {
		return nil, errors.New("site app returned an invalid response document")
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
	if input.SiteID != strings.TrimSpace(input.SiteID) {
		return siteAppInput{}, errors.New("siteID must not have leading or trailing whitespace")
	}
	if input.SiteReference != strings.TrimSpace(input.SiteReference) {
		return siteAppInput{}, errors.New("siteReference must not have leading or trailing whitespace")
	}
	return input, nil
}

func decodeExactSiteID(document json.RawMessage) (string, error) {
	input, errorValue := decodeSiteAppInput(document)
	if errorValue != nil {
		return "", errorValue
	}
	return requireSiteID(input)
}

func requireSiteID(input siteAppInput) (string, error) {
	if input.SiteID == "" {
		return "", errors.New("siteID is required")
	}
	return input.SiteID, nil
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
	reference := strings.TrimSpace(input.SiteReference)
	for _, site := range sites {
		isExactID := strings.TrimSpace(site.SiteID) == reference
		isExactSlug := strings.TrimSpace(site.Slug) == reference
		if !isExactID && !isExactSlug {
			continue
		}
		if !siteAppRecordCanEdit(site, input) {
			continue
		}
		matches = append(matches, site)
	}
	return matches
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
		return false
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
	deleteSiteAppRuntimeIdentity(payload)
	setSiteAppRuntimeString(payload, "requestedBy", siteRequester(toolContext))
	setSiteAppRuntimeString(payload, "platform", strings.TrimSpace(toolContext.Platform))
	setSiteAppRuntimeString(payload, "conversationID", strings.TrimSpace(toolContext.ConversationID))
	setSiteAppRuntimeIdentity(payload, "requester", siteRequesterIdentity(toolContext))
	setSiteAppRuntimeIdentity(payload, "createdBy", siteRequesterIdentity(toolContext))
	setSiteAppRuntimeIdentity(payload, "ownerIdentity", siteRequesterIdentity(toolContext))
	enrichedDocument, errorValue := json.Marshal(payload)
	if errorValue != nil {
		return nil, errorValue
	}
	return enrichedDocument, nil
}

func deleteSiteAppRuntimeIdentity(payload map[string]any) {
	for _, key := range []string{"requestedBy", "platform", "conversationID", "requester", "createdBy", "ownerIdentity"} {
		delete(payload, key)
	}
}

func setSiteAppRuntimeString(payload map[string]any, key string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	payload[key] = value
}

func setSiteAppRuntimeIdentity(payload map[string]any, key string, value map[string]string) {
	if len(value) == 0 {
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
