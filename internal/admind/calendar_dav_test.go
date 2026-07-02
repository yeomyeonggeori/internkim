package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"
)

func TestCalendarDAVAcceptsTokenBasicAuth(t *testing.T) {
	service := newCalendarTestService(t)
	syncRequest := httptest.NewRequest(http.MethodGet, "/calendar/api/sync", nil)
	syncRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	syncResponse := httptest.NewRecorder()
	service.router().ServeHTTP(syncResponse, syncRequest)
	if syncResponse.Code != http.StatusOK {
		t.Fatalf("sync status = %d body = %s", syncResponse.Code, syncResponse.Body.String())
	}
	var syncDocument calendarSyncResponse
	if errorValue := json.Unmarshal(syncResponse.Body.Bytes(), &syncDocument); errorValue != nil {
		t.Fatal(errorValue)
	}

	authorizedRequest := httptest.NewRequest("PROPFIND", calendarCollectionPath, nil)
	authorizedRequest.RemoteAddr = "203.0.113.10:49152"
	authorizedRequest.SetBasicAuth(calendarDAVUsername, syncDocument.CalDAVPassword)
	if !service.authorizeCalendarRequest(authorizedRequest) {
		t.Fatal("calendar token basic auth was rejected")
	}

	rejectedRequest := httptest.NewRequest("PROPFIND", calendarCollectionPath, nil)
	rejectedRequest.RemoteAddr = "203.0.113.10:49152"
	rejectedRequest.SetBasicAuth(calendarDAVUsername, "wrong-token")
	if service.authorizeCalendarRequest(rejectedRequest) {
		t.Fatal("wrong calendar token was accepted")
	}
}

