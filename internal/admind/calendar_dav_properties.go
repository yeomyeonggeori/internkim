package admind

// PROPPATCH/PROPFIND 처리, ctag 계산, calendar collection custom property 응답.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/xml"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const (
	calendarDAVNamespace          = "DAV:"
	calendarCalDAVXMLNamespace    = "urn:ietf:params:xml:ns:caldav"
	calendarAppleICalXMLNamespace = "http://apple.com/ns/ical/"
)

type calendarRawProperty struct {
	XMLName  xml.Name
	InnerXML string `xml:",innerxml"`
}

type calendarPropPatchProp struct {
	Properties []calendarRawProperty `xml:",any"`
}

type calendarPropPatchSet struct {
	XMLName xml.Name              `xml:"DAV: set"`
	Prop    calendarPropPatchProp `xml:"DAV: prop"`
}

type calendarPropPatchRemove struct {
	XMLName xml.Name              `xml:"DAV: remove"`
	Prop    calendarPropPatchProp `xml:"DAV: prop"`
}

type calendarPropertyUpdate struct {
	XMLName xml.Name                  `xml:"DAV: propertyupdate"`
	Set     []calendarPropPatchSet    `xml:"DAV: set"`
	Remove  []calendarPropPatchRemove `xml:"DAV: remove"`
}

type calendarPropFindRequest struct {
	AllProp bool
	Props   []xml.Name
}

// calendarStorablePropertyWhitelist는 PROPPATCH 가 받아들이는 property 집합이다.
// 모두 텍스트 값만 사용하는 property — nested element 는 silent corruption 위험이 있으므로 일부러 제외한다.
var calendarStorablePropertyWhitelist = map[xml.Name]bool{
	{Space: calendarDAVNamespace, Local: "displayname"}:             true,
	{Space: calendarAppleICalXMLNamespace, Local: "calendar-color"}: true,
	{Space: calendarAppleICalXMLNamespace, Local: "calendar-order"}: true,
}

func calendarPropertyIsStorable(name xml.Name) bool {
	return calendarStorablePropertyWhitelist[name]
}

func (service *Service) computeCalendarCTag(ctx context.Context) (string, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return "", errorValue
	}
	defer database.Close()
	var maxUpdated sql.NullString
	var eventCount int64
	row := database.QueryRowContext(ctx, `
SELECT COALESCE(MAX(updated_at), '') AS max_updated, COUNT(*) AS event_count
FROM calendar_events
WHERE deleted_at = ''`)
	if errorValue := row.Scan(&maxUpdated, &eventCount); errorValue != nil {
		return "", errorValue
	}
	fingerprint := maxUpdated.String + ":" + strconv.FormatInt(eventCount, 10)
	digest := sha256.Sum256([]byte(fingerprint))
	return "v1-" + hex.EncodeToString(digest[:8]), nil
}

func decodeCalendarPropertyTextValue(innerXML string) (string, bool) {
	decoder := xml.NewDecoder(strings.NewReader(innerXML))
	var builder strings.Builder
	for {
		token, errorValue := decoder.Token()
		if errorValue == io.EOF {
			break
		}
		if errorValue != nil {
			return "", false
		}
		characterData, isText := token.(xml.CharData)
		if !isText {
			return "", false
		}
		builder.Write(characterData)
	}
	return strings.TrimSpace(builder.String()), true
}

