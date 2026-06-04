package admind

const attendanceLocationsFilename = "attendance-locations.json"

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
