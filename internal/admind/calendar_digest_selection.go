package admind

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

const calendarDigestLongest = 3

type calendarDigestEntry struct {
	At    string
	Title string
}

func calendarDigestFor(events []calendarEvent, email string, location *time.Location) []calendarDigestEntry {
	wanted := strings.ToLower(strings.TrimSpace(email))
	if wanted == "" {
		return nil
	}
	entries := make([]calendarDigestEntry, 0, len(events))
	for _, event := range events {
		if !calendarEventNames(event, wanted) {
			continue
		}
		entries = append(entries, calendarDigestEntry{At: calendarDigestTime(event, location), Title: event.Title})
	}
	sort.SliceStable(entries, func(first int, second int) bool { return entries[first].At < entries[second].At })
	return entries
}

func calendarEventNames(event calendarEvent, wanted string) bool {
	if strings.ToLower(strings.TrimSpace(event.CreatedByEmail)) == wanted {
		return true
	}
	for _, participant := range event.Participants {
		if strings.ToLower(strings.TrimSpace(participant.Email)) == wanted {
			return true
		}
	}
	return false
}

func calendarDigestTime(event calendarEvent, location *time.Location) string {
	if event.IsAllDay {
		return ""
	}
	startsAt, errorValue := time.Parse(time.RFC3339, event.StartISO)
	if errorValue != nil {
		return ""
	}
	return startsAt.In(location).Format("15:04")
}

func calendarDigestBody(entries []calendarDigestEntry) string {
	lines := make([]string, 0, calendarDigestLongest)
	for _, entry := range entries {
		if len(lines) == calendarDigestLongest {
			break
		}
		if entry.At == "" {
			lines = append(lines, entry.Title)
			continue
		}
		lines = append(lines, entry.At+" "+entry.Title)
	}
	body := strings.Join(lines, "\n")
	if remaining := len(entries) - len(lines); remaining > 0 {
		body = body + "\n외 " + strconv.Itoa(remaining) + "건"
	}
	return body
}