func (service *Service) handleCalendarPropPatch(responseWriter http.ResponseWriter, request *http.Request) {
	body, errorValue := io.ReadAll(request.Body)
	if errorValue != nil {
		http.Error(responseWriter, "read body failed", http.StatusBadRequest)
		return
	}
	var update calendarPropertyUpdate
	if errorValue := xml.Unmarshal(body, &update); errorValue != nil {
		http.Error(responseWriter, "invalid propertyupdate xml", http.StatusBadRequest)
		return
	}
	type propResult struct {
		name   xml.Name
		status int
	}
	results := []propResult{}
	for _, set := range update.Set {
		for _, property := range set.Prop.Properties {
			if !calendarPropertyIsStorable(property.XMLName) {
				results = append(results, propResult{property.XMLName, http.StatusForbidden})
				continue
			}
			value, isPureText := decodeCalendarPropertyTextValue(property.InnerXML)
			if !isPureText {
				results = append(results, propResult{property.XMLName, http.StatusForbidden})
				continue
			}
			if errorValue := service.writeCalendarProperty(request.Context(), request.URL.Path, property.XMLName.Space, property.XMLName.Local, value); errorValue != nil {
				results = append(results, propResult{property.XMLName, http.StatusInternalServerError})
				continue
			}
			results = append(results, propResult{property.XMLName, http.StatusOK})
		}
	}
	for _, remove := range update.Remove {
		for _, property := range remove.Prop.Properties {
			if !calendarPropertyIsStorable(property.XMLName) {
				results = append(results, propResult{property.XMLName, http.StatusForbidden})
				continue
			}
			_ = service.deleteCalendarProperty(request.Context(), request.URL.Path, property.XMLName.Space, property.XMLName.Local)
			results = append(results, propResult{property.XMLName, http.StatusOK})
		}
	}
	var buffer bytes.Buffer
	buffer.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
	buffer.WriteString(`<D:multistatus xmlns:D="DAV:">`)
	buffer.WriteString(`<D:response><D:href>`)
	xml.EscapeText(&buffer, []byte(request.URL.Path))
	buffer.WriteString(`</D:href>`)
	for _, result := range results {
		buffer.WriteString(`<D:propstat><D:prop>`)
		writeCalendarEmptyXMLElement(&buffer, result.name)
		buffer.WriteString(`</D:prop><D:status>HTTP/1.1 `)
		buffer.WriteString(strconv.Itoa(result.status))
		buffer.WriteString(` `)
		buffer.WriteString(http.StatusText(result.status))
		buffer.WriteString(`</D:status></D:propstat>`)
	}
	buffer.WriteString(`</D:response></D:multistatus>`)
	responseWriter.Header().Set("Content-Type", "application/xml; charset=utf-8")
	responseWriter.WriteHeader(http.StatusMultiStatus)
	_, _ = responseWriter.Write(buffer.Bytes())
}

func writeCalendarEmptyXMLElement(buffer *bytes.Buffer, name xml.Name) {
	if name.Space == "" {
		buffer.WriteString(`<`)
		buffer.WriteString(name.Local)
		buffer.WriteString(`/>`)
		return
	}
	buffer.WriteString(`<X:`)
	buffer.WriteString(name.Local)
	buffer.WriteString(` xmlns:X="`)
	xml.EscapeText(buffer, []byte(name.Space))
	buffer.WriteString(`"/>`)
}

func (service *Service) handleCalendarPropFind(responseWriter http.ResponseWriter, request *http.Request) {
	if !shouldHandleCalendarCollectionPropFind(request) {
		service.invokeCalendarDAVHandler(responseWriter, request)
		return
	}
	body, errorValue := io.ReadAll(request.Body)
	if errorValue != nil {
		http.Error(responseWriter, "read body failed", http.StatusBadRequest)
		return
	}
	parsed, errorValue := parseCalendarPropFindRequest(body)
	if errorValue != nil {
		http.Error(responseWriter, "invalid propfind xml", http.StatusBadRequest)
		return
	}
	response, errorValue := service.buildCalendarCollectionPropFindResponse(request.Context(), parsed)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/xml; charset=utf-8")
	responseWriter.WriteHeader(http.StatusMultiStatus)
	_, _ = responseWriter.Write(response)
}

func shouldHandleCalendarCollectionPropFind(request *http.Request) bool {
	if request.Method != "PROPFIND" {
		return false
	}
	depth := strings.TrimSpace(request.Header.Get("Depth"))
	if depth != "" && depth != "0" {
		return false
	}
	normalized := request.URL.Path
	if !strings.HasSuffix(normalized, "/") {
		normalized += "/"
	}
	return normalized == calendarCollectionPath
}

