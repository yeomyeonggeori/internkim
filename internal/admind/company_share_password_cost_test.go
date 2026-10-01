package admind

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestCompanySharePasswordIsHashedAtTheOWASPCost(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir()})
	settings := saveCompanyShareTestSettings(t, service, "a-chosen-password")
	if cost := companySharePasswordHashCost(t, settings.PasswordHash); cost < companySharePasswordCost {
		t.Fatalf("company share password hashed at bcrypt cost %d, want at least %d", cost, companySharePasswordCost)
	}
}

func TestCompanyShareUnlockStrengthensAHashFromBeforeTheCostRose(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir()})
	settings := publishCompanyShareWithHashCost(t, service, "a-chosen-password", bcrypt.DefaultCost)

	cookie := unlockCompanyShareWith(t, service, "a-chosen-password")

	stored, errorValue := service.readCompanyShareAccessState()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if cost := companySharePasswordHashCost(t, stored.PasswordHash); cost != companySharePasswordCost {
		t.Fatalf("after unlocking, bcrypt cost = %d, want %d", cost, companySharePasswordCost)
	}
	if bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte("a-chosen-password")) != nil {
		t.Fatal("the strengthened hash no longer opens with the password it was made from")
	}
	if stored.AccessVersion != settings.AccessVersion {
		t.Fatalf("strengthening the hash moved the access version from %d to %d, which signs everyone out", settings.AccessVersion, stored.AccessVersion)
	}
	contentRequest := httptest.NewRequest(http.MethodGet, "/company/api/content", nil)
	contentRequest.AddCookie(cookie)
	contentResponse := httptest.NewRecorder()
	service.writeCompanyShareContent(contentResponse, contentRequest)
	if contentResponse.Code != http.StatusOK {
		t.Fatalf("session issued alongside the upgrade answered %d", contentResponse.Code)
	}
}

func TestCompanyShareUnlockDoesNotStrengthenOverARotatedPassword(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir()})
	settings := publishCompanyShareWithHashCost(t, service, "the-old-password", bcrypt.DefaultCost)
	rotated := saveCompanyShareTestSettings(t, service, "the-new-password")

	service.strengthenCompanySharePasswordHash(settings.PasswordHash, "the-old-password")

	stored, errorValue := service.readCompanyShareAccessState()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if stored.PasswordHash != rotated.PasswordHash {
		t.Fatal("an unlock that verified the old password wrote its hash over the rotated one")
	}
}

func publishCompanyShareWithHashCost(t *testing.T, service *Service, password string, cost int) companyShareSettings {
	t.Helper()
	settings := saveCompanyShareTestSettings(t, service, password)
	weakerHash, errorValue := bcrypt.GenerateFromPassword([]byte(password), cost)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	settings.PasswordHash = string(weakerHash)
	snapshot := companyShareSnapshot{Revision: 1, PublishedAt: time.Now().UTC().Format(time.RFC3339), Profiles: map[string]companyShareProfile{"ko": {Name: "테스트 회사"}}}
	if errorValue := service.writeCompanyShareSnapshotFile(snapshot); errorValue != nil {
		t.Fatal(errorValue)
	}
	settings.PublishedAt = snapshot.PublishedAt
	settings.PublicationRevision = 1
	if errorValue := service.writeCompanyShareSettingsFile(settings); errorValue != nil {
		t.Fatal(errorValue)
	}
	return settings
}

func unlockCompanyShareWith(t *testing.T, service *Service, password string) *http.Cookie {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/company/api/unlock", strings.NewReader(`{"password":"`+password+`"}`))
	request.RemoteAddr = "198.51.100.7:443"
	response := httptest.NewRecorder()
	service.unlockCompanyShare(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("unlock status = %d, body = %s", response.Code, response.Body.String())
	}
	return response.Result().Cookies()[0]
}

func companySharePasswordHashCost(t *testing.T, passwordHash string) int {
	t.Helper()
	cost, errorValue := bcrypt.Cost([]byte(passwordHash))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return cost
}