func TestCalendarDAVBackendStoresCalendarObjects(t *testing.T) {
	service := newCalendarTestService(t)
	backend := calendarDAVBackend{service: service}
	calendar := newCalendarDocument()
	event := ical.NewEvent()
	event.Props.SetText(ical.PropUID, "client-event@example.com")
	event.Props.SetDateTime(ical.PropDateTimeStamp, time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC))
	event.Props.SetText(ical.PropSummary, "Client event")
	event.Props.SetDateTime(ical.PropDateTimeStart, time.Date(2026, 5, 8, 3, 0, 0, 0, time.UTC))
	event.Props.SetDateTime(ical.PropDateTimeEnd, time.Date(2026, 5, 8, 4, 0, 0, 0, time.UTC))
	calendar.Children = append(calendar.Children, event.Component)

	object, errorValue := backend.PutCalendarObject(context.Background(), calendarCollectionPath+"client-event.ics", calendar, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if object.Path != calendarCollectionPath+"client-event.ics" || object.ETag == "" {
		t.Fatalf("stored object = %#v", object)
	}
	storedObject, errorValue := backend.GetCalendarObject(context.Background(), calendarCollectionPath+"client-event.ics", nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if storedObject.Data.Events()[0].Props.Get(ical.PropSummary).Value != "Client event" {
		t.Fatalf("stored summary = %#v", storedObject.Data.Events()[0].Props.Get(ical.PropSummary))
	}
	if errorValue := backend.DeleteCalendarObject(context.Background(), calendarCollectionPath+"client-event.ics"); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := backend.GetCalendarObject(context.Background(), calendarCollectionPath+"client-event.ics", nil); errorValue == nil {
		t.Fatal("deleted calendar object was returned")
	}
}

func TestCalendarDAVPutAcceptsCurrentIfMatch(t *testing.T) {
	service := newCalendarTestService(t)
	backend := calendarDAVBackend{service: service}
	objectPath := calendarCollectionPath + "ifmatch-current.ics"

	initial := newCalendarDocumentWithEvent("ifmatch-current@example.com", "Original")
	stored, errorValue := backend.PutCalendarObject(context.Background(), objectPath, initial, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	updated := newCalendarDocumentWithEvent("ifmatch-current@example.com", "Updated")
	if _, errorValue := backend.PutCalendarObject(context.Background(), objectPath, updated, &caldav.PutCalendarObjectOptions{
		IfMatch: webdav.ConditionalMatch(`"` + stored.ETag + `"`),
	}); errorValue != nil {
		t.Fatalf("update with current ETag failed: %v", errorValue)
	}

	current, errorValue := backend.GetCalendarObject(context.Background(), objectPath, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if current.Data.Events()[0].Props.Get(ical.PropSummary).Value != "Updated" {
		t.Fatalf("expected updated summary, got %s", current.Data.Events()[0].Props.Get(ical.PropSummary).Value)
	}
}

func TestCalendarDAVPutRejectsStaleIfMatch(t *testing.T) {
	service := newCalendarTestService(t)
	backend := calendarDAVBackend{service: service}
	objectPath := calendarCollectionPath + "ifmatch-stale.ics"

	initial := newCalendarDocumentWithEvent("ifmatch-stale@example.com", "Original")
	if _, errorValue := backend.PutCalendarObject(context.Background(), objectPath, initial, nil); errorValue != nil {
		t.Fatal(errorValue)
	}

	updated := newCalendarDocumentWithEvent("ifmatch-stale@example.com", "Overwrite")
	if _, errorValue := backend.PutCalendarObject(context.Background(), objectPath, updated, &caldav.PutCalendarObjectOptions{
		IfMatch: webdav.ConditionalMatch(`"stale-etag-xyz"`),
	}); errorValue == nil {
		t.Fatal("expected stale If-Match to be rejected")
	}

	stored, errorValue := backend.GetCalendarObject(context.Background(), objectPath, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if stored.Data.Events()[0].Props.Get(ical.PropSummary).Value != "Original" {
		t.Fatalf("stale PUT overwrote data: got summary %s", stored.Data.Events()[0].Props.Get(ical.PropSummary).Value)
	}
}

func TestCalendarDAVPutHTTPRejectsStaleIfMatch(t *testing.T) {
	service := newCalendarTestService(t)
	objectPath := calendarCollectionPath + "http-stale.ics"

	initial := encodeCalendarTestICS(t, "http-stale@example.com", "Original")
	createRequest := httptest.NewRequest(http.MethodPut, objectPath, strings.NewReader(initial))
	createRequest.Header.Set("Content-Type", "text/calendar")
	createRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	createResponse := httptest.NewRecorder()
	service.router().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated && createResponse.Code != http.StatusNoContent && createResponse.Code != http.StatusOK {
		t.Fatalf("initial PUT status = %d body = %s", createResponse.Code, createResponse.Body.String())
	}

	updated := encodeCalendarTestICS(t, "http-stale@example.com", "Overwrite")
	updateRequest := httptest.NewRequest(http.MethodPut, objectPath, strings.NewReader(updated))
	updateRequest.Header.Set("Content-Type", "text/calendar")
	updateRequest.Header.Set("If-Match", `"stale-etag-xyz"`)
	updateRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	updateResponse := httptest.NewRecorder()
	service.router().ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale If-Match: expected 412, got %d body = %s", updateResponse.Code, updateResponse.Body.String())
	}

	stored, errorValue := calendarDAVBackend{service: service}.GetCalendarObject(context.Background(), objectPath, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if stored.Data.Events()[0].Props.Get(ical.PropSummary).Value != "Original" {
		t.Fatalf("stale PUT must not overwrite event")
	}
}

func TestCalendarDAVPutHTTPRejectsIfNoneMatchWildcardWhenExists(t *testing.T) {
	service := newCalendarTestService(t)
	objectPath := calendarCollectionPath + "http-ifnonematch.ics"

	initial := encodeCalendarTestICS(t, "http-ifnonematch@example.com", "Original")
	createRequest := httptest.NewRequest(http.MethodPut, objectPath, strings.NewReader(initial))
	createRequest.Header.Set("Content-Type", "text/calendar")
	createRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	createResponse := httptest.NewRecorder()
	service.router().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated && createResponse.Code != http.StatusNoContent && createResponse.Code != http.StatusOK {
		t.Fatalf("initial PUT status = %d body = %s", createResponse.Code, createResponse.Body.String())
	}

	replacement := encodeCalendarTestICS(t, "http-ifnonematch@example.com", "Replacement")
	replaceRequest := httptest.NewRequest(http.MethodPut, objectPath, strings.NewReader(replacement))
	replaceRequest.Header.Set("Content-Type", "text/calendar")
	replaceRequest.Header.Set("If-None-Match", "*")
	replaceRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	replaceResponse := httptest.NewRecorder()
	service.router().ServeHTTP(replaceResponse, replaceRequest)
	if replaceResponse.Code != http.StatusPreconditionFailed {
		t.Fatalf("If-None-Match: * with existing object: expected 412, got %d body = %s", replaceResponse.Code, replaceResponse.Body.String())
	}
}

func encodeCalendarTestICS(t *testing.T, uid string, summary string) string {
	t.Helper()
	cal := newCalendarDocumentWithEvent(uid, summary)
	var buffer bytes.Buffer
	if errorValue := ical.NewEncoder(&buffer).Encode(cal); errorValue != nil {
		t.Fatal(errorValue)
	}
	return buffer.String()
}

func TestCalendarDAVPutRejectsIfNoneMatchWildcardWhenExists(t *testing.T) {
	service := newCalendarTestService(t)
	backend := calendarDAVBackend{service: service}
	objectPath := calendarCollectionPath + "ifnonematch.ics"

	initial := newCalendarDocumentWithEvent("ifnonematch@example.com", "Original")
	if _, errorValue := backend.PutCalendarObject(context.Background(), objectPath, initial, nil); errorValue != nil {
		t.Fatal(errorValue)
	}

	replacement := newCalendarDocumentWithEvent("ifnonematch@example.com", "Replacement")
	if _, errorValue := backend.PutCalendarObject(context.Background(), objectPath, replacement, &caldav.PutCalendarObjectOptions{
		IfNoneMatch: webdav.ConditionalMatch("*"),
	}); errorValue == nil {
		t.Fatal("expected If-None-Match wildcard to fail for existing event")
	}
}

type calendarPropFindResult struct {
	Href     string
	OK       map[xml.Name]string
	NotFound []xml.Name
}

func parseCalendarMultistatusOrFatal(t *testing.T, body []byte) []calendarPropFindResult {
	t.Helper()
	results, errorValue := parseCalendarMultistatus(body)
	if errorValue != nil {
		t.Fatalf("parse multistatus failed: %v\nbody=%s", errorValue, string(body))
	}
	return results
}

func parseCalendarMultistatus(body []byte) ([]calendarPropFindResult, error) {
	type rawPropertyXML struct {
		XMLName  xml.Name
		InnerXML string `xml:",innerxml"`
	}
	type propXML struct {
		Properties []rawPropertyXML `xml:",any"`
	}
	type propstatXML struct {
		Prop   propXML `xml:"DAV: prop"`
		Status string  `xml:"DAV: status"`
	}
	type responseXML struct {
		Href      string        `xml:"DAV: href"`
		Propstats []propstatXML `xml:"DAV: propstat"`
	}
	type multistatusXML struct {
		XMLName   xml.Name      `xml:"DAV: multistatus"`
		Responses []responseXML `xml:"DAV: response"`
	}
	var document multistatusXML
	if errorValue := xml.Unmarshal(body, &document); errorValue != nil {
		return nil, errorValue
	}
	results := make([]calendarPropFindResult, 0, len(document.Responses))
	for _, response := range document.Responses {
		entry := calendarPropFindResult{
			Href: strings.TrimSpace(response.Href),
			OK:   map[xml.Name]string{},
		}
		for _, propstat := range response.Propstats {
			isOK, isNotFound := false, false
			if statusFields := strings.Fields(propstat.Status); len(statusFields) >= 2 {
				switch statusFields[1] {
				case "200":
					isOK = true
				case "404":
					isNotFound = true
				}
			}
			for _, property := range propstat.Prop.Properties {
				switch {
				case isOK:
					entry.OK[property.XMLName] = strings.TrimSpace(property.InnerXML)
				case isNotFound:
					entry.NotFound = append(entry.NotFound, property.XMLName)
				}
			}
		}
		results = append(results, entry)
	}
	return results, nil
}

func containsName(names []xml.Name, target xml.Name) bool {
	for _, name := range names {
		if name == target {
			return true
		}
	}
	return false
}

func newCalendarDocumentWithEvent(uid string, summary string) *ical.Calendar {
	calendar := newCalendarDocument()
	event := ical.NewEvent()
	event.Props.SetText(ical.PropUID, uid)
	event.Props.SetDateTime(ical.PropDateTimeStamp, time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC))
	event.Props.SetText(ical.PropSummary, summary)
	event.Props.SetDateTime(ical.PropDateTimeStart, time.Date(2026, 5, 12, 3, 0, 0, 0, time.UTC))
	event.Props.SetDateTime(ical.PropDateTimeEnd, time.Date(2026, 5, 12, 4, 0, 0, 0, time.UTC))
	calendar.Children = append(calendar.Children, event.Component)
	return calendar
}
