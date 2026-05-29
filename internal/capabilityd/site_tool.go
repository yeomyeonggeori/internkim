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
	SiteID              string `json:"siteID"`
	Slug                string `json:"slug"`
	Title               string `json:"title"`
	Prompt              string `json:"prompt"`
	DesignBrief         string `json:"designBrief"`
	PrototypeScope      string `json:"prototypeScope"`
	SourceWorkspacePath string `json:"sourceWorkspacePath"`
	SourceBundleBase64  string `json:"sourceBundleBase64"`
	SourceBundleFormat  string `json:"sourceBundleFormat"`
	FromRevision        string `json:"fromRevision"`
	ToRevision          string `json:"toRevision"`
	Revision            string `json:"revision"`
	RequestedBy         string `json:"requestedBy"`
	Platform            string `json:"platform"`
	ConversationID      string `json:"conversationID"`
}

type siteAppRecord struct {
	SiteID         string `json:"siteID"`
	Slug           string `json:"slug"`
	Platform       string `json:"platform"`
	ConversationID string `json:"conversationID"`
	Status         string `json:"status"`
}

type siteAppListResponse struct {
	Sites []siteAppRecord `json:"sites"`
}

func isSiteAppTool(toolName string) bool {
	return strings.HasPrefix(strings.TrimSpace(toolName), "site.app.")
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
	case "site.app.create":
		return service.postAdmindSite(ctx, "/admin/api/sites", inputDocument)
	case "site.app.publish":
		return service.publishAdmindSite(ctx, inputDocument)
	case "site.app.status":
		input, errorValue := decodeSiteAppInput(inputDocument)
		if errorValue != nil {
			return nil, errorValue
		}
		siteID, errorValue := service.resolveAdmindSiteID(ctx, input)
		if errorValue != nil {
			return nil, errorValue
		}
		return service.getAdmindSite(ctx, "/admin/api/sites/"+url.PathEscape(siteID))
	case "site.app.history":
		input, errorValue := decodeSiteAppInput(inputDocument)
		if errorValue != nil {
			return nil, errorValue
		}
		siteID, errorValue := service.resolveAdmindSiteID(ctx, input)
		if errorValue != nil {
			return nil, errorValue
		}
		return service.getAdmindSite(ctx, "/admin/api/sites/"+url.PathEscape(siteID)+"/history")
	case "site.app.diff":
		return service.getAdmindSiteDiff(ctx, inputDocument)
	case "site.app.logs":
		input, errorValue := decodeSiteAppInput(inputDocument)
		if errorValue != nil {
			return nil, errorValue
		}
		siteID, errorValue := service.resolveAdmindSiteID(ctx, input)
		if errorValue != nil {
			return nil, errorValue
		}
		return service.getAdmindSite(ctx, "/admin/api/sites/"+url.PathEscape(siteID)+"/logs")
	case "site.app.rollback":
		return service.postAdmindSiteAction(ctx, inputDocument, "rollback")
	case "site.app.unpublish":
		return service.postAdmindSiteAction(ctx, inputDocument, "unpublish")
	case "site.app.restore":
		return service.postAdmindSiteAction(ctx, inputDocument, "restore")
	case "site.app.delete":
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
	input.Platform = strings.TrimSpace(input.Platform)
	input.ConversationID = strings.TrimSpace(input.ConversationID)
	input.FromRevision = strings.TrimSpace(input.FromRevision)
	input.ToRevision = strings.TrimSpace(input.ToRevision)
	input.Revision = strings.TrimSpace(input.Revision)
	if input.SiteID == "" && input.Slug == "" && input.ConversationID == "" {
		return siteAppInput{}, errors.New("siteID, slug, or conversationID is required")
	}
	return input, nil
}

func (service Service) resolveAdmindSiteID(ctx context.Context, input siteAppInput) (string, error) {
	if strings.TrimSpace(input.SiteID) != "" {
		return strings.TrimSpace(input.SiteID), nil
	}
	listDocument, errorValue := service.getAdmindSite(ctx, "/admin/api/sites")
	if errorValue != nil {
		return "", errorValue
	}
	var listResponse siteAppListResponse
	if errorValue := json.Unmarshal(listDocument, &listResponse); errorValue != nil {
		return "", errorValue
	}
	if siteID := matchSiteAppRecord(listResponse.Sites, input); siteID != "" {
		return siteID, nil
	}
	return "", errors.New("siteID could not be resolved")
}

func matchSiteAppRecord(sites []siteAppRecord, input siteAppInput) string {
	for _, site := range sites {
		if input.Slug != "" && strings.EqualFold(site.Slug, input.Slug) {
			return site.SiteID
		}
	}
	for _, site := range sites {
		if input.ConversationID == "" || site.ConversationID != input.ConversationID {
			continue
		}
		if input.Platform != "" && site.Platform != "" && !strings.EqualFold(site.Platform, input.Platform) {
			continue
		}
		return site.SiteID
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
