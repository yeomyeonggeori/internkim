package admind

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

var (
	centralPlaneOnce   sync.Once
	centralPlaneClient *centralplane.Client
)

func (service *Service) centralPlane() *centralplane.Client {
	centralPlaneOnce.Do(func() {
		agentKey := service.centralPlaneAgentKey()
		settings := centralplane.Settings{
			AppURL:         service.Configuration.CentralPlaneAppURL,
			AgentAPIKey:    agentKey,
			ProjectURL:     service.Configuration.CentralPlaneProjectURL,
			PublishableKey: service.Configuration.CentralPlanePublishableKey,
		}
		if !settings.Configured() {
			return
		}
		centralPlaneClient = centralplane.New(settings)
		log.Printf("attendance also goes to the central plane at %s", settings.ProjectURL)
	})
	return centralPlaneClient
}

func (service *Service) alsoRecordAttendanceCentrally(event attendanceEvent) {
	client := service.centralPlane()
	if client == nil || strings.TrimSpace(event.MattermostUserID) == "" {
		return
	}
	occurredAt, errorValue := time.Parse(time.RFC3339, event.OccurredAt)
	if errorValue != nil {
		occurredAt = time.Now().UTC()
	}
	record := centralplane.AttendanceRecord{
		Platform:   "mattermost",
		ExternalID: event.MattermostUserID,
		Kind:       event.Kind,
		Location:   event.LocationName,
		OccurredAt: occurredAt,
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if errorValue := client.RecordAttendance(ctx, record); errorValue != nil {
			log.Printf("central plane did not take %s for %s: %v", record.Kind, record.ExternalID, errorValue)
		}
	}()
}
