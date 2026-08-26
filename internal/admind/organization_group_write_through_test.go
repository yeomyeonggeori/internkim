package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

func isCompanyTeamRequest(request *http.Request) bool {
	return strings.HasPrefix(request.URL.String(), companyDirectoryURLForTest+"/api/agent/team")
}

func TestOrganizationGroupUpdateReachesTheCompanyDirectory(t *testing.T) {
	service := newLocalUsersTestService(t)
	seatPeopleInACompanyDirectoryForTest(t, service)
	var offered []map[string]any
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if response, isHandled := localOrganizationMattermostResponse(t, request); isHandled {
			return response, nil
		}
		if isCompanyTeamRequest(request) {
			var payload struct {
				Teams []map[string]any `json:"teams"`
			}
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			offered = payload.Teams
			return jsonResponse(http.StatusOK, `{"teams":[{"teamID":"team-product","name":"제품","parentTeamID":""}],"dropped":0}`, nil), nil
		}
		if isCompanyDirectoryRequest(request) {
			return companyDirectoryResponse(t, request)
		}
		if response, handled := companyPlumbingAnswerForTest(request); handled {
			return response, nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}

	requestBody := strings.NewReader(`{"groups":[{"id":"product","name":"제품"}]}`)
	responseRecorder := httptest.NewRecorder()
	service.localSetOrgGroups(responseRecorder, httptest.NewRequest(http.MethodPut, "/admin/api/org-groups", requestBody))

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if len(offered) != 1 || offered[0]["name"] != "제품" {
		t.Fatalf("the directory was offered %#v; a team made here has to reach it", offered)
	}

	groups, errorValue := service.readOrganizationGroups(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(groups) != 1 || groups[0].ID != "team-product" {
		t.Fatalf("groups = %#v; want the id the directory issued", groups)
	}
}

func TestRenamingATeamInTheDirectoryDoesNotMakeASecondGroup(t *testing.T) {
	held := []orgGroupRecord{{ID: "team-product", Name: "제품"}}
	members := []centralplane.Member{{Email: "one@example.com", TeamID: "team-product", TeamName: "프로덕트"}}

	renamed := groupsRenamedByTheDirectory(held, members)
	if len(renamed) != 1 || renamed[0].ID != "team-product" || renamed[0].Name != "프로덕트" {
		t.Fatalf("groups = %#v; a team keeps its id when it is renamed", renamed)
	}

	groupIDByName := map[string]string{strings.ToLower(renamed[0].Name): renamed[0].ID}
	heldIDs := map[string]bool{"team-product": true}
	missing := groupsTheDirectoryNamesAndTheDeviceLacks(members, groupIDByName, heldIDs)
	if len(missing) != 0 {
		t.Fatalf("missing = %#v; the renamed team is not a team the device lacks", missing)
	}
}

func TestGroupsAdoptTheTeamIDsTheDirectoryIssues(t *testing.T) {
	groups := []orgGroupRecord{{ID: "product", Name: "제품"}, {ID: "design", Name: "디자인", ParentID: "product"}}
	settled := []centralplane.Team{{TeamID: "team-product", Name: "제품"}, {TeamID: "team-design", Name: "디자인"}}

	adopted, renamedIDs := groupsAdoptingTeamIDs(groups, settled)

	if len(adopted) != 2 || adopted[0].ID != "team-product" || adopted[1].ID != "team-design" {
		t.Fatalf("adopted = %#v", adopted)
	}
	if adopted[1].ParentID != "team-product" {
		t.Fatalf("parent = %q; the hierarchy has to move with the ids", adopted[1].ParentID)
	}
	if renamedIDs["product"] != "team-product" || renamedIDs["design"] != "team-design" {
		t.Fatalf("renamed = %#v; people pointing at the old ids need to be moved", renamedIDs)
	}
}
