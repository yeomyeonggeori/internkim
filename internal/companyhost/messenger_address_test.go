package companyhost

import (
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func messengerConnectionForTest(appURL string) Connection {
	return Connection{
		AppURL:       appURL,
		Company:      Company{ID: "00000000-0000-4000-8000-000000000001", Name: "Acme", Slug: "acme"},
		CentralPlane: CentralPlane{ProjectURL: "https://example.supabase.test", PublishableKey: "publishable"},
		GatewayURL:   "wss://gateway.example.test",
	}
}

func TestTheMessengerIsNamedByTheSlugOnTheZonePeopleSignInAt(t *testing.T) {
	connection := messengerConnectionForTest("https://intern.example.test")
	if host := MessengerHost(connection); host != "acme.intern.example.test" {
		t.Fatalf("the messenger host is %q", host)
	}
	if address := MessengerURL(connection); address != "wss://acme.intern.example.test" {
		t.Fatalf("the messenger address is %q", address)
	}
	if media := messengerMediaBaseURL(connection); media != "https://acme.intern.example.test/media" {
		t.Fatalf("the media address is %q", media)
	}
}

func TestAPlaneServedWithoutTLSNamesItsMessengerWithoutTLS(t *testing.T) {
	connection := messengerConnectionForTest("http://localhost:5173")
	if address := MessengerURL(connection); address != "ws://acme.localhost" {
		t.Fatalf("the messenger address is %q", address)
	}
	if media := messengerMediaBaseURL(connection); media != "http://acme.localhost/media" {
		t.Fatalf("the media address is %q", media)
	}
}

func TestTheRelayChatdAndAdmindAllAnswerToTheMessengerAddress(t *testing.T) {
	layout := blueclaw.MacCompanyHostLayout(homebrewPrefixForTest)
	connection := messengerConnectionForTest("https://intern.example.test")
	files := map[string]string{}
	for path := range environmentFilesForTest(layout) {
		files[path] = ""
	}
	hostEnvironment, errorValue := environmentFileText(companyEnvironment(layout, "secret", connection))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	files[blueclaw.CompanyHostEnvironmentPath] = hostEnvironment
	files[blueclaw.CompanyHostSettingsPath] = blueclaw.CompanyHostSettingsFile()
	daemons, errorValue := blueclaw.CompanyHostLaunchDaemons(layout, files)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	expected := map[string][]string{
		blueclaw.BuzzRelayServiceName: {
			plistSetting("RELAY_URL", "wss://acme.intern.example.test"),
			plistSetting("BUZZ_MEDIA_BASE_URL", "https://acme.intern.example.test/media"),
		},
		blueclaw.ChatdServiceName: {
			plistSetting("CHATD_BUZZ_RELAY_URL", "wss://acme.intern.example.test"),
			plistSetting("CHATD_BUZZ_RELAY_DIAL_URL", blueclaw.BuzzRelayLocalURL),
		},
		blueclaw.AdmindServiceName: {"<string>-buzz-relay-public-url</string>\n\t\t<string>wss://acme.intern.example.test</string>"},
	}
	for _, daemon := range daemons {
		for _, fragment := range expected[daemon.ServiceName] {
			if !strings.Contains(daemon.Contents, fragment) {
				t.Errorf("%s does not carry %q:\n%s", daemon.ServiceName, fragment, daemon.Contents)
			}
		}
	}
}

func plistSetting(name, value string) string {
	return "<key>" + name + "</key>\n\t\t<string>" + value + "</string>"
}
