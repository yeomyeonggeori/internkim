package admind

import (
	"net/http"
	"strings"
)

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
	token, errorValue := service.mattermostAdminToken(request.Context())
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