func parseCalendarPropFindRequest(body []byte) (calendarPropFindRequest, error) {
	request := calendarPropFindRequest{}
	if len(bytes.TrimSpace(body)) == 0 {
		request.AllProp = true
		return request, nil
	}
	type propXML struct {
		Properties []calendarRawProperty `xml:",any"`
	}
	type propfindXML struct {
		XMLName  xml.Name  `xml:"DAV: propfind"`
		AllProp  *struct{} `xml:"DAV: allprop"`
		PropName *struct{} `xml:"DAV: propname"`
		Prop     *propXML  `xml:"DAV: prop"`
	}
	var document propfindXML
	if errorValue := xml.Unmarshal(body, &document); errorValue != nil {
		return request, errorValue
	}
	if document.AllProp != nil || document.PropName != nil {
		request.AllProp = true
		return request, nil
	}
	if document.Prop != nil {
		for _, property := range document.Prop.Properties {
			request.Props = append(request.Props, property.XMLName)
		}
	}
	return request, nil
}

func (service *Service) buildCalendarCollectionPropFindResponse(ctx context.Context, request calendarPropFindRequest) ([]byte, error) {
	storedProperties, errorValue := service.readCalendarProperties(ctx, calendarCollectionPath)
	if errorValue != nil {
		return nil, errorValue
	}
	storedMap := make(map[xml.Name]string, len(storedProperties))
	for _, property := range storedProperties {
		storedMap[property.XMLName] = property.Value
	}
	ctag, _ := service.computeCalendarCTag(ctx)

	var requestedNames []xml.Name
	if request.AllProp {
		requestedNames = defaultCalendarCollectionPropertyNames()
	} else {
		requestedNames = request.Props
	}

	var foundBuffer bytes.Buffer
	var notFoundBuffer bytes.Buffer
	foundCount := 0
	notFoundCount := 0
	for _, name := range requestedNames {
		if writeCalendarCollectionProperty(&foundBuffer, name, storedMap, ctag) {
			foundCount++
		} else {
			writeCalendarEmptyXMLElement(&notFoundBuffer, name)
			notFoundCount++
		}
	}

	var result bytes.Buffer
	result.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
	result.WriteString(`<D:multistatus xmlns:D="DAV:">`)
	result.WriteString(`<D:response><D:href>`)
	xml.EscapeText(&result, []byte(calendarCollectionPath))
	result.WriteString(`</D:href>`)
	if foundCount > 0 {
		result.WriteString(`<D:propstat><D:prop>`)
		result.Write(foundBuffer.Bytes())
		result.WriteString(`</D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat>`)
	}
	if notFoundCount > 0 {
		result.WriteString(`<D:propstat><D:prop>`)
		result.Write(notFoundBuffer.Bytes())
		result.WriteString(`</D:prop><D:status>HTTP/1.1 404 Not Found</D:status></D:propstat>`)
	}
	result.WriteString(`</D:response></D:multistatus>`)
	return result.Bytes(), nil
}

func defaultCalendarCollectionPropertyNames() []xml.Name {
	return []xml.Name{
		{Space: calendarDAVNamespace, Local: "resourcetype"},
		{Space: calendarDAVNamespace, Local: "displayname"},
		{Space: calendarDAVNamespace, Local: "owner"},
		{Space: calendarDAVNamespace, Local: "current-user-principal"},
		{Space: calendarDAVNamespace, Local: "supported-report-set"},
		{Space: calendarCalDAVXMLNamespace, Local: "supported-calendar-component-set"},
		{Space: calendarCalDAVXMLNamespace, Local: "calendar-description"},
		{Space: calendarCalDAVXMLNamespace, Local: "max-resource-size"},
		{Space: calendarAppleICalXMLNamespace, Local: "calendar-color"},
		{Space: calendarServerXMLNamespace, Local: calendarGetCTagLocalName},
	}
}

