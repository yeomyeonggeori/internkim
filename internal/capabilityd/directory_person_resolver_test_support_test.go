package capabilityd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The company is what decides who a name refers to, so a test that asks about a
// person has to say who works there. Members carry the addresses the board rows
// were written with, which is what joins the two.
func serviceWithDirectoryOf(t *testing.T, members []flowMemberForTool) Service {
	t.Helper()
	people := make([]directoryPerson, 0, len(members))
	for _, member := range members {
		person := directoryPerson{MemberID: member.ID, Email: member.Email, Name: member.Name}
		if handle := strings.TrimSpace(member.MattermostUsername); handle != "" {
			person.Messenger = map[string]string{"mattermost": handle}
		}
		people = append(people, person)
	}
	return serviceWithDirectoryPeople(t, people)
}

func serviceWithDirectoryPeople(t *testing.T, people []directoryPerson) Service {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/admin/api/directory/people" {
			http.NotFound(responseWriter, request)
			return
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(responseWriter).Encode(map[string]any{"people": people})
	}))
	t.Cleanup(server.Close)
	configuration := DefaultConfiguration()
	configuration.AdmindBaseURL = server.URL
	return Service{Configuration: configuration}
}
