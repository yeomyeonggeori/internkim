package admind

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type googleCalDAVCalendarFixture struct {
	Path        string
	DisplayName string
	Color       string
	Components  []string
	Privileges  []string
}

func googleCalDAVFixtureTransport(t *testing.T, requestPaths *[]string, calendars []googleCalDAVCalendarFixture) roundTripFunc {
	t.Helper()
	return googleCalDAVFixtureTransportWithProbe(t, calendars, func(request *http.Request) {
		if requestPaths != nil {
			*requestPaths = append(*requestPaths, request.URL.EscapedPath())
		}
	})
}

func googleCalDAVFixtureTransportWithProbe(t *testing.T, calendars []googleCalDAVCalendarFixture, probe func(*http.Request)) roundTripFunc {
	t.Helper()
	return roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if probe != nil {
			probe(request)
		}
		if request.Header.Get("Authorization") != "Bearer access-1" {
			t.Fatalf("authorization header = %q", request.Header.Get("Authorization"))
		}
		if request.Method != "PROPFIND" {
			t.Fatalf("method = %s, want PROPFIND", request.Method)
		}
		switch request.URL.EscapedPath() {
		case "/caldav/v2/admin@example.com/user", "/caldav/v2/admin@example.com/user/":
			return googleCalDAVUserPrincipalOrHomeSetResponse(request), nil
		case "/caldav/v2/admin@example.com/":
			return xmlResponse(http.StatusMultiStatus, googleCalDAVCalendarListXML(calendars)), nil
		default:
			for _, calendar := range calendars {
				if request.URL.EscapedPath() == calendar.Path {
					return xmlResponse(http.StatusMultiStatus, googleCalDAVCTagXML(calendar.Path)), nil
				}
			}
			t.Fatalf("unexpected CalDAV request path: %s", request.URL.String())
			return nil, nil
		}
	})
}

func googleCalDAVUserPrincipalOrHomeSetResponse(request *http.Request) *http.Response {
	body, _ := io.ReadAll(request.Body)
	if strings.Contains(string(body), "current-user-principal") {
		return xmlResponse(http.StatusMultiStatus, googleCalDAVCurrentUserPrincipalXML())
	}
	return xmlResponse(http.StatusMultiStatus, googleCalDAVHomeSetXML())
}

func googleCalDAVCurrentUserPrincipalXML() string {
	return `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:">
  <D:response>
    <D:href>/caldav/v2/admin@example.com/user/</D:href>
    <D:propstat>
      <D:prop>
        <D:current-user-principal><D:href>/caldav/v2/admin@example.com/user/</D:href></D:current-user-principal>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`
}

func googleCalDAVHomeSetXML() string {
	return `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:response>
    <D:href>/caldav/v2/admin@example.com/user/</D:href>
    <D:propstat>
      <D:prop>
        <C:calendar-home-set><D:href>/caldav/v2/admin@example.com/</D:href></C:calendar-home-set>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`
}

func googleCalDAVCalendarListXML(calendars []googleCalDAVCalendarFixture) string {
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="utf-8"?><D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:I="http://apple.com/ns/ical/" xmlns:CS="http://calendarserver.org/ns/">`)
	for _, calendar := range calendars {
		builder.WriteString(`<D:response><D:href>`)
		builder.WriteString(calendar.Path)
		builder.WriteString(`</D:href><D:propstat><D:prop><D:resourcetype><D:collection/><C:calendar/></D:resourcetype><D:displayname>`)
		builder.WriteString(calendar.DisplayName)
		builder.WriteString(`</D:displayname>`)
		if calendar.Color != "" {
			builder.WriteString(`<I:calendar-color>`)
			builder.WriteString(calendar.Color)
			builder.WriteString(`</I:calendar-color>`)
		}
		builder.WriteString(`<CS:getctag>ctag-`)
		builder.WriteString(calendar.Path)
		builder.WriteString(`</CS:getctag><C:supported-calendar-component-set>`)
		for _, component := range calendar.Components {
			builder.WriteString(`<C:comp name="`)
			builder.WriteString(component)
			builder.WriteString(`"/>`)
		}
		builder.WriteString(`</C:supported-calendar-component-set><D:current-user-privilege-set>`)
		for _, privilege := range calendar.Privileges {
			builder.WriteString(`<D:privilege><D:`)
			builder.WriteString(privilege)
			builder.WriteString(`/></D:privilege>`)
		}
		builder.WriteString(`</D:current-user-privilege-set></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>`)
	}
	builder.WriteString(`</D:multistatus>`)
	return builder.String()
}

func googleCalDAVCTagXML(calendarPath string) string {
	return `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:CS="http://calendarserver.org/ns/">
  <D:response>
    <D:href>` + calendarPath + `</D:href>
    <D:propstat>
      <D:prop><CS:getctag>ctag-selected</CS:getctag></D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`
}

func xmlResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"application/xml"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func containsCalendarTestString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
