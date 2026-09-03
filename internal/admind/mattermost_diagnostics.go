package admind

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"gitlab.com/eastriver/internkim/internal/buzzimport/mattermostadmin"
)

type mattermostPostRecord struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	RootID    string `json:"root_id"`
	Type      string `json:"type"`
	CreateAt  int64  `json:"create_at"`
	DeleteAt  int64  `json:"delete_at"`
}

type mattermostPostDiagnostic struct {
	ID        string `json:"id"`
	ChannelID string `json:"channelID"`
	RootID    string `json:"rootID"`
	Type      string `json:"type"`
	CreateAt  int64  `json:"createAt"`
	DeleteAt  int64  `json:"deleteAt"`
}

func (service *Service) writeMattermostPostDiagnostic(responseWriter http.ResponseWriter, request *http.Request) {
	postID := strings.TrimSpace(request.URL.Query().Get("postID"))
	if postID == "" {
		http.Error(responseWriter, "postID is required", http.StatusBadRequest)
		return
	}
	token, errorValue := service.mattermostAdmin().AdminToken(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	postRecord, isFound, errorValue := service.mattermostPostByID(request.Context(), token, postID)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	if !isFound {
		http.Error(responseWriter, "post was not found", http.StatusNotFound)
		return
	}
	service.writeJSON(responseWriter, mattermostPostDiagnostic{
		ID:        postRecord.ID,
		ChannelID: postRecord.ChannelID,
		RootID:    postRecord.RootID,
		Type:      postRecord.Type,
		CreateAt:  postRecord.CreateAt,
		DeleteAt:  postRecord.DeleteAt,
	})
}

func (service *Service) mattermostPostByID(ctx context.Context, token string, postID string) (mattermostPostRecord, bool, error) {
	var postRecord mattermostPostRecord
	errorValue := service.mattermostAdmin().Request(ctx, http.MethodGet, "/api/v4/posts/"+url.PathEscape(postID), token, nil, &postRecord)
	if errorValue == nil && postRecord.ID != "" {
		return postRecord, true, nil
	}
	if errorValue != nil && !mattermostadmin.IsNotFound(errorValue) {
		return mattermostPostRecord{}, false, errorValue
	}
	return mattermostPostRecord{}, false, nil
}
