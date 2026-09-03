package admind

import (
	"strings"
	"time"
)

const workspaceSystemTimeZone = "system"

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
	return resolveWorkspaceTimeZone(service.readWorkspaceSettings(), systemTimeZone)
}

func resolveWorkspaceTimeZone(
	settings workspaceSettings,
	resolveSystemTimeZone func() resolvedTimeZone,
) resolvedTimeZone {
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
