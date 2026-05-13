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
	"net/http/httptest"
	"strconv"
	"strings"
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
			value := strings.TrimSpace(property.InnerXML)
			if errorValue := service.writeCalendarProperty(request.Context(), request.URL.Path, property.XMLName.Space, property.XMLName.Local, value); errorValue != nil {
				results = append(results, propResult{property.XMLName, http.StatusInternalServerError})
				continue
			}
			results = append(results, propResult{property.XMLName, http.StatusOK})
		}
	}
	for _, remove := range update.Remove {
		for _, property := range remove.Prop.Properties {
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
	recorder := httptest.NewRecorder()
	service.invokeCalendarDAVHandler(recorder, request)
	if recorder.Code != http.StatusMultiStatus {
		copyCalendarRecorderResponse(responseWriter, recorder)
		return
	}
	properties, errorValue := service.readCalendarProperties(request.Context(), calendarCollectionPath)
	if errorValue != nil {
		copyCalendarRecorderResponse(responseWriter, recorder)
		return
	}
	if ctag, errorValue := service.computeCalendarCTag(request.Context()); errorValue == nil {
		properties = append(properties, calendarStoredProperty{
			XMLName: xml.Name{Space: calendarServerXMLNamespace, Local: calendarGetCTagLocalName},
			Value:   ctag,
		})
	}
	if len(properties) == 0 {
		copyCalendarRecorderResponse(responseWriter, recorder)
		return
	}
	augmented, errorValue := injectCalendarPropertiesIntoMultistatus(recorder.Body.Bytes(), calendarCollectionPath, properties)
	if errorValue != nil {
		copyCalendarRecorderResponse(responseWriter, recorder)
		return
	}
	for key, values := range recorder.Header() {
		if strings.EqualFold(key, "Content-Length") {
			continue
		}
		for _, value := range values {
			responseWriter.Header().Add(key, value)
		}
	}
	responseWriter.Header().Set("Content-Length", strconv.Itoa(len(augmented)))
	responseWriter.WriteHeader(http.StatusMultiStatus)
	_, _ = responseWriter.Write(augmented)
}

func copyCalendarRecorderResponse(responseWriter http.ResponseWriter, recorder *httptest.ResponseRecorder) {
	for key, values := range recorder.Header() {
		for _, value := range values {
			responseWriter.Header().Add(key, value)
		}
	}
	responseWriter.WriteHeader(recorder.Code)
	_, _ = responseWriter.Write(recorder.Body.Bytes())
}

func injectCalendarPropertiesIntoMultistatus(body []byte, targetPath string, properties []calendarStoredProperty) ([]byte, error) {
	if len(properties) == 0 {
		return body, nil
	}
	hrefMarker := []byte("<href>" + targetPath + "</href>")
	hrefIndex := bytes.Index(body, hrefMarker)
	if hrefIndex < 0 {
		alternate := []byte("<href>" + strings.TrimSuffix(targetPath, "/") + "</href>")
		hrefIndex = bytes.Index(body, alternate)
		if hrefIndex < 0 {
			return body, nil
		}
	}
	propCloseIndex := bytes.Index(body[hrefIndex:], []byte("</prop>"))
	if propCloseIndex < 0 {
		return body, nil
	}
	insertionPoint := hrefIndex + propCloseIndex
	var injection bytes.Buffer
	for _, property := range properties {
		writeCalendarPropertyDefaultNamespace(&injection, property)
	}
	result := make([]byte, 0, len(body)+injection.Len())
	result = append(result, body[:insertionPoint]...)
	result = append(result, injection.Bytes()...)
	result = append(result, body[insertionPoint:]...)
	return result, nil
}

func writeCalendarPropertyDefaultNamespace(buffer *bytes.Buffer, property calendarStoredProperty) {
	if property.XMLName.Space == "" || property.XMLName.Space == "DAV:" {
		buffer.WriteString(`<`)
		buffer.WriteString(property.XMLName.Local)
		if property.XMLName.Space == "DAV:" {
			buffer.WriteString(` xmlns="DAV:"`)
		}
	} else {
		buffer.WriteString(`<`)
		buffer.WriteString(property.XMLName.Local)
		buffer.WriteString(` xmlns="`)
		xml.EscapeText(buffer, []byte(property.XMLName.Space))
		buffer.WriteString(`"`)
	}
	if property.Value == "" {
		buffer.WriteString(`/>`)
		return
	}
	buffer.WriteString(`>`)
	xml.EscapeText(buffer, []byte(property.Value))
	buffer.WriteString(`</`)
	buffer.WriteString(property.XMLName.Local)
	buffer.WriteString(`>`)
}
