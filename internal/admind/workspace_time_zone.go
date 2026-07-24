package admind

import (
	"strings"
	"time"
)

const workspaceSystemTimeZone = "system"
const workspaceBusinessTimeZone = "Asia/Seoul"

type resolvedTimeZone struct {
	location        *time.Location
	name            string
	isAuthoritative bool
}

func (service *Service) workspaceTimeLocation() (*time.Location, string) {
	resolved := service.workspaceTimeZone()
	return resolved.location, resolved.name
}

func (service *Service) workspaceTimeZone() resolvedTimeZone {
	settings, errorValue := service.readWorkspaceSettings()
	return resolveWorkspaceTimeZone(settings, errorValue, systemTimeZone)
}

func resolveWorkspaceTimeZone(
	settings workspaceSettings,
	settingsError error,
	resolveSystemTimeZone func() resolvedTimeZone,
) resolvedTimeZone {
	if settingsError != nil {
		return nonAuthoritativeTimeZone(resolveSystemTimeZone())
	}
	timeZone := strings.TrimSpace(settings.TimeZone)
	if timeZone == "" || timeZone == workspaceSystemTimeZone {
		return resolveSystemTimeZone()
	}
	location, errorValue := time.LoadLocation(timeZone)
	if errorValue != nil {
		return nonAuthoritativeTimeZone(resolveSystemTimeZone())
	}
	return resolvedTimeZone{
		location:        location,
		name:            timeZone,
		isAuthoritative: true,
	}
}

func nonAuthoritativeTimeZone(resolved resolvedTimeZone) resolvedTimeZone {
	resolved.isAuthoritative = false
	return resolved
}
