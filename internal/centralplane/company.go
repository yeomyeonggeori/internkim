package centralplane

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Company is what the company calls itself. The messenger a host happens to run
// carries a name of its own, and it is the messenger's name rather than the
// company's — reading it from there is how a company ends up introduced by the
// product it uses.
type Company struct {
	Name         string `json:"name"`
	ProfileImage string `json:"profileImage"`
	Timezone     string `json:"timezone"`
	Locale       string `json:"locale"`
}

func (client *Client) Company(ctx context.Context) (Company, bool, error) {
	if client == nil || !client.settings.Configured() {
		return Company{}, false, fmt.Errorf("central plane is not configured")
	}
	requestURL := strings.TrimSuffix(client.settings.AppURL, "/") + "/api/agent/company"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if errorValue != nil {
		return Company{}, false, errorValue
	}
	request.Header.Set("Authorization", "Bearer "+client.settings.AgentAPIKey)

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return Company{}, false, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return Company{}, false, fmt.Errorf("central plane refused the company lookup: %s", response.Status)
	}

	var answer struct {
		Company *Company `json:"company"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&answer); errorValue != nil {
		return Company{}, false, errorValue
	}
	if answer.Company == nil {
		return Company{}, false, nil
	}
	return *answer.Company, true, nil
}
