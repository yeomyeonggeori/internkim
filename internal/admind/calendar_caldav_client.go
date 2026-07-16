package admind

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"
)

const (
	caldavDefaultTimeout       = 30 * time.Second
	caldavICalendarContentType = "text/calendar; charset=utf-8"
	caldavIfMatchHeader        = "If-Match"
	caldavIfNoneMatchHeader    = "If-None-Match"
	caldavWildcardETag         = "*"
)

var (
	errCalDAVPreconditionFailed = errors.New("caldav precondition failed")
	errCalDAVObjectNotFound     = errors.New("caldav object not found")
)

func isCalDAVPreconditionFailed(errorValue error) bool {
	return errors.Is(errorValue, errCalDAVPreconditionFailed)
}

func isCalDAVObjectNotFound(errorValue error) bool {
	return errors.Is(errorValue, errCalDAVObjectNotFound)
}

type calDAVCalendarInfo struct {
	Path                string
	Name                string
	Description         string
	CTag                string
	Color               string
	CanRead             bool
	CanWrite            bool
	SupportedComponents []string
	MaxResourceSize     int64
}

type calDAVCalendarObject struct {
	Path            string
	ETag            string
	Data            []byte
	ConversionError error
}

type outboundCalDAVClient struct {
	endpoint   string
	httpClient webdav.HTTPClient
	webdav     *webdav.Client
	caldav     *caldav.Client
}

func newOutboundCalDAVClient(endpoint string, httpClient webdav.HTTPClient) (*outboundCalDAVClient, error) {
	if strings.TrimSpace(endpoint) == "" {
		return nil, errors.New("caldav endpoint is required")
	}
	webdavClient, errorValue := webdav.NewClient(httpClient, endpoint)
	if errorValue != nil {
		return nil, errorValue
	}
	caldavClient, errorValue := caldav.NewClient(httpClient, endpoint)
	if errorValue != nil {
		return nil, errorValue
	}
	return &outboundCalDAVClient{
		endpoint:   endpoint,
		httpClient: httpClient,
		webdav:     webdavClient,
		caldav:     caldavClient,
	}, nil
}

func (client *outboundCalDAVClient) discoverPrincipalURL(ctx context.Context) (string, error) {
	return client.webdav.FindCurrentUserPrincipal(ctx)
}

func (client *outboundCalDAVClient) discoverHomeSetURL(ctx context.Context, principalURL string) (string, error) {
	return client.caldav.FindCalendarHomeSet(ctx, calDAVPathOnly(principalURL))
}

func (client *outboundCalDAVClient) queryAllCalendarEvents(ctx context.Context, calendarPath string) ([]calDAVCalendarObject, error) {
	query := &caldav.CalendarQuery{
		CompRequest: caldav.CalendarCompRequest{
			Name: "VCALENDAR",
			Comps: []caldav.CalendarCompRequest{{
				Name:  "VEVENT",
				Props: []string{"UID", "SUMMARY", "DTSTART", "DTEND", "DTSTAMP", "LAST-MODIFIED", "STATUS", "RRULE", "RECURRENCE-ID", "DESCRIPTION", "LOCATION", "ORGANIZER", "ATTENDEE"},
			}},
		},
		CompFilter: caldav.CompFilter{
			Name: "VCALENDAR",
			Comps: []caldav.CompFilter{{
				Name: "VEVENT",
			}},
		},
	}
	objects, errorValue := client.caldav.QueryCalendar(ctx, calDAVPathOnly(calendarPath), query)
	if errorValue != nil {
		return nil, errorValue
	}
	return convertCalDAVObjects(objects), nil
}

func (client *outboundCalDAVClient) multiGetCalendarObjects(ctx context.Context, calendarPath string, paths []string) ([]calDAVCalendarObject, error) {
	multiGet := &caldav.CalendarMultiGet{
		Paths: append([]string(nil), paths...),
		CompRequest: caldav.CalendarCompRequest{
			Name:     "VCALENDAR",
			AllProps: true,
			AllComps: true,
		},
	}
	objects, errorValue := client.caldav.MultiGetCalendar(ctx, calDAVPathOnly(calendarPath), multiGet)
	if errorValue != nil {
		return nil, errorValue
	}
	return convertCalDAVObjects(objects), nil
}

