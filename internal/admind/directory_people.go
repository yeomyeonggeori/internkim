package admind

import (
	"encoding/json"
	"net/http"
	"strings"
)

type directoryPerson struct {
	MemberID  string            `json:"memberID"`
	Email     string            `json:"email"`
	Name      string            `json:"name"`
	Messenger map[string]string `json:"messenger,omitempty"`
}

type directoryPeopleResponse struct {
	People []directoryPerson `json:"people"`
}

// Who works here is one question with one answer. Anything that has to turn a
// name into a person asks this rather than keeping a list of its own, so a
// colleague called by half their name means the same person to the calendar,
// the task board, and a message.
func (service *Service) handleDirectoryPeople(responseWriter http.ResponseWriter, request *http.Request) {
	client := service.centralPlane()
	if client == nil {
		http.Error(responseWriter, "this host has no company directory configured", http.StatusBadGateway)
		return
	}
	members, errorValue := client.Members(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	people := make([]directoryPerson, 0, len(members))
	for _, member := range members {
		if !member.IsActive() {
			continue
		}
		people = append(people, directoryPerson{
			MemberID:  strings.TrimSpace(member.MemberID),
			Email:     strings.ToLower(strings.TrimSpace(member.Email)),
			Name:      strings.TrimSpace(member.Name),
			Messenger: member.Messenger,
		})
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(responseWriter).Encode(directoryPeopleResponse{People: people})
}
