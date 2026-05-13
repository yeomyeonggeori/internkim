package admind

// go-webdav caldav.Backend 인터페이스 구현과 calendarEvent <-> caldav.CalendarObject 어댑터.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	pathpkg "path"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"
)

type calendarDAVBackend struct {
	service *Service
}

func checkCalendarPutPreconditions(options *caldav.PutCalendarObjectOptions, existing calendarEvent, exists bool) error {
	if options == nil {
		return nil
	}
	if options.IfNoneMatch.IsWildcard() && exists {
		return webdav.NewHTTPError(http.StatusPreconditionFailed, errors.New("calendar object already exists"))
	}
	if !options.IfMatch.IsSet() {
		return nil
	}
	if options.IfMatch.IsWildcard() {
		if !exists {
			return webdav.NewHTTPError(http.StatusPreconditionFailed, errors.New("calendar object does not exist"))
		}
		return nil
	}
	if !exists {
		return webdav.NewHTTPError(http.StatusPreconditionFailed, errors.New("calendar object does not exist"))
	}
	match, errorValue := options.IfMatch.MatchETag(calendarEventETag(existing))
	if errorValue != nil {
		return webdav.NewHTTPError(http.StatusBadRequest, errorValue)
	}
	if !match {
		return webdav.NewHTTPError(http.StatusPreconditionFailed, errors.New("etag mismatch"))
	}
	return nil
}

func calendarUIDFromObjectPath(path string) string {
	return strings.TrimSuffix(pathpkg.Base(path), ".ics")
}

func (backend calendarDAVBackend) CurrentUserPrincipal(ctx context.Context) (string, error) {
	return calendarPrincipalPath, nil
}

func (backend calendarDAVBackend) CalendarHomeSetPath(ctx context.Context) (string, error) {
	return calendarHomeSetPath, nil
}

func (backend calendarDAVBackend) CreateCalendar(ctx context.Context, calendar *caldav.Calendar) error {
	return nil
}

func (backend calendarDAVBackend) ListCalendars(ctx context.Context) ([]caldav.Calendar, error) {
	return []caldav.Calendar{calendarDAVCollection()}, nil
}

func (backend calendarDAVBackend) GetCalendar(ctx context.Context, path string) (*caldav.Calendar, error) {
	normalizedPath := strings.TrimSuffix(path, "/") + "/"
	if normalizedPath != calendarCollectionPath {
		return nil, fmt.Errorf("calendar not found")
	}
	calendar := calendarDAVCollection()
	return &calendar, nil
}

func calendarDAVCollection() caldav.Calendar {
	return caldav.Calendar{
		Path:                  calendarCollectionPath,
		Name:                  calendarName,
		Description:           "Shared Work calendar",
		MaxResourceSize:       1024 * 1024,
		SupportedComponentSet: []string{ical.CompEvent},
	}
}

func (backend calendarDAVBackend) GetCalendarObject(ctx context.Context, path string, request *caldav.CalendarCompRequest) (*caldav.CalendarObject, error) {
	uid := calendarUIDFromObjectPath(path)
	event, found, errorValue := backend.service.readCalendarEventByID(ctx, uid)
	if errorValue != nil {
		return nil, errorValue
	}
	if !found {
		return nil, fmt.Errorf("calendar object not found")
	}
	object, errorValue := calendarObjectFromEvent(event)
	if errorValue != nil {
		return nil, errorValue
	}
	return &object, nil
}

func (backend calendarDAVBackend) ListCalendarObjects(ctx context.Context, path string, request *caldav.CalendarCompRequest) ([]caldav.CalendarObject, error) {
	events, errorValue := backend.service.readCalendarEvents(ctx, time.Time{}, time.Time{})
	if errorValue != nil {
		return nil, errorValue
	}
	return calendarObjectsFromEvents(events)
}

func (backend calendarDAVBackend) QueryCalendarObjects(ctx context.Context, path string, query *caldav.CalendarQuery) ([]caldav.CalendarObject, error) {
	startTime, endTime := calendarQueryRange(query)
	events, errorValue := backend.service.readCalendarEvents(ctx, startTime, endTime)
	if errorValue != nil {
		return nil, errorValue
	}
	return calendarObjectsFromEvents(events)
}

func (backend calendarDAVBackend) PutCalendarObject(ctx context.Context, path string, calendar *ical.Calendar, options *caldav.PutCalendarObjectOptions) (*caldav.CalendarObject, error) {
	if _, _, errorValue := caldav.ValidateCalendarObject(calendar); errorValue != nil {
		return nil, errorValue
	}
	event, errorValue := calendarEventFromCalendarObject(path, calendar, "")
	if errorValue != nil {
		return nil, errorValue
	}
	existingEvent, existingFound, errorValue := backend.service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := checkCalendarPutPreconditions(options, existingEvent, existingFound); errorValue != nil {
		return nil, errorValue
	}
	if existingFound {
		event.CreatedByEmail = existingEvent.CreatedByEmail
		event.MattermostPostID = existingEvent.MattermostPostID
	}
	if errorValue := backend.service.writeCalendarEvent(ctx, event); errorValue != nil {
		return nil, errorValue
	}
	writtenEvent, found, errorValue := backend.service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil {
		return nil, errorValue
	}
	if !found {
		return nil, fmt.Errorf("calendar object was not persisted")
	}
	object, errorValue := calendarObjectFromEvent(writtenEvent)
	if errorValue != nil {
		return nil, errorValue
	}
	return &object, nil
}

func (backend calendarDAVBackend) DeleteCalendarObject(ctx context.Context, path string) error {
	return backend.service.softDeleteCalendarEvent(ctx, calendarUIDFromObjectPath(path))
}

func calendarObjectsFromEvents(events []calendarEvent) ([]caldav.CalendarObject, error) {
	objects := make([]caldav.CalendarObject, 0, len(events))
	for _, event := range events {
		object, errorValue := calendarObjectFromEvent(event)
		if errorValue != nil {
			return nil, errorValue
		}
		objects = append(objects, object)
	}
	return objects, nil
}

func calendarQueryRange(query *caldav.CalendarQuery) (time.Time, time.Time) {
	if query == nil {
		return time.Time{}, time.Time{}
	}
	return calendarCompFilterRange(query.CompFilter)
}

func calendarCompFilterRange(filter caldav.CompFilter) (time.Time, time.Time) {
	if !filter.Start.IsZero() || !filter.End.IsZero() {
		return filter.Start.UTC(), filter.End.UTC()
	}
	for _, childFilter := range filter.Comps {
		startTime, endTime := calendarCompFilterRange(childFilter)
		if !startTime.IsZero() || !endTime.IsZero() {
			return startTime, endTime
		}
	}
	return time.Time{}, time.Time{}
}

var _ caldav.Backend = calendarDAVBackend{}
var _ webdav.UserPrincipalBackend = calendarDAVBackend{}
