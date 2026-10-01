package admind

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

type publicToolNamedWorkspacePaths struct {
	Attachments []string `json:"attachments"`
}

func (service *Service) carriedWorkspaceFiles(ctx context.Context, actor capabilities.ActorContext, input json.RawMessage) ([]capabilities.WorkspaceFile, int, error) {
	namedPaths := workspacePathsNamedInPublicToolInput(input)
	if len(namedPaths) == 0 {
		return nil, 0, nil
	}
	personID := strings.TrimSpace(actor.PersonID)
	if personID == "" {
		return nil, http.StatusForbidden, errors.New("this caller has no person the workspace knows")
	}
	carriedFiles := make([]capabilities.WorkspaceFile, 0, len(namedPaths))
	for _, agentPath := range namedPaths {
		content, status, errorValue := service.readWorkspaceFileAsPerson(ctx, personID, agentPath)
		if errorValue != nil {
			return nil, status, fmt.Errorf("%s: %w", agentPath, errorValue)
		}
		digest := sha256.Sum256(content)
		carriedFiles = append(carriedFiles, capabilities.WorkspaceFile{
			WorkspacePath: agentPath,
			Filename:      path.Base(agentPath),
			ContentBase64: base64.StdEncoding.EncodeToString(content),
			SHA256:        hex.EncodeToString(digest[:]),
		})
	}
	return carriedFiles, 0, nil
}

func workspacePathsNamedInPublicToolInput(input json.RawMessage) []string {
	if len(input) == 0 {
		return nil
	}
	var named publicToolNamedWorkspacePaths
	if errorValue := json.Unmarshal(input, &named); errorValue != nil {
		return nil
	}
	namedPaths := make([]string, 0, len(named.Attachments))
	for _, agentPath := range named.Attachments {
		if trimmedPath := strings.TrimSpace(agentPath); trimmedPath != "" {
			namedPaths = append(namedPaths, trimmedPath)
		}
	}
	return namedPaths
}

func (service *Service) readWorkspaceFileAsPerson(ctx context.Context, personID string, agentPath string) ([]byte, int, error) {
	downloadURL := strings.TrimRight(service.Configuration.BlueclawBaseURL, "/") + "/admin/api/workspace/download?" + workspaceReadQuery(personID, agentPath)
	downloadRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if errorValue != nil {
		return nil, http.StatusInternalServerError, errorValue
	}
	response, errorValue := service.httpClient().Do(downloadRequest)
	if errorValue != nil {
		return nil, http.StatusBadGateway, errorValue
	}
	defer response.Body.Close()
	content, errorValue := io.ReadAll(response.Body)
	if errorValue != nil {
		return nil, http.StatusBadGateway, errorValue
	}
	if response.StatusCode != http.StatusOK {
		return nil, workspaceReadRefusalStatus(response.StatusCode), errors.New(strings.TrimSpace(string(content)))
	}
	return content, 0, nil
}

func workspaceReadRefusalStatus(blueclawStatus int) int {
	switch blueclawStatus {
	case http.StatusForbidden, http.StatusNotFound, http.StatusRequestEntityTooLarge:
		return blueclawStatus
	default:
		return http.StatusBadGateway
	}
}