func writeCalendarCollectionProperty(buffer *bytes.Buffer, name xml.Name, storedMap map[xml.Name]string, ctag string) bool {
	switch {
	case name.Space == calendarDAVNamespace && name.Local == "resourcetype":
		buffer.WriteString(`<D:resourcetype><D:collection/><C:calendar xmlns:C="urn:ietf:params:xml:ns:caldav"/></D:resourcetype>`)
		return true
	case name.Space == calendarDAVNamespace && name.Local == "displayname":
		value := calendarName
		if stored, ok := storedMap[name]; ok && strings.TrimSpace(stored) != "" {
			value = stored
		}
		buffer.WriteString(`<D:displayname>`)
		xml.EscapeText(buffer, []byte(value))
		buffer.WriteString(`</D:displayname>`)
		return true
	case name.Space == calendarDAVNamespace && name.Local == "owner":
		buffer.WriteString(`<D:owner><D:href>`)
		xml.EscapeText(buffer, []byte(calendarPrincipalPath))
		buffer.WriteString(`</D:href></D:owner>`)
		return true
	case name.Space == calendarDAVNamespace && name.Local == "current-user-principal":
		buffer.WriteString(`<D:current-user-principal><D:href>`)
		xml.EscapeText(buffer, []byte(calendarPrincipalPath))
		buffer.WriteString(`</D:href></D:current-user-principal>`)
		return true
	case name.Space == calendarDAVNamespace && name.Local == "principal-URL":
		buffer.WriteString(`<D:principal-URL><D:href>`)
		xml.EscapeText(buffer, []byte(calendarPrincipalPath))
		buffer.WriteString(`</D:href></D:principal-URL>`)
		return true
	case name.Space == calendarDAVNamespace && name.Local == "supported-report-set":
		buffer.WriteString(`<D:supported-report-set>`)
		buffer.WriteString(`<D:supported-report><D:report><C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav"/></D:report></D:supported-report>`)
		buffer.WriteString(`<D:supported-report><D:report><C:calendar-multiget xmlns:C="urn:ietf:params:xml:ns:caldav"/></D:report></D:supported-report>`)
		buffer.WriteString(`</D:supported-report-set>`)
		return true
	case name.Space == calendarCalDAVXMLNamespace && name.Local == "supported-calendar-component-set":
		buffer.WriteString(`<C:supported-calendar-component-set xmlns:C="urn:ietf:params:xml:ns:caldav"><C:comp name="VEVENT"/></C:supported-calendar-component-set>`)
		return true
	case name.Space == calendarCalDAVXMLNamespace && name.Local == "calendar-description":
		buffer.WriteString(`<C:calendar-description xmlns:C="urn:ietf:params:xml:ns:caldav">Shared Work calendar</C:calendar-description>`)
		return true
	case name.Space == calendarCalDAVXMLNamespace && name.Local == "max-resource-size":
		buffer.WriteString(`<C:max-resource-size xmlns:C="urn:ietf:params:xml:ns:caldav">1048576</C:max-resource-size>`)
		return true
	case name.Space == calendarAppleICalXMLNamespace && name.Local == "calendar-color":
		value := "#2563eb"
		if stored, ok := storedMap[name]; ok && strings.TrimSpace(stored) != "" {
			value = stored
		}
		buffer.WriteString(`<A:calendar-color xmlns:A="http://apple.com/ns/ical/">`)
		xml.EscapeText(buffer, []byte(value))
		buffer.WriteString(`</A:calendar-color>`)
		return true
	case name.Space == calendarServerXMLNamespace && name.Local == calendarGetCTagLocalName:
		if ctag == "" {
			return false
		}
		buffer.WriteString(`<CS:getctag xmlns:CS="`)
		xml.EscapeText(buffer, []byte(calendarServerXMLNamespace))
		buffer.WriteString(`">`)
		xml.EscapeText(buffer, []byte(ctag))
		buffer.WriteString(`</CS:getctag>`)
		return true
	}
	return false
}
