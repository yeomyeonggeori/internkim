package admind

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type localizedText map[string]string

type companyInfo struct {
	Name                localizedText                `json:"name,omitempty"`
	BrandName           localizedText                `json:"brandName,omitempty"`
	Slogan              localizedText                `json:"slogan,omitempty"`
	Description         localizedText                `json:"description,omitempty"`
	Representative      localizedText                `json:"representative,omitempty"`
	RepresentativeTitle localizedText                `json:"representativeTitle,omitempty"`
	Address             localizedText                `json:"address,omitempty"`
	OfficeAddress       localizedText                `json:"officeAddress,omitempty"`
	Jurisdiction        localizedText                `json:"jurisdiction,omitempty"`
	BankAccount         localizedText                `json:"bankAccount,omitempty"`
	LegalAttributes     map[string]map[string]string `json:"legalAttributes,omitempty"`
	FoundedDate         string                       `json:"foundedDate,omitempty"`
	Capital             string                       `json:"capital,omitempty"`
	FiscalYearEnd       string                       `json:"fiscalYearEnd,omitempty"`
	EmployeeCount       int                          `json:"employeeCount,omitempty"`
	Phone               string                       `json:"phone,omitempty"`
	Fax                 string                       `json:"fax,omitempty"`
	Email               string                       `json:"email,omitempty"`
	Website             string                       `json:"website,omitempty"`
	UpdatedAt           string                       `json:"updatedAt,omitempty"`
}

type companyInfoUpdate struct {
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
}

type companyInfoView struct {
	Language            string            `json:"language"`
	Name                string            `json:"name"`
	BrandName           string            `json:"brandName,omitempty"`
	Slogan              string            `json:"slogan,omitempty"`
	Description         string            `json:"description,omitempty"`
	Representative      string            `json:"representative"`
	RepresentativeTitle string            `json:"representativeTitle"`
	Address             string            `json:"address"`
	OfficeAddress       string            `json:"officeAddress,omitempty"`
	Jurisdiction        string            `json:"jurisdiction,omitempty"`
	BankAccount         string            `json:"bankAccount"`
	LegalAttributes     map[string]string `json:"legalAttributes"`
	FoundedDate         string            `json:"foundedDate,omitempty"`
	Capital             string            `json:"capital,omitempty"`
	FiscalYearEnd       string            `json:"fiscalYearEnd,omitempty"`
	EmployeeCount       int               `json:"employeeCount,omitempty"`
	Phone               string            `json:"phone"`
	Fax                 string            `json:"fax,omitempty"`
	Email               string            `json:"email"`
	Website             string            `json:"website,omitempty"`
	MissingFields       []string          `json:"missingFields"`
	UpdatedAt           string            `json:"updatedAt,omitempty"`
}

func defaultRepresentativeTitle(language string) string {
	if strings.EqualFold(language, "en") {
		return "CEO"
	}
	return "대표이사"
}

func (service *Service) writeCompanyInfo(responseWriter http.ResponseWriter, request *http.Request) {
	info, errorValue := service.readCompanyInfo()
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	language := strings.TrimSpace(request.URL.Query().Get("language"))
	if language == "" {
		service.writeJSON(responseWriter, info)
		return
	}
	service.writeJSON(responseWriter, resolveCompanyInfoView(info, language))
}

func (service *Service) updateCompanyInfo(responseWriter http.ResponseWriter, request *http.Request) {
	var update companyInfoUpdate
	if errorValue := json.NewDecoder(request.Body).Decode(&update); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	language := normalizeCompanyLanguage(update.Language)
	info, errorValue := service.readCompanyInfo()
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	applyCompanyInfoUpdate(&info, update, language)
	info.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if errorValue := service.writeCompanyInfoFile(info); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, resolveCompanyInfoView(info, language))
}

func applyCompanyInfoUpdate(info *companyInfo, update companyInfoUpdate, language string) {
	setLocalized(&info.Name, language, update.Name)
	setLocalized(&info.BrandName, language, update.BrandName)
	setLocalized(&info.Slogan, language, update.Slogan)
	setLocalized(&info.Description, language, update.Description)
	setLocalized(&info.Representative, language, update.Representative)
	setLocalized(&info.RepresentativeTitle, language, update.RepresentativeTitle)
	setLocalized(&info.Address, language, update.Address)
	setLocalized(&info.OfficeAddress, language, update.OfficeAddress)
	setLocalized(&info.Jurisdiction, language, update.Jurisdiction)
	setLocalized(&info.BankAccount, language, update.BankAccount)
	if len(update.LegalAttributes) > 0 {
		if info.LegalAttributes == nil {
			info.LegalAttributes = map[string]map[string]string{}
		}
		if info.LegalAttributes[language] == nil {
			info.LegalAttributes[language] = map[string]string{}
		}
		for label, value := range update.LegalAttributes {
			label = strings.TrimSpace(label)
			value = strings.TrimSpace(value)
			if label == "" {
				continue
			}
			if value == "" {
				delete(info.LegalAttributes[language], label)
				continue
			}
			info.LegalAttributes[language][label] = value
		}
	}
	setPlain(&info.FoundedDate, update.FoundedDate)
	setPlain(&info.Capital, update.Capital)
	setPlain(&info.FiscalYearEnd, update.FiscalYearEnd)
	if update.EmployeeCount > 0 {
		info.EmployeeCount = update.EmployeeCount
	}
	setPlain(&info.Phone, update.Phone)
	setPlain(&info.Fax, update.Fax)
	setPlain(&info.Email, update.Email)
	setPlain(&info.Website, update.Website)
}

