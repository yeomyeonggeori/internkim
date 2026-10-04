package capabilityd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

func hostDiagnosticsURL(input json.RawMessage) (string, error) {
	var fields map[string]json.RawMessage
	if errorValue := json.Unmarshal(input, &fields); errorValue != nil {
		return "", errorValue
	}
	var view string
	if errorValue := json.Unmarshal(fields["view"], &view); errorValue != nil {
		return "", errorValue
	}
	query := url.Values{}
	for name, value := range fields {
		if name == "view" {
			continue
		}
		var text string
		if json.Unmarshal(value, &text) == nil {
			query.Set(name, text)
			continue
		}
		query.Set(name, string(value))
	}
	return admindRequesterURL("/api/v1/diagnostics/"+url.PathEscape(view)) + "?" + query.Encode(), nil
}

func (service Service) invokeHostDiagnosticsTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	address, errorValue := hostDiagnosticsURL(request.Input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return service.answerAdmindRequesterTool(request, httpRequest, wrapDiagnosticDocument)
}

func wrapDiagnosticDocument(document json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(struct {
		Document string `json:"document"`
	}{Document: string(document)})
}
