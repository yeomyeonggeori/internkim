package admind

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const calendarActorProfileCacheTTL = 5 * time.Minute

type calendarActorKind string

const (
	calendarCreatedActor calendarActorKind = "created"
	calendarUpdatedActor calendarActorKind = "updated"
)

type calendarActorProfile struct {
	Name            string
	UserID          string
	HasProfileImage bool
}

type calendarActorProfileCacheEntry struct {
	Profile   calendarActorProfile
	ExpiresAt time.Time
}

func (service *Service) calendarEventWithActorProfiles(ctx context.Context, event calendarEvent) calendarEvent {
	events := service.calendarEventsWithActorProfiles(ctx, []calendarEvent{event})
	if len(events) == 0 {
		return event
	}
	return events[0]
}

func (service *Service) calendarEventsWithActorProfiles(ctx context.Context, events []calendarEvent) []calendarEvent {
	result := append([]calendarEvent(nil), events...)
	profiles := service.calendarActorProfiles(ctx, result)
	for index := range result {
		result[index].CreatedByName = calendarActorDisplayName(result[index].CreatedByName, result[index].CreatedByEmail, profiles)
		result[index].CreatedByImage = calendarActorImagePath(result[index].ID, calendarCreatedActor, result[index].CreatedByEmail, profiles)
		result[index].UpdatedByName = calendarActorDisplayName(result[index].UpdatedByName, result[index].UpdatedByEmail, profiles)
		result[index].UpdatedByImage = calendarActorImagePath(result[index].ID, calendarUpdatedActor, result[index].UpdatedByEmail, profiles)
	}
	return result
}

func (service *Service) calendarActorProfiles(ctx context.Context, events []calendarEvent) map[string]calendarActorProfile {
	emails := calendarActorEmails(events)
	profiles := map[string]calendarActorProfile{}
	missingEmails := service.cachedCalendarActorProfiles(emails, time.Now(), profiles)
	if len(missingEmails) == 0 || strings.TrimSpace(service.Configuration.MattermostBaseURL) == "" {
		return profiles
	}
	token, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return profiles
	}
	expiresAt := time.Now().Add(calendarActorProfileCacheTTL)
	for _, email := range missingEmails {
		userRecord, found, errorValue := service.findMattermostUserByEmail(ctx, token, email)
		if errorValue != nil || !found || strings.TrimSpace(userRecord.ID) == "" {
			continue
		}
		profile := calendarActorProfile{
			Name:            firstNonEmpty(mattermostDisplayName(userRecord), email),
			UserID:          strings.TrimSpace(userRecord.ID),
			HasProfileImage: mattermostUserHasProfileImage(userRecord),
		}
		profiles[email] = profile
		service.storeCalendarActorProfile(email, profile, expiresAt)
	}
	return profiles
}

func (service *Service) cachedCalendarActorProfiles(emails []string, now time.Time, profiles map[string]calendarActorProfile) []string {
	service.calendarActorCacheMutex.Lock()
	defer service.calendarActorCacheMutex.Unlock()
	if service.calendarActorCache == nil {
		service.calendarActorCache = map[string]calendarActorProfileCacheEntry{}
	}
	missingEmails := []string{}
	for _, email := range emails {
		entry, found := service.calendarActorCache[email]
		if found && now.Before(entry.ExpiresAt) && strings.TrimSpace(entry.Profile.UserID) != "" {
			profiles[email] = entry.Profile
			continue
		}
		if found {
			delete(service.calendarActorCache, email)
		}
		missingEmails = append(missingEmails, email)
	}
	return missingEmails
}

func (service *Service) storeCalendarActorProfile(email string, profile calendarActorProfile, expiresAt time.Time) {
	if strings.TrimSpace(profile.UserID) == "" {
		return
	}
	service.calendarActorCacheMutex.Lock()
	defer service.calendarActorCacheMutex.Unlock()
	if service.calendarActorCache == nil {
		service.calendarActorCache = map[string]calendarActorProfileCacheEntry{}
	}
	service.calendarActorCache[email] = calendarActorProfileCacheEntry{Profile: profile, ExpiresAt: expiresAt}
}

func calendarActorEmails(events []calendarEvent) []string {
	seenEmails := map[string]bool{}
	emails := []string{}
	for _, event := range events {
		for _, email := range []string{event.CreatedByEmail, event.UpdatedByEmail} {
			normalizedEmail := normalizedCalendarActorEmail(email)
			if normalizedEmail == "" || seenEmails[normalizedEmail] {
				continue
			}
			seenEmails[normalizedEmail] = true
			emails = append(emails, normalizedEmail)
		}
	}
	return emails
}

