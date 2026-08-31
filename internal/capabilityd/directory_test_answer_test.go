package capabilityd

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

const directoryPeopleTestPath = "/admin/api/directory/people"

// Every tool that names a person asks the company, so a transport a test stands
// up has to answer that question before its own.
func isDirectoryPeopleRequest(request *http.Request) bool {
	return request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, directoryPeopleTestPath)
}

func directoryPeopleTestResponse(document string) *http.Response {
	if strings.TrimSpace(document) == "" {
		document = `{"people":[]}`
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(document)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

// The people these tests name. Resolution is the company's answer now, so a test
// that hints at somebody has to say that somebody works here.
// The company answers who a name refers to, so a test that hints at somebody has
// to say that somebody works here. A test whose roster is its own calls
// useDirectoryPeopleOf with it.
var directoryPeopleTestDocument = defaultDirectoryPeopleTestDocument

const defaultDirectoryPeopleTestDocument = `{"people":[
	{"memberID":"member","email":"member@example.com","name":"Member"},
	{"memberID":"person-sample","email":"sample@example.com","name":"이샘플","messenger":{"mattermost":"gamyeong"}},
	{"memberID":"person-one","email":"one@example.com","name":"Lee One"},
	{"memberID":"person-jungkook","email":"jungkook@example.com","name":"정국"},
	{"memberID":"person-two","email":"two@example.com","name":"Lee Two"},
	{"memberID":"person-specimen","email":"specimen@example.com","name":"최견본"},
	{"memberID":"person-example","email":"example@example.com","name":"박예시"},
	{"memberID":"person-other-example","email":"other@example.com","name":"이예시"},
	{"memberID":"person-sumin","email":"sumin@example.com","name":"김수민"},
	{"memberID":"person-test","email":"test@example.com","name":"테스트"},
	{"memberID":"person-alice","email":"alice@example.com","name":"Alice"}
]}`

func useDirectoryPeopleOf(t *testing.T, members []taskMemberForTool) {
	t.Helper()
	people := make([]map[string]string, 0, len(members))
	for _, member := range members {
		people = append(people, map[string]string{
			"memberID": member.ID,
			"email":    member.Email,
			"name":     member.Name,
		})
	}
	document, errorValue := json.Marshal(map[string]any{"people": people})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	previous := directoryPeopleTestDocument
	directoryPeopleTestDocument = string(document)
	t.Cleanup(func() { directoryPeopleTestDocument = previous })
}

// A flow state a test serves names the people on that board. The company knows
// the same people, so a test says so once by handing over the state it serves.
func useDirectoryPeopleOfTaskState(t *testing.T, stateDocument string) {
	t.Helper()
	var state struct {
		Members []taskMemberForTool `json:"members"`
	}
	if errorValue := json.Unmarshal([]byte(stateDocument), &state); errorValue != nil {
		t.Fatal(errorValue)
	}
	useDirectoryPeopleOf(t, state.Members)
}

func useDirectoryPeopleOfTaskStateAnd(t *testing.T, stateDocument string) string {
	t.Helper()
	useDirectoryPeopleOfTaskState(t, stateDocument)
	return stateDocument
}