func (client *outboundCalDAVClient) fetchCalendarCTag(ctx context.Context, calendarPath string) (string, error) {
	requestURL, errorValue := client.absoluteURL(calendarPath)
	if errorValue != nil {
		return "", errorValue
	}
	requestCtx, cancel := context.WithTimeout(ctx, caldavDefaultTimeout)
	defer cancel()
	body := strings.NewReader(`<?xml version="1.0" encoding="utf-8"?>
<d:propfind xmlns:d="DAV:" xmlns:cs="http://calendarserver.org/ns/">
  <d:prop>
    <cs:getctag/>
  </d:prop>
</d:propfind>`)
	request, errorValue := http.NewRequestWithContext(requestCtx, "PROPFIND", requestURL, body)
	if errorValue != nil {
		return "", errorValue
	}
	request.Header.Set("Depth", "0")
	request.Header.Set("Content-Type", "application/xml; charset=utf-8")
	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return "", errorValue
	}
	defer drainAndCloseResponse(response)
	if response.StatusCode != http.StatusMultiStatus && response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("caldav ctag propfind %s status %d: %s", requestURL, response.StatusCode, readCalDAVResponseExcerpt(response))
	}
	var decoded calDAVCTagMultiStatus
	if errorValue := xml.NewDecoder(response.Body).Decode(&decoded); errorValue != nil {
		return "", errorValue
	}
	for _, multistatusResponse := range decoded.Responses {
		for _, propstat := range multistatusResponse.Propstats {
			if propstat.Prop.CTag != "" {
				return propstat.Prop.CTag, nil
			}
		}
	}
	return "", nil
}

type calDAVCTagMultiStatus struct {
	XMLName   xml.Name             `xml:"DAV: multistatus"`
	Responses []calDAVCTagResponse `xml:"DAV: response"`
}

type calDAVCTagResponse struct {
	Href      string               `xml:"DAV: href"`
	Propstats []calDAVCTagPropstat `xml:"DAV: propstat"`
}

type calDAVCTagPropstat struct {
	Prop   calDAVCTagProp `xml:"DAV: prop"`
	Status string         `xml:"DAV: status"`
}

type calDAVCTagProp struct {
	CTag string `xml:"http://calendarserver.org/ns/ getctag"`
}

func (client *outboundCalDAVClient) getCalendarObject(ctx context.Context, objectPath string) (calDAVCalendarObject, error) {
	requestURL, errorValue := client.absoluteURL(objectPath)
	if errorValue != nil {
		return calDAVCalendarObject{}, errorValue
	}
	requestCtx, cancel := context.WithTimeout(ctx, caldavDefaultTimeout)
	defer cancel()
	request, errorValue := http.NewRequestWithContext(requestCtx, http.MethodGet, requestURL, nil)
	if errorValue != nil {
		return calDAVCalendarObject{}, errorValue
	}
	request.Header.Set("Accept", ical.MIMEType)
	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return calDAVCalendarObject{}, errorValue
	}
	defer drainAndCloseResponse(response)
	if response.StatusCode == http.StatusNotFound {
		return calDAVCalendarObject{}, errCalDAVObjectNotFound
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return calDAVCalendarObject{}, fmt.Errorf("caldav get %s status %d: %s", requestURL, response.StatusCode, readCalDAVResponseExcerpt(response))
	}
	encoded, errorValue := io.ReadAll(response.Body)
	if errorValue != nil {
		return calDAVCalendarObject{}, errorValue
	}
	return calDAVCalendarObject{
		Path: calDAVPathOnly(objectPath),
		ETag: strings.TrimSpace(response.Header.Get("ETag")),
		Data: encoded,
	}, nil
}

func (client *outboundCalDAVClient) putCalendarObject(ctx context.Context, objectPath string, ics []byte, ifMatch string, ifNoneMatch string) (string, error) {
	responseETag, errorValue := client.putCalendarObjectResponseETag(ctx, objectPath, ics, ifMatch, ifNoneMatch)
	if errorValue != nil {
		return "", errorValue
	}
	return recoverCanonicalETagAfterCalDAVPut(ctx, client, objectPath, responseETag)
}

func (client *outboundCalDAVClient) putCalendarObjectResponseETag(ctx context.Context, objectPath string, ics []byte, ifMatch string, ifNoneMatch string) (string, error) {
	requestURL, errorValue := client.absoluteURL(objectPath)
	if errorValue != nil {
		return "", errorValue
	}
	requestCtx, cancel := context.WithTimeout(ctx, caldavDefaultTimeout)
	defer cancel()
	request, errorValue := http.NewRequestWithContext(requestCtx, http.MethodPut, requestURL, bytes.NewReader(ics))
	if errorValue != nil {
		return "", errorValue
	}
	request.Header.Set("Content-Type", caldavICalendarContentType)
	if value := formatETagHeaderValue(ifMatch); value != "" {
		request.Header.Set(caldavIfMatchHeader, value)
	}
	if value := formatETagHeaderValue(ifNoneMatch); value != "" {
		request.Header.Set(caldavIfNoneMatchHeader, value)
	}
	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return "", errorValue
	}
	if response.StatusCode == http.StatusPreconditionFailed || response.StatusCode == http.StatusConflict {
		responseExcerpt := readCalDAVResponseExcerpt(response)
		drainAndCloseResponse(response)
		log.Printf("caldav put conflict status=%d url=%s if-match=%q if-none-match=%q body=%s", response.StatusCode, requestURL, ifMatch, ifNoneMatch, responseExcerpt)
		return "", errCalDAVPreconditionFailed
	}
	if response.StatusCode == http.StatusNotFound {
		drainAndCloseResponse(response)
		return "", errCalDAVObjectNotFound
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		responseExcerpt := readCalDAVResponseExcerpt(response)
		drainAndCloseResponse(response)
		return "", fmt.Errorf("caldav put %s status %d: %s", requestURL, response.StatusCode, responseExcerpt)
	}
	responseETag := response.Header.Get("ETag")
	drainAndCloseResponse(response)
	return responseETag, nil
}

