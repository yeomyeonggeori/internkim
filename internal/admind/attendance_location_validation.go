package admind

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

var attendanceLocationColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func normalizeAttendanceLocations(locations []attendanceLocation) ([]attendanceLocation, error) {
	normalizedLocations := make([]attendanceLocation, 0, len(locations))
	seenIDs := map[string]bool{}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, location := range locations {
		normalizedLocation := normalizeAttendanceLocation(location, now)
		if normalizedLocation.Name == "" {
			continue
		}
		if seenIDs[normalizedLocation.ID] {
			normalizedLocation.ID = normalizedLocation.ID + "-" + randomHex(4)
		}
		seenIDs[normalizedLocation.ID] = true
		normalizedLocations = append(normalizedLocations, normalizedLocation)
	}
	if len(normalizedLocations) == 0 {
		return nil, errors.New("at least one attendance location is required")
	}
	ensureOneDefaultAttendanceLocation(normalizedLocations)
	return normalizedLocations, nil
}

func normalizeAttendanceLocation(location attendanceLocation, updatedAt string) attendanceLocation {
	name := strings.TrimSpace(location.Name)
	id := strings.TrimSpace(location.ID)
	if id == "" {
		id = "location-" + randomHex(4)
	}
	color := strings.TrimSpace(location.Color)
	if !attendanceLocationColorPattern.MatchString(color) {
		color = "#16a34a"
	}
	return attendanceLocation{
		ID:        id,
		Name:      name,
		Color:     color,
		IsDefault: location.IsDefault,
		UpdatedAt: firstNonEmpty(strings.TrimSpace(location.UpdatedAt), updatedAt),
	}
}

func ensureOneDefaultAttendanceLocation(locations []attendanceLocation) {
	defaultIndex := -1
	for index, location := range locations {
		if location.IsDefault && defaultIndex == -1 {
			defaultIndex = index
			continue
		}
		locations[index].IsDefault = false
	}
	if defaultIndex == -1 {
		locations[0].IsDefault = true
	}
}
