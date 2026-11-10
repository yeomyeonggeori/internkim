package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
)

const seatAdministratorsAdmindPath = "/agent/api/buzz-seat-admins"

func (service Service) seatAdministratorsInCreatedChannel(ctx context.Context, requesterEmail string, channelID string) {
	body, errorValue := json.Marshal(map[string]string{"channelID": channelID})
	if errorValue != nil {
		log.Printf("tool.channel_add.administrators_not_seated: channelID=%s error=%v", channelID, errorValue)
		return
	}
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, admindRequesterURL(seatAdministratorsAdmindPath), bytes.NewReader(body))
	if errorValue != nil {
		log.Printf("tool.channel_add.administrators_not_seated: channelID=%s error=%v", channelID, errorValue)
		return
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	response, errorValue := service.askAdmindAsTheRequester(httpRequest, requesterEmail)
	if errorValue != nil {
		log.Printf("tool.channel_add.administrators_not_seated: channelID=%s error=%v", channelID, errorValue)
		return
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusMultipleChoices {
		answer, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		log.Printf("tool.channel_add.administrators_not_seated: channelID=%s status=%d answer=%s", channelID, response.StatusCode, strings.TrimSpace(string(answer)))
	}
}
