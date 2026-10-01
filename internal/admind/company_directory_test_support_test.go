package admind

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/centralplane"
)

const companyDirectoryURLForTest = "https://app.example.test"

const companyProjectURLForTest = "https://project.example.test"

func seatPeopleInACompanyDirectoryForTest(t *testing.T, service *Service) {
	t.Helper()
	service.centralPlaneOnce = sync.Once{}
	service.centralPlaneClient = nil
	service.Configuration.CentralPlaneAppURL = companyDirectoryURLForTest
	service.Configuration.CentralPlaneProjectURL = companyProjectURLForTest
	service.Configuration.CentralPlanePublishableKey = "publishable"
	service.Configuration.CentralPlaneAgentKeyPath = writeAgentKeyForTest(t, "agent-key")
	service.Configuration.CentralPlaneAppURLPath = filepath.Join(t.TempDir(), "central-plane-app-url")
}

func isCompanyDirectoryRequest(request *http.Request) bool {
	return strings.HasPrefix(request.URL.String(), companyDirectoryURLForTest+"/api/agent/member")
}

// A test that seats people in a company also wakes every path that reads the
// record. One that only cares who works here answers those with a failure the
// code has to live with, never with a fatal.
func isRecordRequestOutsideTheDirectory(request *http.Request) bool {
	requestURL := request.URL.String()
	return !isCompanyDirectoryRequest(request) &&
		(strings.HasPrefix(requestURL, companyDirectoryURLForTest+"/") || strings.HasPrefix(requestURL, companyProjectURLForTest+"/"))
}

// The company's member door, held in memory: it answers who works here, keeps a
// write the way the real door does (a field nobody offered keeps what it held),
// and withdraws somebody by marking the row rather than dropping it.
type companyDirectoryForTest struct {
	members   []centralplane.Member
	writes    []centralplane.MemberWrite
	withdrawn []string
}

func companyDirectoryHolding(members ...centralplane.Member) *companyDirectoryForTest {
	return &companyDirectoryForTest{members: members}
}

func (directory *companyDirectoryForTest) respond(t *testing.T, request *http.Request) (*http.Response, error) {
	t.Helper()
	switch request.Method {
	case http.MethodGet:
		return directory.answerWhoWorksHere(t, request)
	case http.MethodPost:
		return directory.keepWrite(t, request)
	case http.MethodDelete:
		return directory.withdraw(t, request)
	default:
		return jsonResponse(http.StatusMethodNotAllowed, `{}`, nil), nil
	}
}

func (directory *companyDirectoryForTest) answerWhoWorksHere(t *testing.T, request *http.Request) (*http.Response, error) {
	t.Helper()
	email := strings.ToLower(strings.TrimSpace(request.URL.Query().Get("email")))
	if email == "" {
		return jsonResponse(http.StatusOK, directoryJSONForTest(t, map[string]any{"members": directory.members}), nil), nil
	}
	index := directory.indexOf(email)
	if index < 0 {
		return jsonResponse(http.StatusOK, `{"member":null}`, nil), nil
	}
	return jsonResponse(http.StatusOK, directoryJSONForTest(t, map[string]any{"member": directory.members[index]}), nil), nil
}

func (directory *companyDirectoryForTest) keepWrite(t *testing.T, request *http.Request) (*http.Response, error) {
	t.Helper()
	var write centralplane.MemberWrite
	if errorValue := json.NewDecoder(request.Body).Decode(&write); errorValue != nil {
		t.Fatal(errorValue)
	}
	write.Email = strings.ToLower(strings.TrimSpace(write.Email))
	directory.writes = append(directory.writes, write)
	index := directory.indexOf(write.Email)
	if index < 0 {
		directory.members = append(directory.members, centralplane.Member{
			MemberID: memberIDForTest(write.Email), Email: write.Email, Role: centralplane.MemberRoleMember, Status: centralplane.MemberStatusActive,
		})
		index = len(directory.members) - 1
	}
	held := &directory.members[index]
	if write.Name != "" {
		held.Name = write.Name
	}
	if write.Role != "" {
		held.Role = write.Role
	}
	if write.Note != "" {
		held.Note = write.Note
	}
	return jsonResponse(http.StatusOK, directoryJSONForTest(t, map[string]any{"member": *held}), nil), nil
}

func (directory *companyDirectoryForTest) withdraw(t *testing.T, request *http.Request) (*http.Response, error) {
	t.Helper()
	email := strings.ToLower(strings.TrimSpace(request.URL.Query().Get("email")))
	index := directory.indexOf(email)
	if index < 0 {
		return jsonResponse(http.StatusNotFound, `{"message":"nobody here goes by that address"}`, nil), nil
	}
	directory.withdrawn = append(directory.withdrawn, email)
	directory.members[index].Status = centralplane.MemberStatusWithdrawn
	return jsonResponse(http.StatusOK, directoryJSONForTest(t, map[string]any{"member": directory.members[index]}), nil), nil
}

func (directory *companyDirectoryForTest) indexOf(email string) int {
	for index, member := range directory.members {
		if strings.EqualFold(member.Email, email) {
			return index
		}
	}
	return -1
}

func directoryJSONForTest(t *testing.T, answer map[string]any) string {
	t.Helper()
	document, errorValue := json.Marshal(answer)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return string(document)
}

func memberForTest(email string, name string, role string) centralplane.Member {
	return centralplane.Member{MemberID: memberIDForTest(email), Email: email, Name: name, Role: role, Status: centralplane.MemberStatusActive}
}

func memberJSONForTest(email string) string {
	return `{"memberID":"` + memberIDForTest(email) + `","email":"` + email + `","role":"member","status":"active"}`
}

func memberIDForTest(email string) string {
	return "member-" + strings.ReplaceAll(strings.Split(email, "@")[0], ".", "-")
}
