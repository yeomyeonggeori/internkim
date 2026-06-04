package admind

import "context"

func (service *Service) ensureMattermostAttendanceChannel(ctx context.Context, token string, teamID string) (string, error) {
	channel, _ := service.mattermostManagedPublicChannel(attendanceChannelName)
	channelID, errorValue := service.ensureMattermostPublicChannel(ctx, token, teamID, channel.Name, channel.DisplayName)
	if errorValue != nil {
		return "", errorValue
	}
	service.saveMattermostAttendanceChannelID(channelID)
	if errorValue := service.updateMattermostManagedPublicChannelText(ctx, token, channelID, channel); errorValue != nil {
		return "", errorValue
	}
	service.syncMattermostAttendanceEntryPost(ctx, token, channelID)
	return channelID, nil
}

func (service *Service) updateMattermostAttendanceChannelText(ctx context.Context, token string, channelID string) error {
	channel, _ := service.mattermostManagedPublicChannel(attendanceChannelName)
	return service.updateMattermostManagedPublicChannelText(ctx, token, channelID, channel)
}
