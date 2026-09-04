package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

const mcpRequesterMetaKey = "kim.intern/requester"

const (
	mcpMessageCeiling      = 1 << 20
	mcpProtocolErrorParse  = -32700
	mcpProtocolErrorMethod = -32601
	mcpProtocolErrorParams = -32602
	mcpProtocolErrorServer = -32000
)

type mcpMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func (service Service) handleMCP(responseWriter http.ResponseWriter, request *http.Request) {
	body, errorValue := io.ReadAll(http.MaxBytesReader(responseWriter, request.Body, mcpMessageCeiling))
	if errorValue != nil {
		writeMCPError(responseWriter, nil, mcpProtocolErrorParse, "that message is larger than an MCP call carries")
		return
	}

	var message mcpMessage
	if json.Unmarshal(body, &message) != nil {
		writeMCPError(responseWriter, nil, mcpProtocolErrorParse, "this carrier takes one JSON-RPC message at a time")
		return
	}

	switch message.Method {
	case "initialize":
		writeMCPResult(responseWriter, message.ID, initializeResultFor(message.Params))
	case "notifications/initialized", "notifications/cancelled":
		responseWriter.WriteHeader(http.StatusAccepted)
	case "ping":
		writeMCPResult(responseWriter, message.ID, map[string]any{})
	case "tools/list", "tools/call":
		service.carryMCPToTheRecord(request.Context(), responseWriter, message, body)
	default:
		writeMCPError(responseWriter, message.ID, mcpProtocolErrorMethod, "this carrier serves tools only: "+message.Method)
	}
}

func (service Service) carryMCPToTheRecord(
	ctx context.Context,
	responseWriter http.ResponseWriter,
	message mcpMessage,
	body []byte,
) {
	requesterEmail := requesterNamedBy(message.Params)
	if requesterEmail == "" {
		writeMCPError(responseWriter, message.ID, mcpProtocolErrorParams,
			"a tool of the record runs as the person who asked, and this message named nobody in _meta."+mcpRequesterMetaKey)
		return
	}

	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost,
		admindRequesterURL("/record/api/mcp"), bytes.NewReader(body))
	if errorValue != nil {
		writeMCPError(responseWriter, message.ID, mcpProtocolErrorServer, errorValue.Error())
		return
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, errorValue := service.askAdmindAsTheRequester(httpRequest, requesterEmail)
	if errorValue != nil {
		writeMCPError(responseWriter, message.ID, mcpProtocolErrorServer, errorValue.Error())
		return
	}
	defer httpResponse.Body.Close()

	answer, errorValue := io.ReadAll(httpResponse.Body)
	if errorValue != nil {
		writeMCPError(responseWriter, message.ID, mcpProtocolErrorServer, errorValue.Error())
		return
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		writeMCPError(responseWriter, message.ID, mcpProtocolErrorServer,
			"the record refused this message with "+httpResponse.Status+": "+strings.TrimSpace(string(answer)))
		return
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.Write(answer)
}

func requesterNamedBy(params json.RawMessage) string {
	var carried struct {
		Meta map[string]json.RawMessage `json:"_meta"`
	}
	if len(params) == 0 || json.Unmarshal(params, &carried) != nil {
		return ""
	}
	named, isCarried := carried.Meta[mcpRequesterMetaKey]
	if !isCarried {
		return ""
	}
	var email string
	if json.Unmarshal(named, &email) != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(email))
}

func initializeResultFor(params json.RawMessage) map[string]any {
	var asked struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if len(params) > 0 {
		_ = json.Unmarshal(params, &asked)
	}
	return map[string]any{
		"protocolVersion": asked.ProtocolVersion,
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"serverInfo":      map[string]any{"name": "internkim-capabilityd", "version": "1"},
	}
}

func writeMCPResult(responseWriter http.ResponseWriter, id json.RawMessage, result any) {
	writeMCPMessage(responseWriter, map[string]any{
		"jsonrpc": "2.0",
		"id":      identityOrNull(id),
		"result":  result,
	})
}

func writeMCPError(responseWriter http.ResponseWriter, id json.RawMessage, code int, message string) {
	writeMCPMessage(responseWriter, map[string]any{
		"jsonrpc": "2.0",
		"id":      identityOrNull(id),
		"error":   map[string]any{"code": code, "message": message},
	})
}

func writeMCPMessage(responseWriter http.ResponseWriter, message map[string]any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(responseWriter).Encode(message)
}

func identityOrNull(id json.RawMessage) any {
	if len(bytes.TrimSpace(id)) == 0 {
		return nil
	}
	return id
}
