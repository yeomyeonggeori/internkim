package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/centralplane"
)

func (service *Service) companyProfile(ctx context.Context, requesterEmail string, language string) (centralplane.CompanyProfile, error) {
	client := service.centralPlane()
	if client == nil {
		return centralplane.CompanyProfile{}, errNoCompanyDirectory
	}
	return client.CompanyProfile(ctx, requesterEmail, language)
}

func (service *Service) writeCompanyInfo(responseWriter http.ResponseWriter, request *http.Request) {
	profile, errorValue := service.companyProfile(
		request.Context(),
		service.recordReaderEmail(request),
		normalizeCompanyLanguage(request.URL.Query().Get("language")),
	)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, profile)
}

func (service *Service) updateCompanyInfo(responseWriter http.ResponseWriter, request *http.Request) {
	var asked map[string]any
	if errorValue := json.NewDecoder(request.Body).Decode(&asked); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	client := service.centralPlane()
	if client == nil {
		http.Error(responseWriter, errNoCompanyDirectory.Error(), http.StatusBadGateway)
		return
	}
	change, errorValue := companyProfileChange(asked)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	profile, errorValue := client.WriteCompanyProfile(request.Context(), service.recordReaderEmail(request), change)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	if errorValue := service.syncCompanySnapshotToBlueclaw(request.Context(), profile); errorValue != nil {
		fmt.Printf("warning: company snapshot policy sync failed: %v\n", errorValue)
	}
	service.writeJSON(responseWriter, profile)
}

// company_info_set takes country-specific labels as a JSON object string, and
// this route has always taken them as an object, so the one is written as the
// other rather than the caller being asked to change.
func companyProfileChange(asked map[string]any) (map[string]any, error) {
	change := map[string]any{}
	for field, value := range asked {
		if field == "legalAttributes" {
			continue
		}
		change[field] = value
	}
	language, _ := asked["language"].(string)
	change["language"] = normalizeCompanyLanguage(language)
	attributes, hasAttributes := asked["legalAttributes"]
	if !hasAttributes || attributes == nil {
		return change, nil
	}
	document, errorValue := json.Marshal(attributes)
	if errorValue != nil {
		return nil, errorValue
	}
	change["legalAttributes"] = string(document)
	return change, nil
}

func companyProfileSnapshot(profile centralplane.CompanyProfile) map[string]string {
	return map[string]string{
		"name":           strings.TrimSpace(profile.Name),
		"brandName":      strings.TrimSpace(profile.BrandName),
		"slogan":         strings.TrimSpace(profile.Slogan),
		"description":    strings.TrimSpace(profile.Description),
		"representative": strings.TrimSpace(profile.Representative),
		"website":        strings.TrimSpace(profile.Website),
	}
}

func (service *Service) reconcileCompanySnapshot(ctx context.Context, policyDocument map[string]any) {
	service.writeCompanySettingsIntoPolicy(ctx, policyDocument)
	profile, errorValue := service.companyProfile(ctx, service.claimedAdminEmail(), "")
	if errorValue != nil {
		return
	}
	writeCompanyProfileIntoPolicy(policyDocument, profile)
}

func (service *Service) writeCompanySettingsIntoPolicy(ctx context.Context, policyDocument map[string]any) {
	settings, isAnswered := service.readCompanySettings(ctx)
	if !isAnswered {
		return
	}
	company := companySnapshotIn(policyDocument)
	company["timeZone"] = settings.timeZone
	company["locale"] = settings.language
}

func writeCompanyProfileIntoPolicy(policyDocument map[string]any, profile centralplane.CompanyProfile) {
	company := companySnapshotIn(policyDocument)
	for field, value := range companyProfileSnapshot(profile) {
		company[field] = value
	}
}

func companySnapshotIn(policyDocument map[string]any) map[string]any {
	company, held := policyDocument["company"].(map[string]any)
	if !held {
		company = map[string]any{}
		policyDocument["company"] = company
	}
	return company
}

func (service *Service) syncCompanySnapshotToBlueclaw(ctx context.Context, profile centralplane.CompanyProfile) error {
	var policyDocument map[string]any
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		return errorValue
	}
	writeCompanyProfileIntoPolicy(policyDocument, profile)
	service.writeCompanySettingsIntoPolicy(ctx, policyDocument)
	return service.deliverBlueclawPolicy(ctx, policyDocument)
}

func normalizeCompanyLanguage(language string) string {
	normalized := strings.ToLower(strings.TrimSpace(language))
	if normalized == "" {
		return "en"
	}
	return normalized
}

// The share page prints the profile in several languages, and the record
// resolves one language per answer.
func (service *Service) companyProfilesByLanguage(ctx context.Context, languages []string) (map[string]centralplane.CompanyProfile, error) {
	profiles := map[string]centralplane.CompanyProfile{}
	administratorEmail := service.claimedAdminEmail()
	for _, language := range languages {
		profile, errorValue := service.companyProfile(ctx, administratorEmail, language)
		if errorValue != nil {
			return nil, errorValue
		}
		profiles[language] = profile
	}
	return profiles, nil
}
