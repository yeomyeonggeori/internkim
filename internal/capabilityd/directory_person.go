package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

// An account the agent does not recognize is the moment to ask whether it belongs to
// somebody here, because a person is waiting on the answer. The host owns that question:
// it reads the company directory and leaves the agent carrying the person, so the message
// that prompted this is served rather than refused.
//
// A directory that cannot answer leaves the agent as it was. The agent refuses, which is
// the same thing it did before this existed.
func (service Service) ensureDirectoryPerson(ctx context.Context, resolved map[string]string) {
	email := strings.TrimSpace(resolved["email"])
	if email == "" {
		return
	}
	payload, errorValue := json.Marshal(map[string]string{"email": email, "name": strings.TrimSpace(resolved["name"])})
	if errorValue != nil {
		return
	}
	requestContext, cancel := context.WithTimeout(ctx, directoryLookupBudget)
	defer cancel()
	request, errorValue := http.NewRequestWithContext(requestContext, http.MethodPost,
		strings.TrimRight(service.Configuration.AdmindBaseURL, "/")+"/admin/api/directory/person",
		bytes.NewReader(payload))
	if errorValue != nil {
		return
	}
	request.Header.Set("Content-Type", "application/json")
	response, errorValue := http.DefaultClient.Do(request)
	if errorValue != nil {
		log.Printf("directory.person.unreachable: %v", errorValue)
		return
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		log.Printf("directory.person.refused: %s for %s", response.Status, email)
	}
}

// directoryLookupBudget bounds the wait an inbound message inherits from this lookup.
// The person is waiting, so a slow directory has to give up rather than hold the turn.
const directoryLookupBudget = 5 * time.Second
