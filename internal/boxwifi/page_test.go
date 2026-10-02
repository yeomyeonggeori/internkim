package boxwifi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestPageRedirectsUnknownPathsToRoot(t *testing.T) {
	server := httptest.NewServer(newPage(nil, false, make(chan submission, 1)))
	defer server.Close()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	for _, path := range []string{"/hotspot-detect.html", "/generate_204"} {
		response, errorValue := client.Get(server.URL + path)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusFound || response.Header.Get("Location") != "/" {
			t.Fatalf("%s: status = %d, location = %q", path, response.StatusCode, response.Header.Get("Location"))
		}
	}
}

func TestPageRefusesAnEmptySSID(t *testing.T) {
	server := httptest.NewServer(newPage(nil, false, make(chan submission, 1)))
	defer server.Close()

	response, errorValue := http.PostForm(server.URL+"/join", url.Values{"ssid": {""}, "password": {"secret"}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusBadRequest)
	}
}

func TestPageAcceptsOneSubmissionAndRefusesTheNext(t *testing.T) {
	submitted := make(chan submission, 1)
	server := httptest.NewServer(newPage([]Network{{SSID: "Office"}}, false, submitted))
	defer server.Close()

	first := submitJoinTo(t, server.URL, "Office", "office-secret")
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first submit status = %d, want %d", first.StatusCode, http.StatusOK)
	}

	second := submitJoinTo(t, server.URL, "Office", "office-secret")
	if second.StatusCode != http.StatusConflict {
		t.Fatalf("second submit status = %d, want %d", second.StatusCode, http.StatusConflict)
	}

	select {
	case result := <-submitted:
		if result.ssid != "Office" || result.password != "office-secret" {
			t.Fatalf("submission = %+v", result)
		}
	default:
		t.Fatal("no submission was recorded")
	}
}

func TestPageNeverEchoesThePasswordOrATypedSSID(t *testing.T) {
	submitted := make(chan submission, 1)
	server := httptest.NewServer(newPage([]Network{{SSID: "Office"}}, false, submitted))
	defer server.Close()

	formResponse, errorValue := http.Get(server.URL + "/")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	formBody, errorValue := io.ReadAll(formResponse.Body)
	formResponse.Body.Close()
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	joinResponse, errorValue := http.PostForm(server.URL+"/join", url.Values{"customSSID": {"Hidden Office Net"}, "password": {"super-secret-password"}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	joinBody, errorValue := io.ReadAll(joinResponse.Body)
	joinResponse.Body.Close()
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if joinResponse.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", joinResponse.Header.Get("Cache-Control"))
	}
	for _, body := range [][]byte{formBody, joinBody} {
		if strings.Contains(string(body), "super-secret-password") {
			t.Fatalf("response echoed the password: %s", body)
		}
		if strings.Contains(string(body), "Hidden Office Net") {
			t.Fatalf("response echoed the typed ssid: %s", body)
		}
	}

	select {
	case result := <-submitted:
		if result.ssid != "Hidden Office Net" {
			t.Fatalf("ssid = %q, want the typed network to win over the selected one", result.ssid)
		}
	default:
		t.Fatal("no submission was recorded")
	}
}

func submitJoinTo(t *testing.T, baseURL, ssid, password string) *http.Response {
	t.Helper()
	response, errorValue := http.PostForm(baseURL+"/join", url.Values{"ssid": {ssid}, "password": {password}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	response.Body.Close()
	return response
}

func TestPageServesTheBuiltPageAndItsFiles(t *testing.T) {
	server := httptest.NewServer(newPage(nil, false, make(chan submission, 1)))
	defer server.Close()

	root := fetchBody(t, server.URL+"/")
	if !strings.Contains(root, `<div id="captive">`) || !strings.Contains(root, `src="/captive.js"`) {
		t.Fatalf("root did not serve the built setup page: %s", root)
	}
	for _, path := range []string{"/captive.js", "/index.css", "/logo.svg", "/favicon.svg"} {
		response, errorValue := http.Get(server.URL + path)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s: status = %d, want %d", path, response.StatusCode, http.StatusOK)
		}
	}
}

type captiveContract struct {
	Scanned          []Network       `json:"scanned"`
	HasJoinFailed    bool            `json:"hasJoinFailed"`
	NetworksResponse json.RawMessage `json:"networksResponse"`
	JoinFields       []string        `json:"joinFields"`
	JoinStatuses     map[string]int  `json:"joinStatuses"`
}

func TestPageAnswersTheContractTheSetupPageReads(t *testing.T) {
	contents, errorValue := os.ReadFile("testdata/captive-contract.json")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var contract captiveContract
	if errorValue := json.Unmarshal(contents, &contract); errorValue != nil {
		t.Fatal(errorValue)
	}
	server := httptest.NewServer(newPage(contract.Scanned, contract.HasJoinFailed, make(chan submission, 1)))
	defer server.Close()

	var answered, promised any
	if errorValue := json.Unmarshal([]byte(fetchBody(t, server.URL+"/networks")), &answered); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := json.Unmarshal(contract.NetworksResponse, &promised); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !reflect.DeepEqual(answered, promised) {
		t.Fatalf("/networks = %v, the page expects %v", answered, promised)
	}

	ssidField, customSSIDField, passwordField := contract.JoinFields[0], contract.JoinFields[1], contract.JoinFields[2]
	for _, attempt := range []struct {
		outcome string
		form    url.Values
	}{
		{"networkRequired", url.Values{ssidField: {""}, customSSIDField: {""}, passwordField: {"secret"}}},
		{"accepted", url.Values{ssidField: {"Office"}, customSSIDField: {""}, passwordField: {"secret"}}},
		{"alreadySubmitted", url.Values{ssidField: {"Office"}, customSSIDField: {""}, passwordField: {"secret"}}},
	} {
		response, errorValue := http.PostForm(server.URL+"/join", attempt.form)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		response.Body.Close()
		if response.StatusCode != contract.JoinStatuses[attempt.outcome] {
			t.Fatalf("%s: status = %d, the page expects %d", attempt.outcome, response.StatusCode, contract.JoinStatuses[attempt.outcome])
		}
	}
}

func fetchBody(t *testing.T, address string) string {
	t.Helper()
	response, errorValue := http.Get(address)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer response.Body.Close()
	body, errorValue := io.ReadAll(response.Body)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return string(body)
}

func TestPageKeepsTheSpacesOfAChosenNetworkName(t *testing.T) {
	submitted := make(chan submission, 1)
	server := httptest.NewServer(newPage([]Network{{SSID: "Office "}}, false, submitted))
	defer server.Close()

	submitJoinTo(t, server.URL, "Office ", "office-secret")

	if result := <-submitted; result.ssid != "Office " {
		t.Fatalf("ssid = %q, want %q", result.ssid, "Office ")
	}
}
