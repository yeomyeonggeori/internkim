package admind

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"
	"golang.org/x/oauth2"
)

func TestCalDAVBearerHTTPClientAttachesAuthorization(t *testing.T) {
	gotAuthorization := ""
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotAuthorization = request.Header.Get("Authorization")
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	source := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: "access-1",
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(time.Hour),
	})
	bearerClient := newCalDAVBearerHTTPClient(server.Client(), source)
	request, errorValue := http.NewRequest(http.MethodGet, server.URL+"/probe", nil)
	if errorValue != nil {
		t.Fatalf("new request: %v", errorValue)
	}
	response, errorValue := bearerClient.Do(request)
	if errorValue != nil {
		t.Fatalf("do: %v", errorValue)
	}
	_ = response.Body.Close()
	if gotAuthorization != "Bearer access-1" {
		t.Errorf("authorization: got %q, want %q", gotAuthorization, "Bearer access-1")
	}
}

func TestCalDAVBearerHTTPClientPropagatesTokenSourceError(t *testing.T) {
	failing := &failingTokenSource{message: "refresh failed"}
	bearerClient := newCalDAVBearerHTTPClient(http.DefaultClient, failing)
	request, _ := http.NewRequest(http.MethodGet, "http://example.invalid/", nil)
	if _, errorValue := bearerClient.Do(request); errorValue == nil {
		t.Fatal("expected token source error to surface")
	}
}

type failingTokenSource struct{ message string }

func (source *failingTokenSource) Token() (*oauth2.Token, error) {
	return nil, &tokenSourceError{message: source.message}
}

type tokenSourceError struct{ message string }

func (e *tokenSourceError) Error() string { return e.message }

func TestCalDAVClientPutCreatesNewObjectWithIfNoneMatchWildcard(t *testing.T) {
	gotHeaders := map[string]string{}
	gotBody := []byte(nil)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPut {
			t.Errorf("method: got %s", request.Method)
		}
		gotHeaders["If-None-Match"] = request.Header.Get("If-None-Match")
		gotHeaders["If-Match"] = request.Header.Get("If-Match")
		gotHeaders["Content-Type"] = request.Header.Get("Content-Type")
		body, _ := io.ReadAll(request.Body)
		gotBody = body
		writer.Header().Set("ETag", `"etag-created"`)
		writer.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := newTestOutboundCalDAVClient(t, server)
	etag, errorValue := client.putCalendarObject(context.Background(),
		"/calendars/me/events/abc.ics",
		[]byte("BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n"),
		"", caldavWildcardETag)
	if errorValue != nil {
		t.Fatalf("put: %v", errorValue)
	}
	if etag != `"etag-created"` {
		t.Errorf("etag: got %q", etag)
	}
	if gotHeaders["If-None-Match"] != "*" {
		t.Errorf("If-None-Match: got %q", gotHeaders["If-None-Match"])
	}
	if gotHeaders["If-Match"] != "" {
		t.Errorf("If-Match should be empty for create, got %q", gotHeaders["If-Match"])
	}
	if !strings.HasPrefix(gotHeaders["Content-Type"], "text/calendar") {
		t.Errorf("Content-Type: got %q", gotHeaders["Content-Type"])
	}
	if !strings.Contains(string(gotBody), "VCALENDAR") {
		t.Errorf("body not delivered: %q", string(gotBody))
	}
}

func TestCalDAVClientPutUpdatesExistingObjectWithIfMatch(t *testing.T) {
	gotIfMatch := ""
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotIfMatch = request.Header.Get("If-Match")
		writer.Header().Set("ETag", `"etag-updated"`)
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newTestOutboundCalDAVClient(t, server)
	etag, errorValue := client.putCalendarObject(context.Background(),
		"/calendars/me/events/abc.ics",
		[]byte("payload"),
		`"etag-old"`, "")
	if errorValue != nil {
		t.Fatalf("put: %v", errorValue)
	}
	if etag != `"etag-updated"` {
		t.Errorf("etag: got %q", etag)
	}
	if gotIfMatch != `"etag-old"` {
		t.Errorf("If-Match: got %q", gotIfMatch)
	}
}

