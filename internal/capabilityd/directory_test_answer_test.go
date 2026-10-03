package capabilityd

import (
	"net/http"
	"strings"
)

const directoryPeopleTestPath = "/admin/api/directory/people"

// Every tool that names a person asks the company, so a transport a test stands
// up has to answer that question before its own.
func isDirectoryPeopleRequest(request *http.Request) bool {
	return request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, directoryPeopleTestPath)
}

// A flow state a test serves names the people on that board. The company knows
