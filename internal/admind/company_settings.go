package admind

import (
	"context"
	"strings"
	"sync"
	"time"
)

// The zone and the language are read once per calendar event, once per
// message, and once per roster reconcile, so the company is asked at most this
// often and the last answer stands in between.
const companySettingsFreshFor = 30 * time.Second

const defaultCompanyTimeZone = "Asia/Seoul"

type companySettings struct {
	timeZone string
	language string
}

type heldCompanySettings struct {
	mutex    sync.Mutex
	settings companySettings
	readAt   time.Time
	isHeld   bool
}

func (service *Service) readCompanySettings(ctx context.Context) (companySettings, bool) {
	if settings, isFresh := service.freshCompanySettings(); isFresh {
		return settings, true
	}
	client := service.centralPlane()
	if client == nil {
		return service.lastCompanySettings()
	}
	company, found, errorValue := client.Company(ctx)
	if errorValue != nil || !found {
		return service.lastCompanySettings()
	}
	return service.holdCompanySettings(companySettings{
		timeZone: loadableTimeZoneName(company.Timezone),
		language: workspaceLanguageOf(company.Locale),
	}), true
}

func (service *Service) freshCompanySettings() (companySettings, bool) {
	service.companySettingsCache.mutex.Lock()
	defer service.companySettingsCache.mutex.Unlock()
	isFresh := service.companySettingsCache.isHeld &&
		time.Since(service.companySettingsCache.readAt) < companySettingsFreshFor
	return service.companySettingsCache.settings, isFresh
}

func (service *Service) lastCompanySettings() (companySettings, bool) {
	service.companySettingsCache.mutex.Lock()
	defer service.companySettingsCache.mutex.Unlock()
	if !service.companySettingsCache.isHeld {
		return companySettings{language: workspaceLanguageKorean}, false
	}
	return service.companySettingsCache.settings, true
}

func (service *Service) holdCompanySettings(settings companySettings) companySettings {
	service.companySettingsCache.mutex.Lock()
	defer service.companySettingsCache.mutex.Unlock()
	service.companySettingsCache.settings = settings
	service.companySettingsCache.readAt = time.Now()
	service.companySettingsCache.isHeld = true
	return settings
}

func loadableTimeZoneName(timeZone string) string {
	named := strings.TrimSpace(timeZone)
	if named == "" {
		return ""
	}
	if _, errorValue := time.LoadLocation(named); errorValue != nil {
		return ""
	}
	return named
}

// Every day boundary this device decides is the company's, and a company it
// could not ask keeps the zone this product has always counted days in.
func (service *Service) companyTimeLocation(ctx context.Context) *time.Location {
	settings, _ := service.readCompanySettings(ctx)
	location, errorValue := time.LoadLocation(settings.timeZone)
	if errorValue != nil || settings.timeZone == "" {
		return defaultCompanyLocation()
	}
	return location
}

func (service *Service) companyTimeZoneName(ctx context.Context) string {
	return service.companyTimeLocation(ctx).String()
}

func (service *Service) companyDateNow(ctx context.Context) time.Time {
	return time.Now().In(service.companyTimeLocation(ctx))
}

func (service *Service) workspaceLanguage(ctx context.Context) string {
	settings, _ := service.readCompanySettings(ctx)
	return settings.language
}

func defaultCompanyLocation() *time.Location {
	location, errorValue := time.LoadLocation(defaultCompanyTimeZone)
	if errorValue != nil {
		return time.FixedZone(defaultCompanyTimeZone, 9*60*60)
	}
	return location
}
