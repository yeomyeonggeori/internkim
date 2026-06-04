package admind

import (
	"os"
	"path/filepath"
	"strings"
)

func (service *Service) saveMattermostAttendanceEntryPostID(postID string) {
	normalizedPostID := strings.TrimSpace(postID)
	if normalizedPostID == "" {
		return
	}
	path := service.mattermostAttendanceEntryPostIDPath()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return
	}
	_ = os.WriteFile(path, []byte(normalizedPostID), 0o600)
}

func (service *Service) mattermostAttendanceEntryPostIDPath() string {
	return filepath.Join(service.Configuration.StateDirectory, attendanceEntryPostIDFilename)
}

func (service *Service) isMattermostAttendanceEntryPostID(postID string) bool {
	return strings.TrimSpace(postID) != "" && strings.TrimSpace(postID) == strings.TrimSpace(readTrimmedFile(service.mattermostAttendanceEntryPostIDPath()))
}

func (service *Service) saveMattermostAttendanceChannelID(channelID string) {
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return
	}
	path := service.mattermostAttendanceChannelIDPath()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return
	}
	_ = os.WriteFile(path, []byte(normalizedChannelID), 0o600)
}

func (service *Service) mattermostAttendanceChannelIDPath() string {
	return filepath.Join(service.Configuration.StateDirectory, attendanceChannelIDFilename)
}
