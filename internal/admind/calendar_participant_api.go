package admind

import (
	"net/http"
	"net/url"
	"strings"
)

type calendarParticipantsResponse struct {
	Participants []calendarParticipant `json:"participants"`
}

func (service *Service) listCalendarParticipants(responseWriter http.ResponseWriter, request *http.Request) {
	members := service.flowMembers(request)
	service.writeJSON(responseWriter, calendarParticipantsResponse{
		Participants: calendarParticipantsFromMembers(members),
	})
}

func (service *Service) serveCalendarParticipantImage(responseWriter http.ResponseWriter, request *http.Request, path string) {
	member, found := calendarParticipantImageMember(service.flowMembers(request), path)
	if !found {
		http.NotFound(responseWriter, request)
		return
	}
	token, errorValue := service.mattermostAdminToken(request.Context())
	if errorValue != nil {
		http.NotFound(responseWriter, request)
		return
	}
	userRecord, found, errorValue := service.calendarParticipantMattermostUser(request, token, member)
	if errorValue != nil || !found || strings.TrimSpace(userRecord.ID) == "" {
		http.NotFound(responseWriter, request)
		return
	}
	service.serveMattermostUserImage(responseWriter, request, token, userRecord.ID)
}

func calendarParticipantImagePath(personID string) string {
	trimmedPersonID := strings.TrimSpace(personID)
	if trimmedPersonID == "" {
		return ""
	}
	return "/calendar/api/participants/" + url.PathEscape(trimmedPersonID) + "/image"
}

func calendarParticipantImageMember(members []flowMember, path string) (flowMember, bool) {
	participantPath := strings.TrimPrefix(path, "/participants/")
	personID, suffix, found := strings.Cut(participantPath, "/image")
	if !found || suffix != "" || strings.TrimSpace(personID) == "" || strings.Contains(personID, "/") {
		return flowMember{}, false
	}
	for _, member := range members {
		if strings.EqualFold(strings.TrimSpace(member.ID), personID) {
			return member, true
		}
	}
	return flowMember{}, false
}

func (service *Service) calendarParticipantMattermostUser(request *http.Request, token string, member flowMember) (mattermostUserRecord, bool, error) {
	username := strings.TrimSpace(member.MattermostUsername)
	if username != "" {
		userRecord, found, errorValue := service.findMattermostUserByUsername(request.Context(), token, username)
		if errorValue != nil || found {
			return userRecord, found, errorValue
		}
	}
	email := strings.ToLower(strings.TrimSpace(member.Email))
	if email == "" {
		return mattermostUserRecord{}, false, nil
	}
	return service.findMattermostUserByEmail(request.Context(), token, email)
}
