package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type companyInfoSetInput struct {
	Language            string `json:"language"`
	Name                string `json:"name"`
	BrandName           string `json:"brandName"`
	Slogan              string `json:"slogan"`
	Description         string `json:"description"`
	Representative      string `json:"representative"`
	RepresentativeTitle string `json:"representativeTitle"`
	Address             string `json:"address"`
	OfficeAddress       string `json:"officeAddress"`
	Jurisdiction        string `json:"jurisdiction"`
	BankAccount         string `json:"bankAccount"`
	LegalAttributes     string `json:"legalAttributes"`
	FoundedDate         string `json:"foundedDate"`
	Capital             string `json:"capital"`
	FiscalYearEnd       string `json:"fiscalYearEnd"`
	EmployeeCount       int    `json:"employeeCount"`
	Phone               string `json:"phone"`
	Fax                 string `json:"fax"`
	Email               string `json:"email"`
	Website             string `json:"website"`
}

type companyMetricRecordInput struct {
	Metric  string  `json:"metric"`
	Year    int     `json:"year"`
	Quarter int     `json:"quarter"`
	Month   int     `json:"month"`
	Value   float64 `json:"value"`
	Unit    string  `json:"unit"`
	Note    string  `json:"note"`
}

type companyRecordInput struct {
	ID         string `json:"id"`
	Category   string `json:"category"`
	Date       string `json:"date"`
	Title      string `json:"title"`
	Detail     string `json:"detail"`
	Attributes string `json:"attributes"`
}

type companyDocumentRegisterInput struct {
	Kind         string `json:"kind"`
	DocumentType string `json:"documentType"`
	Title        string `json:"title"`
	Counterpart  string `json:"counterpart"`
	Language     string `json:"language"`
	FilePath     string `json:"filePath"`
	Summary      string `json:"summary"`
}

type companyDocumentSearchInput struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

type companyDocumentUpdateInput struct {
	ID          string `json:"id"`
	FilePath    string `json:"filePath"`
	Title       string `json:"title"`
	Counterpart string `json:"counterpart"`
	Summary     string `json:"summary"`
}

func (service Service) invokeCompanyTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	switch strings.TrimSpace(request.ToolName) {
	case "company.info.get":
		return service.invokeCompanyInfoGet(ctx, request)
	case "company.info.set":
		return service.invokeCompanyInfoSet(ctx, request)
	case "company.metric.record":
		return service.invokeCompanyMetricRecord(ctx, request)
	case "company.metric.list":
		return service.invokeCompanyMetricList(ctx, request)
	case "company.record.add", "company.record.update":
		return service.invokeCompanyRecordWrite(ctx, request)
	case "company.record.list":
		return service.invokeCompanyRecordList(ctx, request)
	case "company.record.delete":
		return service.invokeCompanyRecordDelete(ctx, request)
	case "company.document.register":
		return service.invokeCompanyDocumentRegister(ctx, request)
	case "company.document.list":
		return service.invokeCompanyDocumentList(ctx, request)
	case "company.document.search":
		return service.invokeCompanyDocumentSearch(ctx, request)
	case "company.document.update":
		return service.invokeCompanyDocumentUpdate(ctx, request)
	default:
		return capabilities.ToolInvokeResponse{}, fmt.Errorf("company tool is not configured: %s", request.ToolName)
	}
}

