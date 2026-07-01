package admind

import (
	"context"
	"errors"
	"strings"

	"github.com/emersion/go-webdav"
)

const googleCalDAVBaseURL = "https://apidata.googleusercontent.com/caldav/v2"

type calendarProvider interface {
	Name() string
	Endpoint(account remoteCalendarAccount) string
	BuildHTTPClient(ctx context.Context, service *Service, account remoteCalendarAccount) (webdav.HTTPClient, error)
	Discover(ctx context.Context, service *Service, account remoteCalendarAccount) (remoteCalendarAccount, error)
}

type googleCalendarProvider struct{}

func (provider googleCalendarProvider) Name() string {
	return remoteCalendarProviderGoogle
}

func (provider googleCalendarProvider) Endpoint(account remoteCalendarAccount) string {
	return googleCalDAVBaseURL + "/" + strings.TrimSpace(account.AccountEmail) + "/user/"
}

func (provider googleCalendarProvider) BuildHTTPClient(ctx context.Context, service *Service, account remoteCalendarAccount) (webdav.HTTPClient, error) {
	tokenSource, errorValue := service.googleOAuthTokenSource(ctx, account, "")
	if errorValue != nil {
		return nil, errorValue
	}
	return newCalDAVBearerHTTPClient(service.googleOAuthHTTPClient(), tokenSource), nil
}

func (provider googleCalendarProvider) Discover(ctx context.Context, service *Service, account remoteCalendarAccount) (remoteCalendarAccount, error) {
	email := strings.TrimSpace(account.AccountEmail)
	if email == "" {
		return account, errors.New("account email required for google calendar discovery")
	}
	base := googleCalDAVBaseURL + "/" + email
	return service.saveCalendarDiscovery(ctx, account, base+"/user", base+"/", base+"/events/")
}