func calendarActorDisplayName(name string, email string, profiles map[string]calendarActorProfile) string {
	normalizedEmail := normalizedCalendarActorEmail(email)
	if profile, found := profiles[normalizedEmail]; found && strings.TrimSpace(profile.Name) != "" {
		return strings.TrimSpace(profile.Name)
	}
	return firstNonEmpty(strings.TrimSpace(name), normalizedEmail)
}

func calendarActorImagePath(eventID string, actorKind calendarActorKind, email string, profiles map[string]calendarActorProfile) string {
	normalizedEmail := normalizedCalendarActorEmail(email)
	profile, found := profiles[normalizedEmail]
	if !found || strings.TrimSpace(eventID) == "" || strings.TrimSpace(profile.UserID) == "" || !profile.HasProfileImage {
		return ""
	}
	return "/calendar/api/events/" + url.PathEscape(strings.TrimSpace(eventID)) + "/actor-image?actor=" + url.QueryEscape(string(actorKind))
}

func normalizedCalendarActorEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (service *Service) serveCalendarActorImage(responseWriter http.ResponseWriter, request *http.Request, path string) {
	eventID := calendarActorImageEventID(path)
	actorKind, ok := calendarActorKindFromRequest(request.URL.Query().Get("actor"))
	if eventID == "" || !ok {
		http.NotFound(responseWriter, request)
		return
	}
	event, found, errorValue := service.readCalendarEventByID(request.Context(), eventID)
	if errorValue != nil {
		http.Error(responseWriter, "actor image unavailable", http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(responseWriter, request)
		return
	}
	email := calendarActorEmail(event, actorKind)
	if email == "" {
		http.NotFound(responseWriter, request)
		return
	}
	profile, found := service.calendarActorProfiles(request.Context(), []calendarEvent{event})[email]
	if !found || strings.TrimSpace(profile.UserID) == "" || !profile.HasProfileImage {
		http.NotFound(responseWriter, request)
		return
	}
	token, errorValue := service.mattermostAdminToken(request.Context())
	if errorValue != nil {
		http.NotFound(responseWriter, request)
		return
	}
	service.serveMattermostUserImage(responseWriter, request, token, profile.UserID)
}

func calendarActorImageEventID(path string) string {
	eventsPath := strings.TrimPrefix(path, "/events/")
	eventID, suffix, found := strings.Cut(eventsPath, "/actor-image")
	if !found || suffix != "" || strings.TrimSpace(eventID) == "" || strings.Contains(eventID, "/") {
		return ""
	}
	return eventID
}

func calendarActorKindFromRequest(value string) (calendarActorKind, bool) {
	actorKind := calendarActorKind(strings.TrimSpace(value))
	switch actorKind {
	case calendarCreatedActor, calendarUpdatedActor:
		return actorKind, true
	default:
		return "", false
	}
}

func calendarActorEmail(event calendarEvent, actorKind calendarActorKind) string {
	switch actorKind {
	case calendarCreatedActor:
		return normalizedCalendarActorEmail(event.CreatedByEmail)
	case calendarUpdatedActor:
		return normalizedCalendarActorEmail(event.UpdatedByEmail)
	default:
		return ""
	}
}

func (service *Service) serveMattermostUserImage(responseWriter http.ResponseWriter, request *http.Request, token string, userID string) {
	requestURL := strings.TrimRight(service.Configuration.MattermostBaseURL, "/") + "/api/v4/users/" + url.PathEscape(strings.TrimSpace(userID)) + "/image"
	mattermostRequest, errorValue := http.NewRequestWithContext(request.Context(), http.MethodGet, requestURL, nil)
	if errorValue != nil {
		http.Error(responseWriter, "actor image unavailable", http.StatusBadGateway)
		return
	}
	mattermostRequest.Header.Set("Authorization", "Bearer "+token)
	mattermostResponse, errorValue := service.httpClient().Do(mattermostRequest)
	if errorValue != nil {
		http.Error(responseWriter, "actor image unavailable", http.StatusBadGateway)
		return
	}
	defer mattermostResponse.Body.Close()
	if mattermostResponse.StatusCode == http.StatusNotFound {
		http.NotFound(responseWriter, request)
		return
	}
	if mattermostResponse.StatusCode < http.StatusOK || mattermostResponse.StatusCode >= http.StatusMultipleChoices {
		http.Error(responseWriter, "actor image unavailable", http.StatusBadGateway)
		return
	}
	contentType := strings.TrimSpace(mattermostResponse.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "image/png"
	}
	responseWriter.Header().Set("Content-Type", contentType)
	responseWriter.Header().Set("Cache-Control", "private, max-age=300")
	_, _ = io.Copy(responseWriter, mattermostResponse.Body)
}