func TestCalDAVClientPutRecoversCanonicalETagWithGet(t *testing.T) {
	const objectPath = "/calendars/me/events/canonical-etag.ics"
	const calendarData = "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n"
	requestMethods := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestMethods = append(requestMethods, request.Method)
		if request.URL.EscapedPath() != objectPath {
			t.Errorf("path: got %q, want %q", request.URL.EscapedPath(), objectPath)
		}
		switch request.Method {
		case http.MethodPut:
			writer.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			if request.Header.Get("Accept") != ical.MIMEType {
				t.Errorf("Accept: got %q, want %q", request.Header.Get("Accept"), ical.MIMEType)
			}
			writer.Header().Set("ETag", `"etag-canonical"`)
			_, _ = writer.Write([]byte(calendarData))
		default:
			t.Fatalf("unexpected method: %s", request.Method)
		}
	}))
	defer server.Close()

	client := newTestOutboundCalDAVClient(t, server)
	etag, errorValue := client.putCalendarObject(context.Background(), objectPath, []byte(calendarData), "", caldavWildcardETag)
	if errorValue != nil {
		t.Fatalf("put: %v", errorValue)
	}
	if etag != `"etag-canonical"` {
		t.Fatalf("etag: got %q, want canonical ETag", etag)
	}
	if strings.Join(requestMethods, ",") != http.MethodPut+","+http.MethodGet {
		t.Fatalf("request methods: %v", requestMethods)
	}
}

func TestCalDAVClientGetReturnsCanonicalCalendarObject(t *testing.T) {
	const objectPath = "/calendars/me/events/get-canonical.ics"
	const calendarData = "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Errorf("method: got %q, want GET", request.Method)
		}
		if request.URL.EscapedPath() != objectPath {
			t.Errorf("path: got %q, want %q", request.URL.EscapedPath(), objectPath)
		}
		if request.Header.Get("Accept") != ical.MIMEType {
			t.Errorf("Accept: got %q, want %q", request.Header.Get("Accept"), ical.MIMEType)
		}
		writer.Header().Set("ETag", `"etag-get"`)
		_, _ = writer.Write([]byte(calendarData))
	}))
	defer server.Close()

	client := newTestOutboundCalDAVClient(t, server)
	object, errorValue := client.getCalendarObject(context.Background(), objectPath)
	if errorValue != nil {
		t.Fatalf("get: %v", errorValue)
	}
	if object.Path != objectPath || object.ETag != `"etag-get"` || string(object.Data) != calendarData {
		t.Fatalf("object: %+v", object)
	}
}

func TestCalDAVClientQueriesCalendarObjectByUID(t *testing.T) {
	const calendarPath = "/calendars/me/"
	const objectPath = "/calendars/me/server-generated-42.ics"
	const eventUID = "query-by-uid@internkim"
	const calendarData = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//InternKim//Calendar//EN\r\nBEGIN:VEVENT\r\nUID:" + eventUID + "\r\nDTSTAMP:20260716T000000Z\r\nDTSTART:20260716T010000Z\r\nDTEND:20260716T020000Z\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	requestMethod := ""
	requestPath := ""
	requestDepth := ""
	requestBody := ""
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestMethod = request.Method
		requestPath = request.URL.EscapedPath()
		requestDepth = request.Header.Get("Depth")
		encodedRequest, _ := io.ReadAll(request.Body)
		requestBody = string(encodedRequest)
		writer.Header().Set("Content-Type", "application/xml; charset=utf-8")
		writer.WriteHeader(http.StatusMultiStatus)
		_, _ = io.WriteString(writer, `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:response>
    <D:href>`+objectPath+`</D:href>
    <D:propstat>
      <D:prop>
        <D:getetag>"etag-query"</D:getetag>
        <C:calendar-data><![CDATA[`+calendarData+`]]></C:calendar-data>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`)
	}))
	defer server.Close()

	client := newTestOutboundCalDAVClient(t, server)
	objects, errorValue := client.queryCalendarObjectsByUID(context.Background(), calendarPath, eventUID)
	if errorValue != nil {
		t.Fatalf("query by UID: %v", errorValue)
	}
	if requestMethod != "REPORT" || requestPath != calendarPath || requestDepth != "1" {
		t.Fatalf("request method=%q path=%q depth=%q", requestMethod, requestPath, requestDepth)
	}
	for _, expectedFragment := range []string{"calendar-query", "getetag", "calendar-data", `name="VCALENDAR"`, `name="VEVENT"`, `name="UID"`, "text-match", eventUID} {
		if !strings.Contains(requestBody, expectedFragment) {
			t.Errorf("REPORT body missing %q: %s", expectedFragment, requestBody)
		}
	}
	if len(objects) != 1 {
		t.Fatalf("objects=%+v", objects)
	}
	if objects[0].Path != objectPath || objects[0].ETag != "etag-query" || !strings.Contains(string(objects[0].Data), "UID:"+eventUID) {
		t.Fatalf("object=%+v data=%q", objects[0], string(objects[0].Data))
	}
}

