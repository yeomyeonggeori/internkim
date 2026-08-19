package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

type directoryPersonRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type directoryPersonResponse struct {
	Known bool `json:"known"`
}

// handleDirectoryPerson answers whether an address belongs to this company and, when it
// does, makes sure the agent carries that person before saying so. The agent asks while a
// message waits, so the answer has to leave the roster ready rather than schedule work.
func (service *Service) handleDirectoryPerson(responseWriter http.ResponseWriter, request *http.Request) {
	var payload directoryPersonRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, "invalid request", http.StatusBadRequest)
		return
	}
	known, errorValue := service.ensurePersonFromDirectory(request.Context(), payload.Email, payload.Name)
	if errorValue != nil {
		log.Printf("directory.person.failed: %v", errorValue)
		http.Error(responseWriter, "the company directory could not be reached", http.StatusBadGateway)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(responseWriter).Encode(directoryPersonResponse{Known: known})
}

// ensurePersonFromDirectory reports whether the address belongs to an active member and
// projects that member onto the agent. A directory that cannot be reached is an error and
// never a "no", because refusing a colleague is worse than making them wait.
func (service *Service) ensurePersonFromDirectory(ctx context.Context, email string, name string) (bool, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return false, nil
	}
	client := service.centralPlane()
	if client == nil {
		return false, fmt.Errorf("this host has no company directory configured")
	}
	member, isKnown, errorValue := client.MemberByEmail(ctx, normalizedEmail)
	if errorValue != nil {
		return false, errorValue
	}
	if !isKnown || !member.IsActive() {
		return false, nil
	}
	if errorValue := service.upsertBlueclawPerson(ctx, member.MemberID, normalizedEmail, strings.TrimSpace(name), member.Role, nil, nil); errorValue != nil {
		return false, fmt.Errorf("projecting %s onto the agent failed: %w", normalizedEmail, errorValue)
	}
	log.Printf("directory.person.projected: %s", normalizedEmail)
	return true, nil
}