func (client *outboundCalDAVClient) deleteCalendarObject(ctx context.Context, objectPath string, ifMatch string) error {
	requestURL, errorValue := client.absoluteURL(objectPath)
	if errorValue != nil {
		return errorValue
	}
	requestCtx, cancel := context.WithTimeout(ctx, caldavDefaultTimeout)
	defer cancel()
	request, errorValue := http.NewRequestWithContext(requestCtx, http.MethodDelete, requestURL, nil)
	if errorValue != nil {
		return errorValue
	}
	if value := formatETagHeaderValue(ifMatch); value != "" {
		request.Header.Set(caldavIfMatchHeader, value)
	}
	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer drainAndCloseResponse(response)
	if response.StatusCode == http.StatusPreconditionFailed {
		return errCalDAVPreconditionFailed
	}
	if response.StatusCode == http.StatusNotFound {
		return nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("caldav delete %s status %d: %s", requestURL, response.StatusCode, readCalDAVResponseExcerpt(response))
	}
	return nil
}

func calDAVPathOnly(absoluteOrPath string) string {
	if !strings.HasPrefix(absoluteOrPath, "http://") && !strings.HasPrefix(absoluteOrPath, "https://") {
		return absoluteOrPath
	}
	parsed, errorValue := url.Parse(absoluteOrPath)
	if errorValue != nil {
		return absoluteOrPath
	}
	if parsed.Path == "" {
		return "/"
	}
	return parsed.EscapedPath()
}

func formatETagHeaderValue(etag string) string {
	trimmed := strings.TrimSpace(etag)
	if trimmed == "" || trimmed == "*" {
		return trimmed
	}
	if strings.HasPrefix(trimmed, `W/"`) && strings.HasSuffix(trimmed, `"`) {
		return trimmed
	}
	if strings.HasPrefix(trimmed, `"`) && strings.HasSuffix(trimmed, `"`) {
		return trimmed
	}
	return `"` + trimmed + `"`
}

func (client *outboundCalDAVClient) absoluteURL(path string) (string, error) {
	base, errorValue := url.Parse(client.endpoint)
	if errorValue != nil {
		return "", errorValue
	}
	reference, errorValue := url.Parse(path)
	if errorValue != nil {
		return "", errorValue
	}
	resolved := base.ResolveReference(reference)
	if !strings.EqualFold(resolved.Scheme, base.Scheme) || !strings.EqualFold(resolved.Host, base.Host) {
		return "", fmt.Errorf("caldav URL must use endpoint origin %s://%s", base.Scheme, base.Host)
	}
	return resolved.String(), nil
}

func convertCalDAVObjects(objects []caldav.CalendarObject) []calDAVCalendarObject {
	result := make([]calDAVCalendarObject, 0, len(objects))
	for index := range objects {
		encoded, errorValue := encodeICalendarBytes(&objects[index])
		if errorValue != nil {
			result = append(result, calDAVCalendarObject{
				Path:            objects[index].Path,
				ETag:            objects[index].ETag,
				ConversionError: fmt.Errorf("encode CalDAV calendar object %s: %w", objects[index].Path, errorValue),
			})
			continue
		}
		result = append(result, calDAVCalendarObject{
			Path: objects[index].Path,
			ETag: objects[index].ETag,
			Data: encoded,
		})
	}
	return result
}

func encodeICalendarBytes(object *caldav.CalendarObject) ([]byte, error) {
	if object == nil || object.Data == nil {
		return nil, nil
	}
	var buffer bytes.Buffer
	if errorValue := ical.NewEncoder(&buffer).Encode(object.Data); errorValue != nil {
		return nil, errorValue
	}
	return buffer.Bytes(), nil
}

func readCalDAVResponseExcerpt(response *http.Response) string {
	if response == nil || response.Body == nil {
		return ""
	}
	const limit = 512
	body, errorValue := io.ReadAll(io.LimitReader(response.Body, limit))
	if errorValue != nil {
		return ""
	}
	excerpt := strings.TrimSpace(string(body))
	excerpt = strings.ReplaceAll(excerpt, "\n", " ")
	return excerpt
}

func drainAndCloseResponse(response *http.Response) {
	if response == nil || response.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
}
