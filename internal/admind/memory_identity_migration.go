package admind

import (
	"context"
	"log"
	"net/http"
	"strings"
)

type memoryPersonMigrationEntry struct {
	CurrentPersonID  string `json:"currentPersonID"`
	Email            string `json:"email"`
	MattermostUserID string `json:"mattermostUserID"`
	Username         string `json:"username"`
}

type memoryIdentityMigrationResponse struct {
	Map            map[string]memoryPersonMigrationEntry `json:"map"`
	Unresolved     []string                              `json:"unresolved"`
	AlreadyCurrent []string                              `json:"alreadyCurrent"`
}

type mattermostPostLookup struct {
	UserID  string `json:"user_id"`
	Message string `json:"message"`
}

type memoryPersonMessage struct {
	MessageID  string `json:"messageID"`
	OccurredAt string `json:"occurredAt"`
	Text       string `json:"text"`
}

func (service *Service) writeMemoryPersonMessages(responseWriter http.ResponseWriter, request *http.Request) {
	if !isLocalRequest(request) && !service.isAuthorized(request) {
		http.Error(responseWriter, "admin required", http.StatusForbidden)
		return
	}
	personID := strings.TrimSpace(request.URL.Query().Get("personID"))
	if personID == "" {
		http.Error(responseWriter, "personID is required", http.StatusBadRequest)
		return
	}
	var graph map[string]any
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodGet, "/admin/api/memory/graph", nil, &graph); errorValue != nil {
		http.Error(responseWriter, "memory graph unavailable", http.StatusBadGateway)
		return
	}
	botToken, errorValue := service.mattermostBotToken()
	if errorValue != nil {
		http.Error(responseWriter, "mattermost token unavailable", http.StatusBadGateway)
		return
	}
	messages := []memoryPersonMessage{}
	for _, rawEpisode := range graphEpisodesForSender(graph, personID) {
		messageID, _ := rawEpisode["messageID"].(string)
		occurredAt, _ := rawEpisode["occurredAt"].(string)
		if strings.TrimSpace(messageID) == "" || len(messages) >= 40 {
			continue
		}
		var post mattermostPostLookup
		if errorValue := service.mattermostRequest(request.Context(), http.MethodGet, "/api/v4/posts/"+messageID, botToken, nil, &post); errorValue != nil {
			continue
		}
		messages = append(messages, memoryPersonMessage{MessageID: messageID, OccurredAt: occurredAt, Text: post.Message})
	}
	service.writeJSON(responseWriter, map[string]any{"personID": personID, "messages": messages})
}

func graphEpisodesForSender(graph map[string]any, senderPersonID string) []map[string]any {
	rawEpisodes, _ := graph["episodes"].([]any)
	episodes := []map[string]any{}
	for _, rawEpisode := range rawEpisodes {
		episode, ok := rawEpisode.(map[string]any)
		if !ok {
			continue
		}
		if sender, _ := episode["senderPersonID"].(string); sender == senderPersonID {
			episodes = append(episodes, episode)
		}
	}
	return episodes
}

type mattermostUserLookup struct {
	Email    string `json:"email"`
	Username string `json:"username"`
}

func (service *Service) writeMemoryIdentityMigrationMap(responseWriter http.ResponseWriter, request *http.Request) {
	if !isLocalRequest(request) && !service.isAuthorized(request) {
		http.Error(responseWriter, "admin required", http.StatusForbidden)
		return
	}

	var graph map[string]any
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodGet, "/admin/api/memory/graph", nil, &graph); errorValue != nil {
		log.Printf("memory identity migration graph fetch failed: %v", errorValue)
		http.Error(responseWriter, "memory graph unavailable", http.StatusBadGateway)
		return
	}

	botToken, errorValue := service.mattermostBotToken()
	if errorValue != nil {
		log.Printf("memory identity migration bot token unavailable: %v", errorValue)
		http.Error(responseWriter, "mattermost token unavailable", http.StatusBadGateway)
		return
	}

	policyPersonIDs := service.collectPolicyPersonIDs(request.Context())
	senderMessageIDs := collectMattermostSenderMessageIDs(graph)

	migrationMap := map[string]memoryPersonMigrationEntry{}
	var unresolved []string
	var alreadyCurrent []string

	for senderPersonID, messageIDs := range senderMessageIDs {
		if policyPersonIDs[senderPersonID] {
			alreadyCurrent = append(alreadyCurrent, senderPersonID)
			continue
		}
		entry, resolved := service.resolveMigrationEntry(request.Context(), botToken, messageIDs)
		if !resolved {
			unresolved = append(unresolved, senderPersonID)
			continue
		}
		migrationMap[senderPersonID] = entry
	}

	log.Printf("memory identity migration: resolved=%d unresolved=%d alreadyCurrent=%d", len(migrationMap), len(unresolved), len(alreadyCurrent))
	service.writeJSON(responseWriter, memoryIdentityMigrationResponse{
		Map:            migrationMap,
		Unresolved:     unresolved,
		AlreadyCurrent: alreadyCurrent,
	})
}

func collectMattermostSenderMessageIDs(graph map[string]any) map[string][]string {
	rawEpisodes, _ := graph["episodes"].([]any)
	senderMessageIDs := map[string][]string{}
	for _, rawEpisode := range rawEpisodes {
		episode, ok := rawEpisode.(map[string]any)
		if !ok {
			continue
		}
		platform, _ := episode["platform"].(string)
		if platform != "mattermost" {
			continue
		}
		senderPersonID, _ := episode["senderPersonID"].(string)
		messageID, _ := episode["messageID"].(string)
		if senderPersonID == "" || messageID == "" {
			continue
		}
		senderMessageIDs[senderPersonID] = append(senderMessageIDs[senderPersonID], messageID)
	}
	return senderMessageIDs
}

func (service *Service) collectPolicyPersonIDs(ctx context.Context) map[string]bool {
	var policyDocument memoryPolicyDocument
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		return map[string]bool{}
	}
	personIDs := map[string]bool{}
	for _, person := range policyDocument.People {
		personIDs[strings.TrimSpace(person.PersonID)] = true
	}
	return personIDs
}

func (service *Service) resolveMigrationEntry(ctx context.Context, botToken string, messageIDs []string) (memoryPersonMigrationEntry, bool) {
	limit := 10
	for index, messageID := range messageIDs {
		if index >= limit {
			break
		}
		entry, resolved := service.resolveMigrationEntryFromMessage(ctx, botToken, messageID)
		if resolved {
			return entry, true
		}
	}
	return memoryPersonMigrationEntry{}, false
}

func (service *Service) resolveMigrationEntryFromMessage(ctx context.Context, botToken string, messageID string) (memoryPersonMigrationEntry, bool) {
	var post mattermostPostLookup
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/posts/"+messageID, botToken, nil, &post); errorValue != nil {
		return memoryPersonMigrationEntry{}, false
	}
	if post.UserID == "" {
		return memoryPersonMigrationEntry{}, false
	}

	var user mattermostUserLookup
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/"+post.UserID, botToken, nil, &user); errorValue != nil {
		return memoryPersonMigrationEntry{}, false
	}
	if user.Email == "" {
		return memoryPersonMigrationEntry{}, false
	}

	actor, found, errorValue := service.resolveUserActorByEmail(ctx, user.Email)
	if errorValue != nil || !found {
		return memoryPersonMigrationEntry{}, false
	}

	return memoryPersonMigrationEntry{
		CurrentPersonID:  actor.UserID,
		Email:            user.Email,
		MattermostUserID: post.UserID,
		Username:         user.Username,
	}, true
}
