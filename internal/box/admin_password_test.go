package box

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

type reportedAdminPassword struct {
	settingID string
	result    string
}

type fakeAdminPasswordPlane struct {
	pending   *PendingAdminPassword
	unclaimed bool
	reports   []reportedAdminPassword
}

func (plane *fakeAdminPasswordPlane) serve(t *testing.T) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/box/admin-password":
			if plane.unclaimed {
				writer.WriteHeader(http.StatusNotFound)
				return
			}
			json.NewEncoder(writer).Encode(map[string]any{"companyID": sampleCompanyID, "change": plane.pending})
		case "/api/box/admin-password/outcome":
			var body struct {
				SettingID string `json:"settingID"`
				Result    string `json:"result"`
			}
			json.NewDecoder(request.Body).Decode(&body)
			plane.reports = append(plane.reports, reportedAdminPassword{settingID: body.SettingID, result: body.Result})
			writer.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request to %s", request.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func adminPasswordDaemon(t *testing.T, plane *fakeAdminPasswordPlane, set func(context.Context, string) error) Daemon {
	t.Helper()
	return Daemon{
		Client:           Client{AppURL: plane.serve(t).URL},
		Places:           Places{StateDirectoryPath: filepath.Join(t.TempDir(), "box")},
		SetAdminPassword: set,
	}
}

func pendingAdminPasswordFor(t *testing.T, identity Identity, settingID, password string) *PendingAdminPassword {
	t.Helper()
	purpose := AdminPasswordPurpose(sampleCompanyID, identity.EncryptionPublicKey(), settingID)
	return &PendingAdminPassword{SettingID: settingID, Sealed: sealSecretTo(t, identity, password, purpose)}
}

func TestAPendingAdminPasswordIsOpenedSetAndReportedAsApplied(t *testing.T) {
	identity := freshTestIdentity(t)
	plane := &fakeAdminPasswordPlane{}
	plane.pending = pendingAdminPasswordFor(t, identity, "setting-1", "correct horse: battery")
	var set []string
	daemon := adminPasswordDaemon(t, plane, func(ctx context.Context, password string) error {
		set = append(set, password)
		return nil
	})

	handledSettingID, handledResult := daemon.checkForAdminPassword(context.Background(), identity, "", "")

	if len(set) != 1 || set[0] != "correct horse: battery" {
		t.Fatalf("set the admin password to %q", set)
	}
	if handledSettingID != "setting-1" || handledResult != AdminPasswordApplied {
		t.Fatalf("handled %q as %q", handledSettingID, handledResult)
	}
	if len(plane.reports) != 1 || plane.reports[0] != (reportedAdminPassword{settingID: "setting-1", result: "applied"}) {
		t.Fatalf("reported %+v", plane.reports)
	}
}

func TestAnAdminPasswordAlreadySetIsReportedAgainWithoutSettingItTwice(t *testing.T) {
	identity := freshTestIdentity(t)
	plane := &fakeAdminPasswordPlane{}
	plane.pending = pendingAdminPasswordFor(t, identity, "setting-1", "correct horse battery")
	calls := 0
	daemon := adminPasswordDaemon(t, plane, func(context.Context, string) error {
		calls++
		return nil
	})

	daemon.checkForAdminPassword(context.Background(), identity, "setting-1", AdminPasswordApplied)

	if calls != 0 {
		t.Fatalf("set the same admin password %d more times", calls)
	}
	if len(plane.reports) != 1 || plane.reports[0] != (reportedAdminPassword{settingID: "setting-1", result: "applied"}) {
		t.Fatalf("reported %+v", plane.reports)
	}
}

func TestAnAdminPasswordThatDoesNotOpenOrHoldsALineBreakIsReportedAsFailed(t *testing.T) {
	identity := freshTestIdentity(t)
	sealedForAnotherSetting := pendingAdminPasswordFor(t, identity, "setting-0", "correct horse battery")
	sealedForAnotherSetting.SettingID = "setting-1"
	for name, pending := range map[string]*PendingAdminPassword{
		"sealed for another setting": sealedForAnotherSetting,
		"holding a line break":       pendingAdminPasswordFor(t, identity, "setting-1", "first line\nroot::0:0"),
		"empty":                      pendingAdminPasswordFor(t, identity, "setting-1", ""),
	} {
		plane := &fakeAdminPasswordPlane{pending: pending}
		daemon := adminPasswordDaemon(t, plane, func(context.Context, string) error {
			t.Errorf("%s: the admin password was set", name)
			return nil
		})

		_, handledResult := daemon.checkForAdminPassword(context.Background(), identity, "", "")

		if handledResult != AdminPasswordFailed {
			t.Errorf("%s: handled as %q, want failed", name, handledResult)
		}
	}
}

func TestAnAdminPasswordTheSystemRefusesIsReportedAsFailed(t *testing.T) {
	identity := freshTestIdentity(t)
	plane := &fakeAdminPasswordPlane{}
	plane.pending = pendingAdminPasswordFor(t, identity, "setting-1", "correct horse battery")
	daemon := adminPasswordDaemon(t, plane, func(context.Context, string) error { return errors.New("chpasswd: user admin does not exist") })

	_, handledResult := daemon.checkForAdminPassword(context.Background(), identity, "", "")

	if handledResult != AdminPasswordFailed || len(plane.reports) != 1 || plane.reports[0].result != "failed" {
		t.Fatalf("handled as %q, reported %+v", handledResult, plane.reports)
	}
}

func TestAnUnclaimedBoxHasNoAdminPasswordToSet(t *testing.T) {
	identity := freshTestIdentity(t)
	plane := &fakeAdminPasswordPlane{unclaimed: true}
	daemon := adminPasswordDaemon(t, plane, func(context.Context, string) error {
		t.Fatal("an unclaimed box set an admin password")
		return nil
	})

	daemon.checkForAdminPassword(context.Background(), identity, "", "")

	if len(plane.reports) != 0 {
		t.Fatalf("reported %+v", plane.reports)
	}
}

type adminPasswordFixture struct {
	Password     string       `json:"password"`
	BoxSecretKey string       `json:"boxSecretKey"`
	CompanyID    string       `json:"companyID"`
	SettingID    string       `json:"settingID"`
	Sealed       SealedSecret `json:"sealed"`
}

func TestBoxOpensTheAdminPasswordTheBrowserSealed(t *testing.T) {
	document, errorValue := os.ReadFile(filepath.Join("testdata", "sealed-admin-password.json"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var fixture adminPasswordFixture
	if errorValue := json.Unmarshal(document, &fixture); errorValue != nil {
		t.Fatal(errorValue)
	}

	identity := identityWithEncryptionSeed(t, fixture.BoxSecretKey)
	opened, errorValue := identity.OpenSecret(fixture.Sealed, AdminPasswordPurpose(fixture.CompanyID, identity.EncryptionPublicKey(), fixture.SettingID))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if opened != fixture.Password {
		t.Fatalf("opened %q, want %q", opened, fixture.Password)
	}
}
