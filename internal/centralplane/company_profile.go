package centralplane

import (
	"context"
	"fmt"
	"strings"
)

// CompanyProfile is the master profile a document prints from, resolved for one
// language. It is the record's own answer, passed through unchanged.
type CompanyProfile struct {
	Language            string            `json:"language"`
	Name                string            `json:"name"`
	BrandName           string            `json:"brandName"`
	Slogan              string            `json:"slogan"`
	Description         string            `json:"description"`
	Representative      string            `json:"representative"`
	RepresentativeTitle string            `json:"representativeTitle"`
	Address             string            `json:"address"`
	OfficeAddress       string            `json:"officeAddress"`
	Jurisdiction        string            `json:"jurisdiction"`
	BankAccount         string            `json:"bankAccount"`
	LegalAttributes     map[string]string `json:"legalAttributes"`
	FoundedDate         string            `json:"foundedDate"`
	Capital             string            `json:"capital"`
	FiscalYearEnd       string            `json:"fiscalYearEnd"`
	EmployeeCount       int               `json:"employeeCount"`
	Phone               string            `json:"phone"`
	Fax                 string            `json:"fax"`
	Email               string            `json:"email"`
	Website             string            `json:"website"`
	MissingFields       []string          `json:"missingFields"`
	UpdatedAt           string            `json:"updatedAt"`
}

func (client *Client) CompanyProfile(ctx context.Context, requesterEmail string, language string) (CompanyProfile, error) {
	input := map[string]string{}
	if normalizedLanguage := strings.ToLower(strings.TrimSpace(language)); normalizedLanguage != "" {
		input["language"] = normalizedLanguage
	}
	var profile CompanyProfile
	if errorValue := client.runRecordTool(ctx, requesterEmail, "company_info_get", input, &profile); errorValue != nil {
		return CompanyProfile{}, errorValue
	}
	return profile, nil
}

// One language slot per call, which is the shape company_info_set takes: the
// localized fields go into the slot the call names.
func (client *Client) WriteCompanyProfile(ctx context.Context, requesterEmail string, values map[string]any) (CompanyProfile, error) {
	if strings.TrimSpace(fmt.Sprint(values["language"])) == "" {
		return CompanyProfile{}, fmt.Errorf("a company profile is written into one language slot, and this call named none")
	}
	var profile CompanyProfile
	if errorValue := client.runRecordTool(ctx, requesterEmail, "company_info_set", values, &profile); errorValue != nil {
		return CompanyProfile{}, errorValue
	}
	return profile, nil
}

// CompanySettings is what a company works in: the language every screen speaks
// and the time zone every date is read in.
type CompanySettings struct {
	Name         string `json:"name"`
	Country      string `json:"country"`
	Locale       string `json:"locale"`
	TimeZone     string `json:"timeZone"`
	CurrencyCode string `json:"currencyCode"`
}

func (client *Client) CompanySettings(ctx context.Context, requesterEmail string) (CompanySettings, error) {
	var settings CompanySettings
	if errorValue := client.runRecordTool(ctx, requesterEmail, "company_settings_get", map[string]any{}, &settings); errorValue != nil {
		return CompanySettings{}, errorValue
	}
	return settings, nil
}

func (client *Client) WriteCompanySettings(ctx context.Context, requesterEmail string, change map[string]any) (CompanySettings, error) {
	if len(change) == 0 {
		return CompanySettings{}, fmt.Errorf("a settings write names at least one setting")
	}
	var settings CompanySettings
	if errorValue := client.runRecordTool(ctx, requesterEmail, "company_settings_update", change, &settings); errorValue != nil {
		return CompanySettings{}, errorValue
	}
	return settings, nil
}