func setLocalized(target *localizedText, language string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	if *target == nil {
		*target = localizedText{}
	}
	(*target)[language] = value
}

func setPlain(target *string, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		*target = value
	}
}

func resolveCompanyInfoView(info companyInfo, language string) companyInfoView {
	language = normalizeCompanyLanguage(language)
	missing := []string{}
	resolveCore := func(fieldName string, text localizedText) string {
		value, isExact := resolveLocalized(text, language)
		if !isExact {
			missing = append(missing, fieldName)
		}
		return value
	}
	view := companyInfoView{
		Language:            language,
		Name:                resolveCore("name", info.Name),
		BrandName:           resolveAnyLocalized(info.BrandName, language),
		Slogan:              resolveAnyLocalized(info.Slogan, language),
		Description:         resolveAnyLocalized(info.Description, language),
		Representative:      resolveCore("representative", info.Representative),
		RepresentativeTitle: resolveAnyLocalized(info.RepresentativeTitle, language),
		Address:             resolveCore("address", info.Address),
		OfficeAddress:       resolveAnyLocalized(info.OfficeAddress, language),
		Jurisdiction:        resolveAnyLocalized(info.Jurisdiction, language),
		BankAccount:         resolveCore("bankAccount", info.BankAccount),
		LegalAttributes:     resolveLegalAttributes(info.LegalAttributes, language),
		FoundedDate:         info.FoundedDate,
		Capital:             info.Capital,
		FiscalYearEnd:       info.FiscalYearEnd,
		EmployeeCount:       info.EmployeeCount,
		Phone:               info.Phone,
		Fax:                 info.Fax,
		Email:               info.Email,
		Website:             info.Website,
		UpdatedAt:           info.UpdatedAt,
	}
	if view.RepresentativeTitle == "" {
		view.RepresentativeTitle = defaultRepresentativeTitle(language)
	}
	if strings.TrimSpace(view.Phone) == "" {
		missing = append(missing, "phone")
	}
	if strings.TrimSpace(view.Email) == "" {
		missing = append(missing, "email")
	}
	view.MissingFields = missing
	return view
}

func resolveLocalized(text localizedText, language string) (string, bool) {
	if value := strings.TrimSpace(text[language]); value != "" {
		return value, true
	}
	return resolveAnyLocalized(text, language), false
}

func resolveAnyLocalized(text localizedText, language string) string {
	if value := strings.TrimSpace(text[language]); value != "" {
		return value
	}
	if value := strings.TrimSpace(text["ko"]); value != "" {
		return value
	}
	for _, value := range text {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func resolveLegalAttributes(attributes map[string]map[string]string, language string) map[string]string {
	if resolved := attributes[language]; len(resolved) > 0 {
		return resolved
	}
	if resolved := attributes["ko"]; len(resolved) > 0 {
		return resolved
	}
	for _, resolved := range attributes {
		if len(resolved) > 0 {
			return resolved
		}
	}
	return map[string]string{}
}

func normalizeCompanyLanguage(language string) string {
	language = strings.ToLower(strings.TrimSpace(language))
	if language == "" {
		return "ko"
	}
	return language
}

func (service *Service) readCompanyInfo() (companyInfo, error) {
	document, errorValue := os.ReadFile(service.companyInfoPath())
	if os.IsNotExist(errorValue) {
		return companyInfo{}, nil
	}
	if errorValue != nil {
		return companyInfo{}, errorValue
	}
	var info companyInfo
	if errorValue := json.Unmarshal(document, &info); errorValue != nil {
		return companyInfo{}, nil
	}
	return info, nil
}

func (service *Service) writeCompanyInfoFile(info companyInfo) error {
	document, errorValue := json.MarshalIndent(info, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	path := service.companyInfoPath()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return errorValue
	}
	temporaryPath := path + ".tmp"
	if errorValue := os.WriteFile(temporaryPath, append(document, '\n'), 0o600); errorValue != nil {
		return errorValue
	}
	return os.Rename(temporaryPath, path)
}

func (service *Service) companyInfoPath() string {
	return filepath.Join(service.Configuration.StateDirectory, "company-info.json")
}
