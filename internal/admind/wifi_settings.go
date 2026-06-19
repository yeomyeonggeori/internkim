package admind

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"
)

type wifiProfile struct {
	Name                 string `json:"name"`
	SSID                 string `json:"ssid"`
	IsActive             bool   `json:"isActive"`
	IsManagedByInternkim bool   `json:"isManagedByInternkim"`
}

type wifiProfilesResponse struct {
	Profiles []wifiProfile `json:"profiles"`
}

type addWifiProfileRequest struct {
	SSID     string `json:"ssid"`
	Password string `json:"password"`
}

type updateWifiPasswordRequest struct {
	Password string `json:"password"`
}

func (service *Service) writeWifiProfiles(responseWriter http.ResponseWriter) {
	profiles, errorValue := listWifiProfiles()
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, wifiProfilesResponse{Profiles: profiles})
}

func (service *Service) addWifiProfile(responseWriter http.ResponseWriter, request *http.Request) {
	var payload addWifiProfileRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	ssid := strings.TrimSpace(payload.SSID)
	if ssid == "" {
		http.Error(responseWriter, "ssid is required", http.StatusBadRequest)
		return
	}
	connectionName := "internkim-wifi-" + slugifySSID(ssid)
	args := []string{"connection", "add", "type", "wifi", "con-name", connectionName, "ssid", ssid}
	if password := strings.TrimSpace(payload.Password); password != "" {
		args = append(args, "wifi-sec.key-mgmt", "wpa-psk", "wifi-sec.psk", password)
	}
	if out, errorValue := exec.Command("nmcli", args...).CombinedOutput(); errorValue != nil {
		http.Error(responseWriter, strings.TrimSpace(string(out)), http.StatusInternalServerError)
		return
	}
	exec.Command("nmcli", "connection", "up", connectionName).Run() //nolint:errcheck
	profiles, errorValue := listWifiProfiles()
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, wifiProfilesResponse{Profiles: profiles})
}

func (service *Service) updateWifiPassword(responseWriter http.ResponseWriter, request *http.Request, connectionName string) {
	var payload updateWifiPasswordRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	password := strings.TrimSpace(payload.Password)
	if password == "" {
		http.Error(responseWriter, "password is required", http.StatusBadRequest)
		return
	}
	out, errorValue := exec.Command("nmcli", "connection", "modify", connectionName, "wifi-sec.key-mgmt", "wpa-psk", "wifi-sec.psk", password).CombinedOutput()
	if errorValue != nil {
		http.Error(responseWriter, strings.TrimSpace(string(out)), http.StatusInternalServerError)
		return
	}
	exec.Command("nmcli", "connection", "up", connectionName).Run() //nolint:errcheck
	profiles, errorValue := listWifiProfiles()
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, wifiProfilesResponse{Profiles: profiles})
}

func (service *Service) removeWifiProfile(responseWriter http.ResponseWriter, request *http.Request, connectionName string) {
	out, errorValue := exec.Command("nmcli", "connection", "delete", connectionName).CombinedOutput()
	if errorValue != nil {
		http.Error(responseWriter, strings.TrimSpace(string(out)), http.StatusInternalServerError)
		return
	}
	profiles, errorValue := listWifiProfiles()
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, wifiProfilesResponse{Profiles: profiles})
}

func listWifiProfiles() ([]wifiProfile, error) {
	out, errorValue := exec.Command("nmcli", "-t", "-f", "NAME,TYPE,ACTIVE", "connection", "show").Output()
	if errorValue != nil {
		return nil, errorValue
	}
	var profiles []wifiProfile
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		name, connType, isActive, ok := parseNmcliConnectionLine(line)
		if !ok || connType != "wifi" {
			continue
		}
		profiles = append(profiles, wifiProfile{
			Name:                 name,
			SSID:                 wifiConnectionSSID(name),
			IsActive:             isActive,
			IsManagedByInternkim: strings.HasPrefix(name, "internkim-wifi-"),
		})
	}
	return profiles, nil
}

func parseNmcliConnectionLine(line string) (name, connType string, isActive bool, ok bool) {
	lastColon := strings.LastIndex(line, ":")
	if lastColon < 0 {
		return
	}
	active := line[lastColon+1:]
	rest := line[:lastColon]
	secondLastColon := strings.LastIndex(rest, ":")
	if secondLastColon < 0 {
		return
	}
	return rest[:secondLastColon], rest[secondLastColon+1:], active == "yes", true
}

func wifiConnectionSSID(connectionName string) string {
	out, errorValue := exec.Command("nmcli", "-g", "802-11-wireless.ssid", "connection", "show", connectionName).Output()
	if errorValue != nil {
		return connectionName
	}
	ssid := strings.TrimSpace(string(out))
	if ssid == "" {
		return connectionName
	}
	return ssid
}

func slugifySSID(ssid string) string {
	slug := strings.ToLower(ssid)
	slug = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-':
			return r
		case r == ' ' || r == '_':
			return '-'
		default:
			return -1
		}
	}, slug)
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "wifi"
	}
	if len(slug) > 40 {
		return slug[:40]
	}
	return slug
}
