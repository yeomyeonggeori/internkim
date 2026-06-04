package admind

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const attendanceLocationsFilename = "attendance-locations.json"

var attendanceLocationColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type attendanceLocation struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	IsDefault bool   `json:"isDefault"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type attendanceLocationsResponse struct {
	Locations []attendanceLocation `json:"locations"`
}

func (service *Service) writeAttendanceLocations(responseWriter http.ResponseWriter) {
	locations, errorValue := service.readAttendanceLocations()
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, attendanceLocationsResponse{Locations: locations})
}

func (service *Service) updateAttendanceLocations(responseWriter http.ResponseWriter, request *http.Request) {
	var payload attendanceLocationsResponse
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	locations, errorValue := normalizeAttendanceLocations(payload.Locations)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := service.writeAttendanceLocationsFile(locations); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.syncMattermostAttendanceChannelAfterLocationUpdate(request)
	service.writeJSON(responseWriter, attendanceLocationsResponse{Locations: locations})
}

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

func (service *Service) syncMattermostAttendanceChannelAfterLocationUpdate(request *http.Request) {
	adminToken, errorValue := service.mattermostAdminToken(request.Context())
	if errorValue != nil {
		log.Printf("Mattermost Attendance location sync failed: %v", errorValue)
		return
	}
	teamRecord, errorValue := service.ensureMattermostTeam(request.Context(), adminToken)
	if errorValue != nil {
		log.Printf("Mattermost Attendance location sync failed: %v", errorValue)
		return
	}
	if _, errorValue := service.ensureMattermostAttendanceChannel(request.Context(), adminToken, teamRecord.ID); errorValue != nil {
		log.Printf("Mattermost Attendance location sync failed: %v", errorValue)
	}
}

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

func defaultAttendanceLocations() []attendanceLocation {
	return []attendanceLocation{{
		ID:        "office",
		Name:      "사무실",
		Color:     "#16a34a",
		IsDefault: true,
	}}
}