func TestCalDAVClientPutReturnsPreconditionFailedOn412(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusPreconditionFailed)
	}))
	defer server.Close()

	client := newTestOutboundCalDAVClient(t, server)
	_, errorValue := client.putCalendarObject(context.Background(),
		"/calendars/me/events/abc.ics",
		[]byte("payload"),
		`"stale"`, "")
	if !isCalDAVPreconditionFailed(errorValue) {
		t.Fatalf("expected precondition failed, got %v", errorValue)
	}
}

func TestCalDAVClientPutReturnsPreconditionFailedOn409(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusConflict)
	}))
	defer server.Close()

	client := newTestOutboundCalDAVClient(t, server)
	_, errorValue := client.putCalendarObject(context.Background(),
		"/calendars/me/events/abc.ics",
		[]byte("payload"),
		"", caldavWildcardETag)
	if !isCalDAVPreconditionFailed(errorValue) {
		t.Fatalf("expected conflict to be handled as precondition failed, got %v", errorValue)
	}
}

func TestCalDAVClientPutSurfacesUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := newTestOutboundCalDAVClient(t, server)
	_, errorValue := client.putCalendarObject(context.Background(),
		"/calendars/me/events/abc.ics",
		[]byte("payload"),
		"", "")
	if errorValue == nil {
		t.Fatal("expected error for 500")
	}
	if isCalDAVPreconditionFailed(errorValue) {
		t.Fatalf("500 misclassified as precondition: %v", errorValue)
	}
}

func TestCalDAVClientDeleteSendsIfMatch(t *testing.T) {
	gotMethod := ""
	gotIfMatch := ""
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotMethod = request.Method
		gotIfMatch = request.Header.Get("If-Match")
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := newTestOutboundCalDAVClient(t, server)
	if errorValue := client.deleteCalendarObject(context.Background(),
		"/calendars/me/events/abc.ics", `"etag-current"`); errorValue != nil {
		t.Fatalf("delete: %v", errorValue)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method: got %s", gotMethod)
	}
	if gotIfMatch != `"etag-current"` {
		t.Errorf("If-Match: got %q", gotIfMatch)
	}
}

func TestCalDAVClientDeleteReturnsPreconditionFailedOn412(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusPreconditionFailed)
	}))
	defer server.Close()

	client := newTestOutboundCalDAVClient(t, server)
	errorValue := client.deleteCalendarObject(context.Background(),
		"/calendars/me/events/abc.ics", `"stale"`)
	if !isCalDAVPreconditionFailed(errorValue) {
		t.Fatalf("expected precondition failed, got %v", errorValue)
	}
}

func TestCalDAVClientDeleteTolerates404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := newTestOutboundCalDAVClient(t, server)
	if errorValue := client.deleteCalendarObject(context.Background(),
		"/calendars/me/events/already-gone.ics", ""); errorValue != nil {
		t.Errorf("404 should be tolerated, got %v", errorValue)
	}
}

func TestCalDAVClientPutMaps404ToObjectNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := newTestOutboundCalDAVClient(t, server)
	_, errorValue := client.putCalendarObject(context.Background(), "/calendars/me/events/gone.ics", []byte("calendar"), `"etag"`, "")
	if !isCalDAVObjectNotFound(errorValue) {
		t.Fatalf("expected object not found, got %v", errorValue)
	}
}

func TestCalDAVClientGetMaps404ToObjectNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := newTestOutboundCalDAVClient(t, server)
	_, errorValue := client.getCalendarObject(context.Background(), "/calendars/me/events/gone.ics")
	if !isCalDAVObjectNotFound(errorValue) {
		t.Fatalf("expected object not found, got %v", errorValue)
	}
}

