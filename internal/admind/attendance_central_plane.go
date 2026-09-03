package admind

import (
	"log"
	"strings"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

func (service *Service) centralPlane() *centralplane.Client {
	service.centralPlaneOnce.Do(func() {
		settings := centralplane.Settings{
			AppURL:         service.Configuration.CentralPlaneAppURL,
			ProjectURL:     service.Configuration.CentralPlaneProjectURL,
			PublishableKey: service.Configuration.CentralPlanePublishableKey,
			HTTPClient:     service.HTTPClient,
		}
		// The key is fetched over the network, so a device that names no central
		// plane must not go asking for one to find out it has none.
		if strings.TrimSpace(settings.AppURL) == "" || strings.TrimSpace(settings.ProjectURL) == "" ||
			strings.TrimSpace(settings.PublishableKey) == "" {
			log.Printf("this device keeps its own records: the central plane is not configured")
			return
		}
		settings.AgentAPIKey = service.centralPlaneAgentKey()
		if !settings.Configured() {
			log.Printf("this device keeps its own records: the central plane issued no agent key")
			return
		}
		service.centralPlaneClient = centralplane.New(settings)
		log.Printf("this device reads and writes the central plane at %s", settings.ProjectURL)
	})
	return service.centralPlaneClient
}
