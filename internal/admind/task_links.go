package admind

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/fleetdomain"
)

func (service *Service) linkedTaskID(taskID string) string {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || !service.linksGoToTheRecord() {
		return taskID
	}
	return service.recordTaskIDCarrying(taskID)
}

func (service *Service) linksGoToTheRecord() bool {
	return strings.TrimSpace(service.Configuration.CentralPlaneAppURL) != ""
}

// An identifier belongs with the address it is sent to: the record keys a task
// by its own, this device by the one it made, and a link carrying the other
// one's opens nothing. A task the record has not taken yet has nothing to send,
// and the link opens the week it is in.

func (service *Service) recordTaskIDCarrying(deviceTaskID string) string {
	client := service.centralPlane()
	actorEmail := service.recordLinkActorEmail()
	if client == nil || actorEmail == "" {
		log.Printf("the record cannot be asked which task carries %s: the central plane client or an admin identity is missing", deviceTaskID)
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	recordID, errorValue := client.TaskCarrying(ctx, "email", actorEmail, deviceTaskID)
	if errorValue != nil {
		log.Printf("the record did not answer which task carries %s: %v", deviceTaskID, errorValue)
		return ""
	}
	return strings.TrimSpace(recordID)
}

func (service *Service) recordLinkActorEmail() string {
	if claimedEmail := service.claimedAdminEmail(); claimedEmail != "" {
		return claimedEmail
	}
	return service.seedAdminEmail()
}

// Everyone signs in at the company's own address and the record lives behind it,
// so that is where a link points. A device address is what is left for a company
// that has not moved.
func (service *Service) taskLinkBaseURL() string {
	if appURL := strings.TrimSpace(service.Configuration.CentralPlaneAppURL); appURL != "" {
		return appURL
	}
	if taskPublicURL := strings.TrimSpace(service.Configuration.TaskPublicURL); taskPublicURL != "" {
		return taskPublicURL
	}
	if written := strings.TrimSpace(readTrimmedFile(service.Configuration.TaskPublicURLPath)); written != "" {
		return written
	}
	return service.deviceTaskBaseURL()
}

func (service *Service) deviceTaskBaseURL() string {
	if deviceURL := strings.TrimSpace(readTrimmedFile(service.Configuration.DeviceURLPath)); deviceURL != "" {
		return deviceURL
	}
	if fleetID := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)); fleetID != "" {
		return fleetdomain.Subdomain(strings.ToLower(fleetID), service.fleetZone())
	}
	return ""
}
