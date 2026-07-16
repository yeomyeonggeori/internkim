package admind

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	localTimePath         = "/etc/localtime"
	timeZonePath          = "/etc/timezone"
	zoneInfoDirectoryPath = "/usr/share/zoneinfo"
)

type systemTimeZoneSource struct {
	environmentTimeZone string
	localTimePath       string
	timeZonePath        string
	zoneInfoDirectory   string
	localLocation       *time.Location
}

func systemTimeZone() resolvedTimeZone {
	return resolveSystemTimeZone(systemTimeZoneSource{
		environmentTimeZone: os.Getenv("TZ"),
		localTimePath:       localTimePath,
		timeZonePath:        timeZonePath,
		zoneInfoDirectory:   zoneInfoDirectoryPath,
		localLocation:       time.Local,
	})
}

func resolveSystemTimeZone(source systemTimeZoneSource) resolvedTimeZone {
	if resolved, isResolved := loadSystemTimeZone(source.environmentTimeZone); isResolved {
		return resolved
	}
	if resolved, isResolved := loadSystemTimeZone(linkedTimeZoneName(source.localTimePath)); isResolved {
		return resolved
	}
	if resolved, isResolved := loadSystemTimeZone(storedTimeZoneName(source.timeZonePath)); isResolved &&
		isStoredTimeZoneConsistent(source.localTimePath, source.zoneInfoDirectory, resolved.name) {
		return resolved
	}
	if source.localLocation != nil {
		if resolved, isResolved := loadSystemTimeZone(source.localLocation.String()); isResolved {
			return resolved
		}
	}
	localLocation := source.localLocation
	if localLocation == nil {
		localLocation = time.Local
	}
	return resolvedTimeZone{
		location:        localLocation,
		name:            "Local",
		isAuthoritative: false,
	}
}

func isStoredTimeZoneConsistent(localTimePath string, zoneInfoDirectory string, timeZoneName string) bool {
	if strings.TrimSpace(localTimePath) == "" {
		return true
	}
	localTimeInformation, errorValue := os.Lstat(localTimePath)
	if os.IsNotExist(errorValue) {
		return true
	}
	if errorValue != nil || !localTimeInformation.Mode().IsRegular() {
		return false
	}
	localTimeDocument, errorValue := os.ReadFile(localTimePath)
	if errorValue != nil {
		return false
	}
	zoneInfoPath := filepath.Join(zoneInfoDirectory, filepath.FromSlash(timeZoneName))
	zoneInfoDocument, errorValue := os.ReadFile(zoneInfoPath)
	if errorValue != nil {
		return false
	}
	return bytes.Equal(localTimeDocument, zoneInfoDocument)
}

func loadSystemTimeZone(value string) (resolvedTimeZone, bool) {
	timeZoneName := normalizedTimeZoneName(value)
	if strings.HasPrefix(timeZoneName, "/") {
		timeZoneName = linkedTimeZoneName(timeZoneName)
	}
	if !isCanonicalTimeZoneName(timeZoneName) {
		return resolvedTimeZone{}, false
	}
	location, errorValue := time.LoadLocation(timeZoneName)
	if errorValue != nil {
		return resolvedTimeZone{}, false
	}
	return resolvedTimeZone{
		location:        location,
		name:            timeZoneName,
		isAuthoritative: true,
	}, true
}

func linkedTimeZoneName(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	resolvedPath, errorValue := filepath.EvalSymlinks(path)
	if errorValue != nil {
		return ""
	}
	normalizedPath := filepath.ToSlash(resolvedPath)
	markerIndex := strings.LastIndex(normalizedPath, "/zoneinfo/")
	if markerIndex < 0 {
		return ""
	}
	timeZoneName := strings.TrimPrefix(normalizedPath[markerIndex+len("/zoneinfo/"):], "posix/")
	return strings.TrimPrefix(timeZoneName, "right/")
}

func storedTimeZoneName(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return ""
	}
	return normalizedTimeZoneName(string(document))
}

func normalizedTimeZoneName(value string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(value), ":"))
}

func isCanonicalTimeZoneName(timeZoneName string) bool {
	if timeZoneName == "" || timeZoneName == "Local" || timeZoneName == workspaceSystemTimeZone {
		return false
	}
	if !strings.Contains(timeZoneName, "/") && strings.ContainsAny(timeZoneName, "0123456789,<>+-") {
		return false
	}
	return true
}
