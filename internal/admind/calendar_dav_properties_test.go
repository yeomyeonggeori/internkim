package admind

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCalendarPropPatchStoresAppleColor(t *testing.T) {
	service := newCalendarTestService(t)
	body := `<?xml version="1.0" encoding="utf-8"?>
<D:propertyupdate xmlns:D="DAV:" xmlns:A="http://apple.com/ns/ical/">
  <D:set><D:prop><A:calendar-color>#FF0000</A:calendar-color></D:prop></D:set>
</D:propertyupdate>`
	request := httptest.NewRequest("PROPPATCH", calendarCollectionPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/xml")
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusMultiStatus {
		t.Fatalf("proppatch status = %d body = %s", response.Code, response.Body.String())
	}
	properties, errorValue := service.readCalendarProperties(context.Background(), calendarCollectionPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(properties) != 1 {
		t.Fatalf("properties = %#v", properties)
	}
	if properties[0].XMLName.Space != "http://apple.com/ns/ical/" || properties[0].XMLName.Local != "calendar-color" || properties[0].Value != "#FF0000" {
		t.Fatalf("stored property mismatch: %#v", properties[0])
	}
}

func TestCalendarPropPatchStoresDAVDisplayName(t *testing.T) {
	service := newCalendarTestService(t)
	body := `<?xml version="1.0" encoding="utf-8"?>
<D:propertyupdate xmlns:D="DAV:">
  <D:set><D:prop><D:displayname>Team Calendar</D:displayname></D:prop></D:set>
</D:propertyupdate>`
	request := httptest.NewRequest("PROPPATCH", calendarCollectionPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/xml")
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusMultiStatus {
		t.Fatalf("proppatch status = %d body = %s", response.Code, response.Body.String())
	}
	properties, errorValue := service.readCalendarProperties(context.Background(), calendarCollectionPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(properties) != 1 || properties[0].XMLName.Space != "DAV:" || properties[0].XMLName.Local != "displayname" || properties[0].Value != "Team Calendar" {
		t.Fatalf("stored properties mismatch: %#v", properties)
	}
}

func TestCalendarPropPatchRejectsNonWhitelistedNamespace(t *testing.T) {
	service := newCalendarTestService(t)
	body := `<?xml version="1.0" encoding="utf-8"?>
<D:propertyupdate xmlns:D="DAV:" xmlns:M="http://mozilla.org/ns/calendar/">
  <D:set><D:prop><M:calendar-color>#00FF00</M:calendar-color></D:prop></D:set>
</D:propertyupdate>`
	request := httptest.NewRequest("PROPPATCH", calendarCollectionPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/xml")
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusMultiStatus {
		t.Fatalf("proppatch status = %d body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "403 Forbidden") {
		t.Fatalf("response missing 403 propstat for non-whitelisted property: %s", response.Body.String())
	}
	properties, errorValue := service.readCalendarProperties(context.Background(), calendarCollectionPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(properties) != 0 {
		t.Fatalf("non-whitelisted property must not be persisted: %#v", properties)
	}
}

func TestCalendarPropPatchRejectsNestedXMLValue(t *testing.T) {
	service := newCalendarTestService(t)
	body := `<?xml version="1.0" encoding="utf-8"?>
<D:propertyupdate xmlns:D="DAV:" xmlns:A="http://apple.com/ns/ical/">
  <D:set><D:prop><A:calendar-color><A:child>x</A:child></A:calendar-color></D:prop></D:set>
</D:propertyupdate>`
	request := httptest.NewRequest("PROPPATCH", calendarCollectionPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/xml")
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusMultiStatus {
		t.Fatalf("proppatch status = %d body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "403 Forbidden") {
		t.Fatalf("response missing 403 propstat for nested xml value: %s", response.Body.String())
	}
	properties, errorValue := service.readCalendarProperties(context.Background(), calendarCollectionPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(properties) != 0 {
		t.Fatalf("calendar-color with nested element must not be persisted: %#v", properties)
	}
}

func TestCalendarPropPatchDecodesXMLEntitiesAndRoundTrips(t *testing.T) {
	service := newCalendarTestService(t)
	patchBody := `<?xml version="1.0" encoding="utf-8"?>
<D:propertyupdate xmlns:D="DAV:">
  <D:set><D:prop><D:displayname>R&amp;D Team</D:displayname></D:prop></D:set>
</D:propertyupdate>`
	patchRequest := httptest.NewRequest("PROPPATCH", calendarCollectionPath, strings.NewReader(patchBody))
	patchRequest.Header.Set("Content-Type", "application/xml")
	patchRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	patchResponse := httptest.NewRecorder()
	service.router().ServeHTTP(patchResponse, patchRequest)
	if patchResponse.Code != http.StatusMultiStatus {
		t.Fatalf("proppatch status = %d body = %s", patchResponse.Code, patchResponse.Body.String())
	}

	storedProperties, errorValue := service.readCalendarProperties(context.Background(), calendarCollectionPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(storedProperties) != 1 || storedProperties[0].Value != "R&D Team" {
		t.Fatalf("stored value mismatch: %#v", storedProperties)
	}

	findBody := `<?xml version="1.0" encoding="utf-8"?>
<D:propfind xmlns:D="DAV:"><D:prop><D:displayname/></D:prop></D:propfind>`
	findRequest := httptest.NewRequest("PROPFIND", calendarCollectionPath, strings.NewReader(findBody))
	findRequest.Header.Set("Content-Type", "application/xml")
	findRequest.Header.Set("Depth", "0")
	findRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	findResponse := httptest.NewRecorder()
	service.router().ServeHTTP(findResponse, findRequest)
	if findResponse.Code != http.StatusMultiStatus {
		t.Fatalf("propfind status = %d body = %s", findResponse.Code, findResponse.Body.String())
	}
	if strings.Contains(findResponse.Body.String(), "&amp;amp;") {
		t.Fatalf("propfind response double-escaped XML entity:\n%s", findResponse.Body.String())
	}
	results := parseCalendarMultistatusOrFatal(t, findResponse.Body.Bytes())
	if len(results) != 1 {
		t.Fatalf("expected 1 response entry, got %d", len(results))
	}
	displayName := xml.Name{Space: "DAV:", Local: "displayname"}
	rawValue, ok := results[0].OK[displayName]
	if !ok {
		t.Fatalf("displayname missing from 200 OK propstat: %#v", results[0])
	}
	if rawValue != "R&amp;D Team" {
		t.Fatalf("displayname inner XML = %q, want %q (single-escaped)", rawValue, "R&amp;D Team")
	}
}

func TestCalendarPropPatchRemovesProperty(t *testing.T) {
	service := newCalendarTestService(t)
	if errorValue := service.writeCalendarProperty(context.Background(), calendarCollectionPath, "http://apple.com/ns/ical/", "calendar-color", "#FF0000"); errorValue != nil {
		t.Fatal(errorValue)
	}
	body := `<?xml version="1.0" encoding="utf-8"?>
<D:propertyupdate xmlns:D="DAV:" xmlns:A="http://apple.com/ns/ical/">
  <D:remove><D:prop><A:calendar-color/></D:prop></D:remove>
</D:propertyupdate>`
	request := httptest.NewRequest("PROPPATCH", calendarCollectionPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/xml")
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusMultiStatus {
		t.Fatalf("proppatch status = %d body = %s", response.Code, response.Body.String())
	}
	properties, errorValue := service.readCalendarProperties(context.Background(), calendarCollectionPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(properties) != 0 {
		t.Fatalf("property was not removed: %#v", properties)
	}
}

func TestCalendarPropFindIncludesStoredProperties(t *testing.T) {
	service := newCalendarTestService(t)
	if errorValue := service.writeCalendarProperty(context.Background(), calendarCollectionPath, "http://apple.com/ns/ical/", "calendar-color", "#FF0000"); errorValue != nil {
		t.Fatal(errorValue)
	}
	body := `<?xml version="1.0" encoding="utf-8"?>
<D:propfind xmlns:D="DAV:" xmlns:A="http://apple.com/ns/ical/">
  <D:prop><A:calendar-color/></D:prop>
</D:propfind>`
	request := httptest.NewRequest("PROPFIND", calendarCollectionPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/xml")
	request.Header.Set("Depth", "0")
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusMultiStatus {
		t.Fatalf("propfind status = %d body = %s", response.Code, response.Body.String())
	}
	results := parseCalendarMultistatusOrFatal(t, response.Body.Bytes())
	if len(results) != 1 {
		t.Fatalf("expected 1 response entry, got %d", len(results))
	}
	if results[0].Href != calendarCollectionPath {
		t.Fatalf("href mismatch: %q", results[0].Href)
	}
	colorName := xml.Name{Space: "http://apple.com/ns/ical/", Local: "calendar-color"}
	value, ok := results[0].OK[colorName]
	if !ok {
		t.Fatalf("calendar-color missing from 200 OK propstat: %#v", results[0])
	}
	if value != "#FF0000" {
		t.Fatalf("calendar-color value = %q, want %q", value, "#FF0000")
	}
}

func TestCalendarSyncCollectionRejectedAsUnsupported(t *testing.T) {
	service := newCalendarTestService(t)
	body := `<?xml version="1.0" encoding="utf-8"?>
<D:sync-collection xmlns:D="DAV:">
  <D:sync-token/><D:sync-level>1</D:sync-level><D:prop><D:getetag/></D:prop>
</D:sync-collection>`
	request := httptest.NewRequest("REPORT", calendarCollectionPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/xml")
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("sync-collection status = %d body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "supported-report") {
		t.Fatalf("response missing supported-report precondition: %s", response.Body.String())
	}
	if strings.Contains(response.Body.String(), "valid-sync-token") {
		t.Fatalf("response must not signal valid-sync-token (would loop client retries): %s", response.Body.String())
	}
}

func TestCalendarPropFindIncludesGetCTag(t *testing.T) {
	service := newCalendarTestService(t)
	body := `<?xml version="1.0" encoding="utf-8"?>
<D:propfind xmlns:D="DAV:" xmlns:CS="http://calendarserver.org/ns/">
  <D:prop><CS:getctag/></D:prop>
</D:propfind>`
	request := httptest.NewRequest("PROPFIND", calendarCollectionPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/xml")
	request.Header.Set("Depth", "0")
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusMultiStatus {
		t.Fatalf("propfind status = %d body = %s", response.Code, response.Body.String())
	}
	results := parseCalendarMultistatusOrFatal(t, response.Body.Bytes())
	if len(results) != 1 {
		t.Fatalf("expected 1 response entry, got %d", len(results))
	}
	ctagName := xml.Name{Space: "http://calendarserver.org/ns/", Local: "getctag"}
	value, ok := results[0].OK[ctagName]
	if !ok {
		t.Fatalf("getctag missing from 200 OK propstat: %#v", results[0])
	}
	if !strings.HasPrefix(value, "v1-") {
		t.Fatalf("getctag value %q missing v1- prefix", value)
	}
}

func TestCalendarPropFindAdvertisesCalendarHomeSet(t *testing.T) {
	service := newCalendarTestService(t)
	body := `<?xml version="1.0" encoding="utf-8"?>
<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop><C:calendar-home-set/><C:calendar-user-address-set/><D:current-user-principal/></D:prop>
</D:propfind>`
	request := httptest.NewRequest("PROPFIND", calendarCollectionPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/xml")
	request.Header.Set("Depth", "0")
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusMultiStatus {
		t.Fatalf("propfind status = %d body = %s", response.Code, response.Body.String())
	}
	results := parseCalendarMultistatusOrFatal(t, response.Body.Bytes())
	if len(results) != 1 {
		t.Fatalf("expected 1 response entry, got %d", len(results))
	}
	homeSetName := xml.Name{Space: "urn:ietf:params:xml:ns:caldav", Local: "calendar-home-set"}
	homeSetValue, ok := results[0].OK[homeSetName]
	if !ok {
		t.Fatalf("calendar-home-set missing from 200 OK propstat: %#v", results[0])
	}
	if !strings.Contains(homeSetValue, calendarHomeSetPath) {
		t.Fatalf("calendar-home-set href %q missing %s", homeSetValue, calendarHomeSetPath)
	}
	addressSetName := xml.Name{Space: "urn:ietf:params:xml:ns:caldav", Local: "calendar-user-address-set"}
	addressSetValue, ok := results[0].OK[addressSetName]
	if !ok {
		t.Fatalf("calendar-user-address-set missing from 200 OK propstat: %#v", results[0])
	}
	if !strings.Contains(addressSetValue, calendarPrincipalPath) {
		t.Fatalf("calendar-user-address-set href %q missing %s", addressSetValue, calendarPrincipalPath)
	}
	principalName := xml.Name{Space: "DAV:", Local: "current-user-principal"}
	if _, ok := results[0].OK[principalName]; !ok {
		t.Fatalf("current-user-principal missing from 200 OK propstat: %#v", results[0])
	}
}

func TestCalendarPropFindReportsUnknownPropertyAs404(t *testing.T) {
	service := newCalendarTestService(t)
	body := `<?xml version="1.0" encoding="utf-8"?>
<D:propfind xmlns:D="DAV:" xmlns:X="http://example.org/custom/">
  <D:prop><D:displayname/><X:made-up-property/></D:prop>
</D:propfind>`
	request := httptest.NewRequest("PROPFIND", calendarCollectionPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/xml")
	request.Header.Set("Depth", "0")
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusMultiStatus {
		t.Fatalf("propfind status = %d body = %s", response.Code, response.Body.String())
	}
	results := parseCalendarMultistatusOrFatal(t, response.Body.Bytes())
	if len(results) != 1 {
		t.Fatalf("expected 1 response entry, got %d", len(results))
	}
	knownName := xml.Name{Space: "DAV:", Local: "displayname"}
	if _, ok := results[0].OK[knownName]; !ok {
		t.Fatalf("displayname missing from 200 OK propstat: %#v", results[0])
	}
	unknownName := xml.Name{Space: "http://example.org/custom/", Local: "made-up-property"}
	if !containsName(results[0].NotFound, unknownName) {
		t.Fatalf("unknown property missing from 404 propstat: %#v", results[0])
	}
	if _, ok := results[0].OK[unknownName]; ok {
		t.Fatalf("unknown property must not appear in 200 propstat: %#v", results[0])
	}
}
