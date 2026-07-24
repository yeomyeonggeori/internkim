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
	SiteReference string `json:"siteReference"`
	Mode          string `json:"mode"`
	Title         string `json:"title"`
}

type siteAppRecord struct {
	SiteID           string          `json:"siteID"`
	Slug             string          `json:"slug"`
	Title            string          `json:"title"`
	Description      string          `json:"description"`
	Purpose          string          `json:"purpose"`
	Archetype        string          `json:"archetype"`
	Owner            string          `json:"owner"`
	OwnerIdentity    siteAppIdentity `json:"ownerIdentity"`
	Status           string          `json:"status"`
	PublishedURL     string          `json:"publishedURL"`
	PreviewURL       string          `json:"previewURL"`
	CurrentVersionID string          `json:"currentVersionID"`
	UpdatedAt        time.Time       `json:"updatedAt"`
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
		if _, errorValue := siteAppResultID(projectedResult); errorValue != nil {
			return capabilities.ToolInvokeResponse{}, errorValue
		}
		if errorValue := validateSiteAppResultReference(request.Input, projectedResult); errorValue != nil {
			return capabilities.ToolInvokeResponse{}, errorValue
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
	switch request.ToolName {
	case "site.serve":
		return projectSiteServeResult(request.Input, request.Transport.SiteSourceBundle, result)
	case "site.list":
		return projectSiteListResult(result)
	case "site.unserve":
		return projectSiteUnserveResult(result)
	default:
		return nil, errors.New("site app tool is not configured: " + request.ToolName)
	}
}

func decodeSiteAppRecord(result json.RawMessage) (siteAppRecord, error) {
	var record siteAppRecord
	if errorValue := json.Unmarshal(result, &record); errorValue != nil {
		return siteAppRecord{}, errors.New("site app returned an invalid result")
	}
	if record.SiteID != strings.TrimSpace(record.SiteID) {
		return siteAppRecord{}, errors.New("site app returned an invalid siteID")
	}
	return record, nil
}

func projectSiteServeResult(inputDocument json.RawMessage, sourceBundle *capabilities.SiteSourceBundle, resultDocument json.RawMessage) (json.RawMessage, error) {
	record, errorValue := decodeSiteAppRecord(resultDocument)
	if errorValue != nil {
		return nil, errorValue
	}
	input, errorValue := decodeSiteAppInput(inputDocument)
	if errorValue != nil {
		return nil, errorValue
	}
	mode := strings.TrimSpace(input.Mode)
	if mode != "preview" && mode != "publish" {
		return nil, errors.New(`site.serve mode must be "preview" or "publish"`)
	}
	if sourceBundle == nil {
		return nil, errors.New("site.serve transport is missing source bundle")
	}
	if !isLowercaseSHA256(sourceBundle.SHA256) {
		return nil, errors.New("site.serve source bundle must include a lowercase SHA-256 hash")
	}
	if record.SiteID == "" || strings.TrimSpace(record.Slug) == "" {
		return nil, errors.New("site.serve result is missing canonical fields")
	}
	result := map[string]any{
		"siteID":       record.SiteID,
		"slug":         strings.TrimSpace(record.Slug),
		"mode":         mode,
		"sourceSHA256": sourceBundle.SHA256,
	}
	if mode == "publish" {
		if record.Status != "published" || strings.TrimSpace(record.PublishedURL) == "" {
			return nil, errors.New("site.serve publish result is missing canonical fields")
		}
		result["publishedURL"] = strings.TrimSpace(record.PublishedURL)
		return marshalSiteAppResult(result)
	}
	if strings.TrimSpace(record.PreviewURL) == "" {
		return nil, errors.New("site.serve preview result is missing canonical fields")
	}
	result["previewURL"] = strings.TrimSpace(record.PreviewURL)
	return marshalSiteAppResult(result)
}

func projectSiteListResult(resultDocument json.RawMessage) (json.RawMessage, error) {
	var listResponse siteAppListResponse
	if errorValue := json.Unmarshal(resultDocument, &listResponse); errorValue != nil {
		return nil, errors.New("site app returned an invalid list result")
	}
	sites := []map[string]any{}
	for _, record := range listResponse.Sites {
		if record.Status == "deleting" || record.Status == "deleted" {
			continue
		}
		if record.SiteID == "" || strings.TrimSpace(record.Slug) == "" || !isCanonicalSiteStatus(record.Status) {
			return nil, errors.New("site.list result contains a record missing canonical fields")
		}
		entry := map[string]any{
			"siteID": record.SiteID,
			"slug":   strings.TrimSpace(record.Slug),
			"title":  record.Title,
			"status": record.Status,
		}
		setSiteAppResultString(entry, "publishedURL", record.PublishedURL)
		if !record.UpdatedAt.IsZero() {
			entry["updatedAt"] = record.UpdatedAt.UTC().Format(time.RFC3339Nano)
		}
		sites = append(sites, entry)
	}
	return marshalSiteAppResult(map[string]any{"sites": sites})
}

func projectSiteUnserveResult(resultDocument json.RawMessage) (json.RawMessage, error) {
	record, errorValue := decodeSiteAppRecord(resultDocument)
	if errorValue != nil {
		return nil, errorValue
	}
	if record.SiteID == "" || strings.TrimSpace(record.Slug) == "" || record.Status != "deleted" {
		return nil, errors.New("site.unserve result is missing canonical fields")
	}
	return marshalSiteAppResult(map[string]any{
		"siteID":   record.SiteID,
		"slug":     strings.TrimSpace(record.Slug),
		"unserved": true,
	})
}

func validateSiteAppResultReference(inputDocument json.RawMessage, result json.RawMessage) error {
	input, errorValue := decodeSiteAppInput(inputDocument)
	if errorValue != nil {
		return errorValue
	}
	reference := strings.TrimSpace(input.SiteReference)
	if reference == "" {
		return nil
	}
	var document struct {
		SiteID string `json:"siteID"`
		Slug   string `json:"slug"`
	}
	if json.Unmarshal(result, &document) != nil {
		return errors.New("site app result is not valid JSON")
	}
	if reference == strings.TrimSpace(document.SiteID) || reference == strings.TrimSpace(document.Slug) {
		return nil
	}
	return errors.New("site app result does not match the requested siteReference")
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
	switch request.ToolName {
	case "site.serve":
		inputDocument, errorValue = siteAppInputWithSourceBundle(inputDocument, request.Transport.SiteSourceBundle)
		if errorValue != nil {
			return nil, errorValue
		}
		return service.postAdmindSite(ctx, "/admin/api/sites/serve", inputDocument)
	case "site.list":
		return service.listAdmindSites(ctx, inputDocument)
	case "site.unserve":
		return service.unserveAdmindSite(ctx, inputDocument, request.Context.IsApprovalContinuation)
	default:
		return nil, errors.New("site app tool is not configured: " + request.ToolName)
	}
}

func siteToolNeedsSourceBundle(toolName string) bool {
	return toolName == "site.serve"
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

func (service Service) listAdmindSites(ctx context.Context, inputDocument json.RawMessage) (json.RawMessage, error) {
	input, errorValue := decodeSiteAppInput(inputDocument)
	if errorValue != nil {
		return nil, errorValue
	}
	allSites, errorValue := service.listSiteAppRecords(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	reference := strings.TrimSpace(input.SiteReference)
	if reference == "" {
		return json.Marshal(siteAppListResponse{Sites: allSites})
	}
	matches := siteAppRecordsMatchingReference(allSites, reference)
	if len(matches) == 0 {
		return json.Marshal(map[string]any{"status": "not_found", "siteReference": reference, "candidates": siteAppCandidateSummaries(limitSiteAppRecords(allSites, 10))})
	}
	return json.Marshal(siteAppListResponse{Sites: matches})
}

func (service Service) unserveAdmindSite(ctx context.Context, inputDocument json.RawMessage, isApprovalContinuation bool) (json.RawMessage, error) {
	input, errorValue := decodeSiteAppInput(inputDocument)
	if errorValue != nil {
		return nil, errorValue
	}
	reference := strings.TrimSpace(input.SiteReference)
	if reference == "" {
		return nil, errors.New("siteReference is required")
	}
	allSites, errorValue := service.listSiteAppRecords(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	matches := siteAppRecordsMatchingReference(allSites, reference)
	switch len(matches) {
	case 0:
		return json.Marshal(map[string]any{"status": "not_found", "siteReference": reference, "candidates": siteAppCandidateSummaries(limitSiteAppRecords(allSites, 10))})
	case 1:
		admindInput, errorValue := siteUnserveAdmindInput(inputDocument, isApprovalContinuation)
		if errorValue != nil {
			return nil, errorValue
		}
		return service.admindSiteRequest(ctx, http.MethodDelete, "/admin/api/sites/"+url.PathEscape(matches[0].SiteID), admindInput)
	default:
		return json.Marshal(map[string]any{"status": "ambiguous", "candidates": siteAppCandidateSummaries(matches)})
	}
}

func siteUnserveAdmindInput(inputDocument json.RawMessage, isApprovalContinuation bool) (json.RawMessage, error) {
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
	if input.SiteReference != strings.TrimSpace(input.SiteReference) {
		return siteAppInput{}, errors.New("siteReference must not have leading or trailing whitespace")
	}
	return input, nil
}

func (service Service) listSiteAppRecords(ctx context.Context) ([]siteAppRecord, error) {
	listDocument, errorValue := service.getAdmindSite(ctx, "/admin/api/sites")
	if errorValue != nil {
		return nil, errorValue
	}
	var listResponse siteAppListResponse
	if errorValue := json.Unmarshal(listDocument, &listResponse); errorValue != nil {
		return nil, errorValue
	}
	return listResponse.Sites, nil
}

func limitSiteAppRecords(sites []siteAppRecord, limit int) []siteAppRecord {
	if len(sites) <= limit {
		return sites
	}
	return sites[:limit]
}

func siteAppRecordsMatchingReference(sites []siteAppRecord, reference string) []siteAppRecord {
	matches := []siteAppRecord{}
	for _, site := range sites {
		if strings.TrimSpace(site.SiteID) == reference || strings.TrimSpace(site.Slug) == reference {
			matches = append(matches, site)
		}
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
