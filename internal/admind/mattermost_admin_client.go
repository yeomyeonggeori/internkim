package admind

import (
	"gitlab.com/eastriver/internkim/internal/buzzimport/mattermostadmin"
)

func (service *Service) mattermostAdmin() *mattermostadmin.Client {
	service.mattermostAdminOnce.Do(func() {
		service.mattermostAdminClient = mattermostadmin.New(mattermostadmin.Settings{
			BaseURL:           service.Configuration.MattermostBaseURL,
			AdminPasswordPath: service.Configuration.MattermostAdminPasswordPath,
			BotTokenPath:      service.Configuration.MattermostBotTokenPath,
			LegacyTokenPath:   service.Configuration.MattermostTokenPath,
			TeamName:          service.Configuration.MattermostTeamName,
			HTTPClient:        service.httpClient(),
			ReadTrimmedFile:   readTrimmedFile,
		})
	})
	return service.mattermostAdminClient
}
