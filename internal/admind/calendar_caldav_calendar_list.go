package admind

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func (client *outboundCalDAVClient) listCalendars(ctx context.Context, homeSetURL string) ([]calDAVCalendarInfo, error) {
	requestURL, errorValue := client.absoluteURL(homeSetURL)
	if errorValue != nil {
		return nil, errorValue
	}
	requestCtx, cancel := context.WithTimeout(ctx, caldavDefaultTimeout)
	defer cancel()
	body := strings.NewReader(`<?xml version="1.0" encoding="utf-8"?>
<d:propfind xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav" xmlns:i="http://apple.com/ns/ical/" xmlns:cs="http://calendarserver.org/ns/">
  <d:prop>
    <d:resourcetype/>
    <d:displayname/>
    <d:current-user-privilege-set/>
    <c:calendar-description/>
    <c:max-resource-size/>
    <c:supported-calendar-component-set/>
    <i:calendar-color/>
    <cs:getctag/>
  </d:prop>
</d:propfind>`)
	request, errorValue := http.NewRequestWithContext(requestCtx, "PROPFIND", requestURL, body)
	if errorValue != nil {
		return nil, errorValue
	}
	request.Header.Set("Depth", "1")
	request.Header.Set("Content-Type", "application/xml; charset=utf-8")
	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return nil, errorValue
	}
	defer drainAndCloseResponse(response)
	if response.StatusCode != http.StatusMultiStatus && response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("caldav calendar list propfind %s status %d: %s", requestURL, response.StatusCode, readCalDAVResponseExcerpt(response))
	}
	var decoded calDAVCalendarListMultiStatus
	if errorValue := xml.NewDecoder(response.Body).Decode(&decoded); errorValue != nil {
		return nil, errorValue
	}
	return calDAVCalendarInfosFromMultiStatus(decoded), nil
}

type calDAVCalendarListMultiStatus struct {
	XMLName   xml.Name                     `xml:"DAV: multistatus"`
	Responses []calDAVCalendarListResponse `xml:"DAV: response"`
}

type calDAVCalendarListResponse struct {
	Href      string                       `xml:"DAV: href"`
	Propstats []calDAVCalendarListPropstat `xml:"DAV: propstat"`
}

type calDAVCalendarListPropstat struct {
	Prop   calDAVCalendarListProp `xml:"DAV: prop"`
	Status string                 `xml:"DAV: status"`
}

type calDAVCalendarListProp struct {
	ResourceType        calDAVXMLNames              `xml:"DAV: resourcetype"`
	DisplayName         string                      `xml:"DAV: displayname"`
	Description         string                      `xml:"urn:ietf:params:xml:ns:caldav calendar-description"`
	MaxResourceSize     string                      `xml:"urn:ietf:params:xml:ns:caldav max-resource-size"`
	SupportedComponents calDAVSupportedComponentSet `xml:"urn:ietf:params:xml:ns:caldav supported-calendar-component-set"`
	Color               string                      `xml:"http://apple.com/ns/ical/ calendar-color"`
	CTag                string                      `xml:"http://calendarserver.org/ns/ getctag"`
	PrivilegeSet        calDAVPrivilegeSet          `xml:"DAV: current-user-privilege-set"`
}

type calDAVXMLNames struct {
	Names []xml.Name `xml:",any"`
}

type calDAVSupportedComponentSet struct {
	Components []calDAVSupportedComponent `xml:"urn:ietf:params:xml:ns:caldav comp"`
}

type calDAVSupportedComponent struct {
	Name string `xml:"name,attr"`
}

type calDAVPrivilegeSet struct {
	Privileges []calDAVPrivilege `xml:"DAV: privilege"`
}

type calDAVPrivilege struct {
	Names []xml.Name `xml:",any"`
}

func calDAVCalendarInfosFromMultiStatus(multistatus calDAVCalendarListMultiStatus) []calDAVCalendarInfo {
	calendars := []calDAVCalendarInfo{}
	for _, response := range multistatus.Responses {
		for _, propstat := range response.Propstats {
			if !calDAVStatusIsSuccess(propstat.Status) || !propstat.Prop.ResourceType.Has("DAV:", "calendar") && !propstat.Prop.ResourceType.Has("urn:ietf:params:xml:ns:caldav", "calendar") {
				continue
			}
			maxResourceSize, _ := strconv.ParseInt(strings.TrimSpace(propstat.Prop.MaxResourceSize), 10, 64)
			canRead, canWrite := propstat.Prop.PrivilegeSet.Access()
			calendars = append(calendars, calDAVCalendarInfo{
				Path:                strings.TrimSpace(response.Href),
				Name:                strings.TrimSpace(propstat.Prop.DisplayName),
				Description:         strings.TrimSpace(propstat.Prop.Description),
				CTag:                strings.TrimSpace(propstat.Prop.CTag),
				Color:               strings.TrimSpace(propstat.Prop.Color),
				CanRead:             canRead,
				CanWrite:            canWrite,
				SupportedComponents: propstat.Prop.SupportedComponents.Names(),
				MaxResourceSize:     maxResourceSize,
			})
		}
	}
	return calendars
}

func calDAVStatusIsSuccess(status string) bool {
	fields := strings.Fields(status)
	if len(fields) < 2 {
		return false
	}
	code, errorValue := strconv.Atoi(fields[1])
	return errorValue == nil && code >= 200 && code < 300
}

func (names calDAVXMLNames) Has(space string, local string) bool {
	for _, name := range names.Names {
		if name.Space == space && name.Local == local {
			return true
		}
	}
	return false
}

func (components calDAVSupportedComponentSet) Names() []string {
	names := make([]string, 0, len(components.Components))
	for _, component := range components.Components {
		name := strings.ToUpper(strings.TrimSpace(component.Name))
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func (privileges calDAVPrivilegeSet) Access() (bool, bool) {
	canRead := false
	canWrite := false
	for _, privilege := range privileges.Privileges {
		for _, name := range privilege.Names {
			if name.Space != "DAV:" {
				continue
			}
			switch name.Local {
			case "all":
				canRead = true
				canWrite = true
			case "read":
				canRead = true
			case "write", "write-content", "write-properties", "bind", "unbind":
				canWrite = true
			}
		}
	}
	if canWrite {
		canRead = true
	}
	return canRead, canWrite
}