func (service Service) invokeCompanyInfoGet(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	var input struct {
		Language string `json:"language"`
	}
	if errorValue := decodeCompanyInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	language := firstNonEmpty(strings.TrimSpace(input.Language), "ko")
	result, errorValue := service.sendCompanyToolRequest(ctx, http.MethodGet, "/admin/api/company-info?language="+url.QueryEscape(language), nil, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return companyToolResponse(request.ToolName, "ok", result), nil
}

func (service Service) invokeCompanyInfoSet(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	var input companyInfoSetInput
	if errorValue := decodeCompanyInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	payload := map[string]any{
		"language":            firstNonEmpty(strings.TrimSpace(input.Language), "ko"),
		"name":                input.Name,
		"brandName":           input.BrandName,
		"slogan":              input.Slogan,
		"description":         input.Description,
		"representative":      input.Representative,
		"representativeTitle": input.RepresentativeTitle,
		"address":             input.Address,
		"officeAddress":       input.OfficeAddress,
		"jurisdiction":        input.Jurisdiction,
		"bankAccount":         input.BankAccount,
		"foundedDate":         input.FoundedDate,
		"capital":             input.Capital,
		"fiscalYearEnd":       input.FiscalYearEnd,
		"employeeCount":       input.EmployeeCount,
		"phone":               input.Phone,
		"fax":                 input.Fax,
		"email":               input.Email,
		"website":             input.Website,
	}
	legalAttributes, errorValue := decodeJSONObjectString(input.LegalAttributes, "legalAttributes")
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if len(legalAttributes) > 0 {
		payload["legalAttributes"] = legalAttributes
	}
	result, errorValue := service.sendCompanyToolRequest(ctx, http.MethodPut, "/admin/api/company-info", payload, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return companyToolResponse(request.ToolName, "saved", result), nil
}

func (service Service) invokeCompanyMetricRecord(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	var input companyMetricRecordInput
	if errorValue := decodeCompanyInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	result, errorValue := service.sendCompanyToolRequest(ctx, http.MethodPost, "/admin/api/company-metrics", input, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return companyToolResponse(request.ToolName, "recorded", result), nil
}

func (service Service) invokeCompanyMetricList(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	var input struct {
		Metric   string `json:"metric"`
		FromYear int    `json:"fromYear"`
		ToYear   int    `json:"toYear"`
	}
	if errorValue := decodeCompanyInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	query := url.Values{}
	if strings.TrimSpace(input.Metric) != "" {
		query.Set("metric", strings.TrimSpace(input.Metric))
	}
	if input.FromYear > 0 {
		query.Set("fromYear", strconv.Itoa(input.FromYear))
	}
	if input.ToYear > 0 {
		query.Set("toYear", strconv.Itoa(input.ToYear))
	}
	result, errorValue := service.sendCompanyToolRequest(ctx, http.MethodGet, "/admin/api/company-metrics?"+query.Encode(), nil, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return companyToolResponse(request.ToolName, "ok", result), nil
}

func (service Service) invokeCompanyRecordWrite(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	var input companyRecordInput
	if errorValue := decodeCompanyInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	payload := map[string]any{
		"id":       input.ID,
		"category": input.Category,
		"date":     input.Date,
		"title":    input.Title,
		"detail":   input.Detail,
	}
	attributes, errorValue := decodeJSONObjectString(input.Attributes, "attributes")
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if len(attributes) > 0 {
		payload["attributes"] = attributes
	}
	method := http.MethodPost
	status := "added"
	if request.ToolName == "company.record.update" {
		method = http.MethodPut
		status = "updated"
	}
	result, errorValue := service.sendCompanyToolRequest(ctx, method, "/admin/api/company-records", payload, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return companyToolResponse(request.ToolName, status, result), nil
}

func (service Service) invokeCompanyRecordList(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	var input struct {
		Category string `json:"category"`
		Query    string `json:"query"`
	}
	if errorValue := decodeCompanyInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	query := url.Values{}
	if strings.TrimSpace(input.Category) != "" {
		query.Set("category", strings.TrimSpace(input.Category))
	}
	if strings.TrimSpace(input.Query) != "" {
		query.Set("query", strings.TrimSpace(input.Query))
	}
	result, errorValue := service.sendCompanyToolRequest(ctx, http.MethodGet, "/admin/api/company-records?"+query.Encode(), nil, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return companyToolResponse(request.ToolName, "ok", result), nil
}

func (service Service) invokeCompanyRecordDelete(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	var input struct {
		ID string `json:"id"`
	}
	if errorValue := decodeCompanyInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	result, errorValue := service.sendCompanyToolRequest(ctx, http.MethodDelete, "/admin/api/company-records?id="+url.QueryEscape(strings.TrimSpace(input.ID)), nil, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return companyToolResponse(request.ToolName, "deleted", result), nil
}

func (service Service) invokeCompanyDocumentRegister(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	var input companyDocumentRegisterInput
	if errorValue := decodeCompanyInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	payload := map[string]any{
		"kind":         input.Kind,
		"documentType": input.DocumentType,
		"title":        input.Title,
		"counterpart":  input.Counterpart,
		"language":     input.Language,
		"filePath":     input.FilePath,
		"summary":      input.Summary,
	}
	if embedding := service.companyTextEmbedding(ctx, input.Summary); len(embedding) > 0 {
		payload["summaryEmbedding"] = embedding
	}
	result, errorValue := service.sendCompanyToolRequest(ctx, http.MethodPost, "/admin/api/company-documents", payload, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return companyToolResponse(request.ToolName, "registered", result), nil
}

func (service Service) invokeCompanyDocumentList(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	var input struct {
		Type        string `json:"type"`
		Counterpart string `json:"counterpart"`
		Query       string `json:"query"`
	}
	if errorValue := decodeCompanyInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	query := url.Values{}
	if strings.TrimSpace(input.Type) != "" {
		query.Set("type", strings.TrimSpace(input.Type))
	}
	if strings.TrimSpace(input.Counterpart) != "" {
		query.Set("counterpart", strings.TrimSpace(input.Counterpart))
	}
	if strings.TrimSpace(input.Query) != "" {
		query.Set("query", strings.TrimSpace(input.Query))
	}
	result, errorValue := service.sendCompanyToolRequest(ctx, http.MethodGet, "/admin/api/company-documents?"+query.Encode(), nil, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return companyToolResponse(request.ToolName, "ok", result), nil
}

func (service Service) invokeCompanyDocumentSearch(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	var input companyDocumentSearchInput
	if errorValue := decodeCompanyInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	payload := map[string]any{
		"query": input.Query,
		"limit": input.Limit,
	}
	if embedding := service.companyTextEmbedding(ctx, input.Query); len(embedding) > 0 {
		payload["queryEmbedding"] = embedding
	}
	result, errorValue := service.sendCompanyToolRequest(ctx, http.MethodPost, "/admin/api/company-documents/search", payload, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return companyToolResponse(request.ToolName, "ok", result), nil
}

func (service Service) invokeCompanyDocumentUpdate(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	var input companyDocumentUpdateInput
	if errorValue := decodeCompanyInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	payload := map[string]any{
		"id":          input.ID,
		"filePath":    input.FilePath,
		"title":       input.Title,
		"counterpart": input.Counterpart,
		"summary":     input.Summary,
	}
	if strings.TrimSpace(input.Summary) != "" {
		if embedding := service.companyTextEmbedding(ctx, input.Summary); len(embedding) > 0 {
			payload["summaryEmbedding"] = embedding
		}
	}
	result, errorValue := service.sendCompanyToolRequest(ctx, http.MethodPut, "/admin/api/company-documents", payload, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return companyToolResponse(request.ToolName, "updated", result), nil
}

func (service Service) companyTextEmbedding(ctx context.Context, text string) []float64 {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	response, errorValue := service.createEmbedding(ctx, EmbeddingRequest{Input: text})
	if errorValue != nil {
		return nil
	}
	if len(response.Embedding) > 0 {
		return response.Embedding
	}
	if len(response.Embeddings) > 0 {
		return response.Embeddings[0]
	}
	return nil
}

func decodeCompanyInput(document json.RawMessage, target any) error {
	if len(document) == 0 {
		return nil
	}
	return json.Unmarshal(document, target)
}

func decodeJSONObjectString(text string, fieldName string) (map[string]string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	var values map[string]string
	if errorValue := json.Unmarshal([]byte(text), &values); errorValue != nil {
		return nil, fmt.Errorf("%s must be a JSON object string of label-to-value pairs, e.g. {\"사업자등록번호\": \"123-45-67890\"}: %w", fieldName, errorValue)
	}
	return values, nil
}

func (service Service) sendCompanyToolRequest(ctx context.Context, method string, path string, payload any, requesterEmail string) (json.RawMessage, error) {
	var body io.Reader
	if payload != nil {
		document, errorValue := json.Marshal(payload)
		if errorValue != nil {
			return nil, errorValue
		}
		body = bytes.NewReader(document)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	httpRequest, errorValue := http.NewRequestWithContext(ctx, method, strings.TrimRight(service.Configuration.AdmindBaseURL, "/")+path, body)
	if errorValue != nil {
		return nil, errorValue
	}
	if payload != nil {
		httpRequest.Header.Set("Content-Type", "application/json")
	}
	setCalendarRequesterEmailHeader(httpRequest, requesterEmail)
	httpResponse, errorValue := service.httpClient().Do(httpRequest)
	if errorValue != nil {
		return nil, errorValue
	}
	defer httpResponse.Body.Close()
	responseBody, readError := io.ReadAll(httpResponse.Body)
	if readError != nil {
		return nil, readError
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("company tool failed: %s", strings.TrimSpace(string(responseBody)))
	}
	return json.RawMessage(responseBody), nil
}

func companyToolResponse(toolName string, status string, result json.RawMessage) capabilities.ToolInvokeResponse {
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        toolName,
		Status:          status,
		Result:          result,
	}
}
