package admind

import (
	"encoding/json"
	"io"
	"net/http"
)

const (
	mcpCarryPath      = "/record/api/mcp"
	mcpMessageCeiling = 1 << 20
)

func (service *Service) handleMCPCarry(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(responseWriter, "an MCP message is carried with POST", http.StatusMethodNotAllowed)
		return
	}
	requesterEmail := assertedRequesterEmail(request)
	if requesterEmail == "" {
		http.Error(responseWriter, "an MCP message runs as the person who asked, and this call asserted nobody", http.StatusForbidden)
		return
	}
	client := service.centralPlane()
	if client == nil {
		http.Error(responseWriter, "this device is not attached to a central plane, so the catalog is out of reach", http.StatusServiceUnavailable)
		return
	}

	message, errorValue := io.ReadAll(http.MaxBytesReader(responseWriter, request.Body, mcpMessageCeiling))
	if errorValue != nil {
		http.Error(responseWriter, "that message is larger than an MCP call carries", http.StatusRequestEntityTooLarge)
		return
	}

	answer, errorValue := client.CarryMCPMessage(request.Context(), requesterEmail, json.RawMessage(message))
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(answer.Status)
	responseWriter.Write(answer.Body)
}
