package admind

import (
	"log"
	"net/http"
)

func (service *Service) syncMattermostAttendanceChannelAfterLocationUpdate(request *http.Request) {
	adminToken, errorValue := service.mattermostAdminToken(request.Context())
	if errorValue != nil {
		log.Printf("Mattermost Attendance location sync failed: %v", errorValue)
		return
	}
	teamRecord, errorValue := service.ensureMattermostTeam(request.Context(), adminToken)
	if errorValue != nil {
		log.Printf("Mattermost Attendance location sync failed: %v", errorValue)
		return
	}
	if _, errorValue := service.ensureMattermostAttendanceChannel(request.Context(), adminToken, teamRecord.ID); errorValue != nil {
		log.Printf("Mattermost Attendance location sync failed: %v", errorValue)
	}
}