func TestCalDAVClientAbsoluteURLHandlesPathVariants(t *testing.T) {
	client := &outboundCalDAVClient{endpoint: "https://caldav.example.com/base/"}
	cases := []struct {
		input    string
		expected string
	}{
		{"calendars/me/", "https://caldav.example.com/base/calendars/me/"},
		{"/calendars/me/", "https://caldav.example.com/calendars/me/"},
		{"https://caldav.example.com/x", "https://caldav.example.com/x"},
	}
	for _, testCase := range cases {
		got, errorValue := client.absoluteURL(testCase.input)
		if errorValue != nil {
			t.Errorf("absoluteURL(%q): %v", testCase.input, errorValue)
		}
		if got != testCase.expected {
			t.Errorf("absoluteURL(%q): got %q, want %q", testCase.input, got, testCase.expected)
		}
	}
}

func TestCalDAVPathOnlyPreservesEscapedCalendarSegments(t *testing.T) {
	got := calDAVPathOnly("https://apidata.googleusercontent.com/caldav/v2/company%2Fschedule%23shared@example.com/events/")
	want := "/caldav/v2/company%2Fschedule%23shared@example.com/events/"
	if got != want {
		t.Fatalf("calDAVPathOnly: got %q, want %q", got, want)
	}
}

func TestNewOutboundCalDAVClientRequiresEndpoint(t *testing.T) {
	if _, errorValue := newOutboundCalDAVClient("", http.DefaultClient); errorValue == nil {
		t.Fatal("expected error for empty endpoint")
	}
}

func newTestOutboundCalDAVClient(t *testing.T, server *httptest.Server) *outboundCalDAVClient {
	t.Helper()
	client, errorValue := newOutboundCalDAVClient(server.URL, server.Client())
	if errorValue != nil {
		t.Fatalf("client: %v", errorValue)
	}
	return client
}

func TestFormatETagHeaderValueWrapsUnquotedETag(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"  ", ""},
		{"*", "*"},
		{"63914708510", `"63914708510"`},
		{`"63914708510"`, `"63914708510"`},
		{`W/"abc"`, `W/"abc"`},
	}
	for _, testCase := range cases {
		got := formatETagHeaderValue(testCase.input)
		if got != testCase.want {
			t.Errorf("formatETagHeaderValue(%q) = %q, want %q", testCase.input, got, testCase.want)
		}
	}
}

func TestAbsoluteURLResolvesAbsolutePathAgainstPathfulEndpoint(t *testing.T) {
	cases := []struct {
		name     string
		endpoint string
		path     string
		want     string
	}{
		{
			name:     "absolute path replaces base path (Google CalDAV pattern)",
			endpoint: "https://apidata.googleusercontent.com/caldav/v2/user@example.com/user",
			path:     "/caldav/v2/user@example.com/events/abc.ics",
			want:     "https://apidata.googleusercontent.com/caldav/v2/user@example.com/events/abc.ics",
		},
		{
			name:     "endpoint without path keeps absolute path as-is",
			endpoint: "https://example.com",
			path:     "/caldav/foo.ics",
			want:     "https://example.com/caldav/foo.ics",
		},
		{
			name:     "fully qualified same-origin URL passes through",
			endpoint: "https://example.com/base",
			path:     "https://example.com/calendar/x.ics",
			want:     "https://example.com/calendar/x.ics",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			client, errorValue := newOutboundCalDAVClient(testCase.endpoint, http.DefaultClient)
			if errorValue != nil {
				t.Fatalf("client: %v", errorValue)
			}
			got, errorValue := client.absoluteURL(testCase.path)
			if errorValue != nil {
				t.Fatalf("absoluteURL: %v", errorValue)
			}
			if got != testCase.want {
				t.Errorf("got %q want %q", got, testCase.want)
			}
		})
	}
}

func TestCalDAVClientAbsoluteURLRejectsCrossHost(t *testing.T) {
	client := &outboundCalDAVClient{endpoint: "https://apidata.googleusercontent.com/caldav/v2/user@example.com/user/"}
	for _, candidate := range []string{
		"https://other.example.com/calendar/event.ics",
		"//other.example.com/calendar/event.ics",
		"http://apidata.googleusercontent.com/calendar/event.ics",
	} {
		if _, errorValue := client.absoluteURL(candidate); errorValue == nil {
			t.Fatalf("cross-origin URL accepted: %s", candidate)
		}
	}
}
