package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const companyProfileCarryRecoveryAction = "company-profile-carry-into-the-record"
const companyProfileCarryTimeout = 30 * time.Second

type heldCompanyProfile struct {
	Name                map[string]string            `json:"name,omitempty"`
	BrandName           map[string]string            `json:"brandName,omitempty"`
	Slogan              map[string]string            `json:"slogan,omitempty"`
	Description         map[string]string            `json:"description,omitempty"`
	Representative      map[string]string            `json:"representative,omitempty"`
	RepresentativeTitle map[string]string            `json:"representativeTitle,omitempty"`
	Address             map[string]string            `json:"address,omitempty"`
	OfficeAddress       map[string]string            `json:"officeAddress,omitempty"`
	Jurisdiction        map[string]string            `json:"jurisdiction,omitempty"`
	BankAccount         map[string]string            `json:"bankAccount,omitempty"`
	LegalAttributes     map[string]map[string]string `json:"legalAttributes,omitempty"`
	FoundedDate         string                       `json:"foundedDate,omitempty"`
	Capital             string                       `json:"capital,omitempty"`
	FiscalYearEnd       string                       `json:"fiscalYearEnd,omitempty"`
	EmployeeCount       int                          `json:"employeeCount,omitempty"`
	Phone               string                       `json:"phone,omitempty"`
	Fax                 string                       `json:"fax,omitempty"`
	Email               string                       `json:"email,omitempty"`
	Website             string                       `json:"website,omitempty"`
}

func (service *Service) companyProfilePath() string {
	return filepath.Join(service.Configuration.StateDirectory, "company-info.json")
}

func (service *Service) heldCompanyProfile() (heldCompanyProfile, bool) {
	document, errorValue := os.ReadFile(service.companyProfilePath())
	if errorValue != nil {
		return heldCompanyProfile{}, false
	}
	var held heldCompanyProfile
	if json.Unmarshal(document, &held) != nil {
		return heldCompanyProfile{}, false
	}
	return held, true
}

// One slot per language, because company_info_set writes one slot per call. A
// language nothing was written in is not carried.
func (held heldCompanyProfile) languages() []string {
	seen := map[string]bool{}
	languages := []string{}
	for _, slot := range held.localizedFields() {
		for language, value := range slot {
			language = strings.ToLower(strings.TrimSpace(language))
			if language == "" || strings.TrimSpace(value) == "" || seen[language] {
				continue
			}
			seen[language] = true
			languages = append(languages, language)
		}
	}
	for language := range held.LegalAttributes {
		language = strings.ToLower(strings.TrimSpace(language))
		if language == "" || seen[language] {
			continue
		}
		seen[language] = true
		languages = append(languages, language)
	}
	return languages
}

func (held heldCompanyProfile) localizedFields() map[string]map[string]string {
	return map[string]map[string]string{
		"name": held.Name, "brandName": held.BrandName, "slogan": held.Slogan,
		"description": held.Description, "representative": held.Representative,
		"representativeTitle": held.RepresentativeTitle, "address": held.Address,
		"officeAddress": held.OfficeAddress, "jurisdiction": held.Jurisdiction,
		"bankAccount": held.BankAccount,
	}
}

func (held heldCompanyProfile) changeForLanguage(language string) map[string]any {
	change := map[string]any{"language": language}
	for field, slot := range held.localizedFields() {
		if value := strings.TrimSpace(slot[language]); value != "" {
			change[field] = value
		}
	}
	for field, value := range map[string]string{
		"foundedDate": held.FoundedDate, "capital": held.Capital, "fiscalYearEnd": held.FiscalYearEnd,
		"phone": held.Phone, "fax": held.Fax, "email": held.Email, "website": held.Website,
	} {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			change[field] = trimmed
		}
	}
	if held.EmployeeCount > 0 {
		change["employeeCount"] = held.EmployeeCount
	}
	if labels := held.LegalAttributes[language]; len(labels) > 0 {
		change["legalAttributes"] = legalAttributesDocument(labels)
	}
	return change
}

func legalAttributesDocument(labels map[string]string) string {
	document, errorValue := json.Marshal(labels)
	if errorValue != nil {
		return "{}"
	}
	return string(document)
}

type companyProfileCarryReport struct {
	Languages []string `json:"languages,omitempty"`
	Refused   []string `json:"refused,omitempty"`
}

// The master profile is the company's to write, and a carry names no person, so
// it writes as the administrator who claimed this device.
func (service *Service) carryTheCompanyProfileIntoTheRecord(ctx context.Context) (companyProfileCarryReport, error) {
	report := companyProfileCarryReport{}
	client := service.centralPlane()
	if client == nil {
		return report, errNoCompanyDirectory
	}
	administratorEmail := service.claimedAdminEmail()
	if administratorEmail == "" {
		return report, fmt.Errorf("no administrator is claimed here, and the master profile is an administrator's to record")
	}
	held, isHeld := service.heldCompanyProfile()
	if !isHeld {
		return report, nil
	}
	for _, language := range held.languages() {
		carryContext, cancel := context.WithTimeout(ctx, companyProfileCarryTimeout)
		_, errorValue := client.WriteCompanyProfile(carryContext, administratorEmail, held.changeForLanguage(language))
		cancel()
		if errorValue != nil {
			report.Refused = append(report.Refused, language+": "+errorValue.Error())
			continue
		}
		report.Languages = append(report.Languages, language)
	}
	return report, nil
}

// The file goes once every language in it has been written, and stays whole
// when any of them was refused: a half-carried profile is worse than a file.
func (service *Service) sweepTheCompanyProfileTheRecordNowHolds(ctx context.Context) {
	held, isHeld := service.heldCompanyProfile()
	if !isHeld {
		return
	}
	if service.centralPlane() == nil {
		slog.WarnContext(ctx, "this device names no company, so it keeps the master profile nobody else holds",
			"recovery_action", companyProfileCarryRecoveryAction)
		return
	}
	report, errorValue := service.carryTheCompanyProfileIntoTheRecord(ctx)
	if errorValue != nil || len(report.Refused) > 0 {
		slog.WarnContext(ctx, "the master profile this device holds was not taken whole, so the file stays",
			"languages", strings.Join(report.Languages, ","), "refused", strings.Join(report.Refused, "; "),
			"error", errorValue, "recovery_action", companyProfileCarryRecoveryAction)
		return
	}
	if len(held.languages()) == 0 {
		slog.InfoContext(ctx, "the master profile this device held said nothing, so it goes unwritten")
	}
	if errorValue := os.Remove(service.companyProfilePath()); errorValue != nil {
		slog.WarnContext(ctx, "the master profile the record now holds could not be removed", "error", errorValue)
		return
	}
	slog.InfoContext(ctx, "the record holds the master profile this device was keeping",
		"languages", strings.Join(report.Languages, ","))
}

func (service *Service) startCompanyProfileSweep(ctx context.Context) {
	go service.sweepTheCompanyProfileTheRecordNowHolds(ctx)
}

func (service *Service) handleCompanyProfileCarry(responseWriter http.ResponseWriter, request *http.Request) {
	report, errorValue := service.carryTheCompanyProfileIntoTheRecord(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.sweepTheCompanyProfileTheRecordNowHolds(request.Context())
	service.writeJSON(responseWriter, report)
}
