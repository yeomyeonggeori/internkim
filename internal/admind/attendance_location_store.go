package admind

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func (service *Service) readAttendanceLocations() ([]attendanceLocation, error) {
	document, errorValue := os.ReadFile(service.attendanceLocationsPath())
	if os.IsNotExist(errorValue) {
		return defaultAttendanceLocations(), nil
	}
	if errorValue != nil {
		return nil, errorValue
	}
	var response attendanceLocationsResponse
	if errorValue := json.Unmarshal(document, &response); errorValue != nil {
		return nil, errorValue
	}
	return normalizeAttendanceLocations(response.Locations)
}

func (service *Service) defaultAttendanceLocation() attendanceLocation {
	locations, errorValue := service.readAttendanceLocations()
	if errorValue != nil {
		locations = defaultAttendanceLocations()
	}
	for _, location := range locations {
		if location.IsDefault {
			return location
		}
	}
	return locations[0]
}

func (service *Service) attendanceLocationByID(locationID string) attendanceLocation {
	normalizedLocationID := strings.TrimSpace(locationID)
	locations, errorValue := service.readAttendanceLocations()
	if errorValue != nil {
		locations = defaultAttendanceLocations()
	}
	for _, location := range locations {
		if location.ID == normalizedLocationID {
			return location
		}
	}
	return service.defaultAttendanceLocation()
}

func (service *Service) attendanceLocationByName(locationName string) (attendanceLocation, bool) {
	normalizedLocationName := strings.ToLower(strings.TrimSpace(locationName))
	if normalizedLocationName == "" {
		return attendanceLocation{}, false
	}
	locations, errorValue := service.readAttendanceLocations()
	if errorValue != nil {
		locations = defaultAttendanceLocations()
	}
	for _, location := range locations {
		if strings.ToLower(strings.TrimSpace(location.Name)) == normalizedLocationName {
			return location, true
		}
		if strings.ToLower(strings.TrimSpace(service.attendanceDisplayLocationName(location))) == normalizedLocationName {
			return location, true
		}
	}
	return attendanceLocation{}, false
}

func (service *Service) writeAttendanceLocationsFile(locations []attendanceLocation) error {
	document, errorValue := json.MarshalIndent(attendanceLocationsResponse{Locations: locations}, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	path := service.attendanceLocationsPath()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return errorValue
	}
	return os.WriteFile(path, append(document, '\n'), 0o600)
}

func (service *Service) attendanceLocationsPath() string {
	return filepath.Join(service.Configuration.StateDirectory, attendanceLocationsFilename)
}

func defaultAttendanceLocations() []attendanceLocation {
	return []attendanceLocation{{
		ID:        "office",
		Name:      "사무실",
		Color:     "#16a34a",
		IsDefault: true,
	}}
}
