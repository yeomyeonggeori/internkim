package admind

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"log"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

const companyShareActivityWindowDays = 52 * 7

type companyShareActivityDay struct {
	Date            string `json:"date"`
	AttendanceCount int    `json:"attendanceCount"`
	WorkCount       int    `json:"workCount"`
	WorkMinutes     int    `json:"workMinutes"`
}

type companyShareWorkStatus struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

type companyShareRecentWork struct {
	MemberSeed string `json:"memberSeed"`
	Title      string `json:"title"`
	Business   string `json:"business,omitempty"`
	Type       string `json:"type,omitempty"`
	Size       string `json:"size,omitempty"`
	Status     string `json:"status"`
	StartDate  string `json:"startDate,omitempty"`
	EndDate    string `json:"endDate,omitempty"`
	Date       string `json:"date"`
}

type companyShareMember struct {
	Seed     string `json:"seed"`
	Surname  string `json:"surname"`
	JobTitle string `json:"jobTitle,omitempty"`
}

type companyShareTeamActivity struct {
	WindowDays      int                       `json:"windowDays"`
	Members         []companyShareMember      `json:"members"`
	Days            []companyShareActivityDay `json:"days"`
	WorkStatuses    []companyShareWorkStatus  `json:"workStatuses"`
	RecentWork      []companyShareRecentWork  `json:"recentWork"`
	AttendanceTotal int                       `json:"attendanceTotal"`
	WorkTotal       int                       `json:"workTotal"`
}

type companyShareWorkRow struct {
	MemberID  string
	Name      string
	Title     string
	Business  string
	Type      string
	Size      string
	Status    string
	StartDate string
	EndDate   string
	Date      string
}

type companyShareMemberSource struct {
	MemberID string
	Name     string
	JobTitle string
}

func (service *Service) buildCompanyShareTeamActivity(ctx context.Context, now time.Time) (companyShareTeamActivity, error) {
	location, _ := service.workspaceTimeLocation()
	localNow := now.In(location)
	startDate := localNow.AddDate(0, 0, -(companyShareActivityWindowDays - 1)).Format("2006-01-02")
	endDate := localNow.Format("2006-01-02")
	attendanceByDate, workMinutesByDate, attendanceMembers, errorValue := service.readCompanyShareAttendanceActivity(ctx, startDate, endDate, now, location)
	if errorValue != nil {
		return companyShareTeamActivity{}, errorValue
	}
	workByDate, workRows, statusCounts, workMembers, errorValue := service.readCompanyShareWorkActivity(ctx, startDate, endDate)
	if errorValue != nil {
		return companyShareTeamActivity{}, errorValue
	}
	memberSources := append([]companyShareMemberSource{}, attendanceMembers...)
	memberSources = append(memberSources, workMembers...)
	memberSources = service.enrichCompanyShareMemberSources(ctx, memberSources)
	members, seedByMemberID, errorValue := service.companyShareMembers(memberSources)
	if errorValue != nil {
		return companyShareTeamActivity{}, errorValue
	}
	days := buildCompanyShareActivityDays(localNow, attendanceByDate, workByDate, workMinutesByDate)
	return companyShareTeamActivity{
		WindowDays:      companyShareActivityWindowDays,
		Members:         members,
		Days:            days,
		WorkStatuses:    companyShareWorkStatuses(statusCounts),
		RecentWork:      companyShareRecentWorkRows(workRows, seedByMemberID),
		AttendanceTotal: sumCompanyShareActivity(attendanceByDate),
		WorkTotal:       sumCompanyShareActivity(workByDate),
	}, nil
}

func (service *Service) readCompanyShareAttendanceActivity(ctx context.Context, startDate string, endDate string, now time.Time, location *time.Location) (map[string]int, map[string]int, []companyShareMemberSource, error) {
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return nil, nil, nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT local_date, COUNT(DISTINCT email)
FROM attendance_events
WHERE local_date >= ? AND local_date <= ? AND kind = ? AND canceled_at = ''
GROUP BY local_date`, startDate, endDate, attendanceKindClockIn)
	if errorValue != nil {
		return nil, nil, nil, errorValue
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var date string
		var count int
		if errorValue := rows.Scan(&date, &count); errorValue != nil {
			return nil, nil, nil, errorValue
		}
		counts[date] = count
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, nil, nil, errorValue
	}
	rows.Close()
	memberRows, errorValue := database.QueryContext(ctx, `
SELECT DISTINCT email, display_name
FROM attendance_events
WHERE local_date >= ? AND local_date <= ? AND kind = ? AND canceled_at = ''`, startDate, endDate, attendanceKindClockIn)
	if errorValue != nil {
		return nil, nil, nil, errorValue
	}
	defer memberRows.Close()
	members := []companyShareMemberSource{}
	for memberRows.Next() {
		var email string
		var name string
		if errorValue := memberRows.Scan(&email, &name); errorValue != nil {
			return nil, nil, nil, errorValue
		}
		members = append(members, companyShareMemberSource{MemberID: companySharePersonID(email), Name: name})
	}
	if errorValue := memberRows.Err(); errorValue != nil {
		return nil, nil, nil, errorValue
	}
	memberRows.Close()
	events, errorValue := service.readCompanyShareAttendanceEvents(ctx, database)
	if errorValue != nil {
		return nil, nil, nil, errorValue
	}
	workMinutesByDate := buildCompanyShareWorkMinutesByDate(events, startDate, endDate, now, location)
	return counts, workMinutesByDate, members, nil
}

func (service *Service) readCompanyShareAttendanceEvents(ctx context.Context, database *sql.DB) ([]attendanceEvent, error) {
	rows, errorValue := database.QueryContext(ctx, "SELECT "+attendanceEventSelectColumns+`
FROM attendance_events
WHERE canceled_at = ''
ORDER BY occurred_at ASC`)
	if errorValue != nil {
		return nil, errorValue
	}
	events := []attendanceEvent{}
	for rows.Next() {
		event, scanError := scanAttendanceEvent(rows)
		if scanError != nil {
			rows.Close()
			return nil, scanError
		}
		events = append(events, event)
	}
	errorValue = rows.Err()
	rows.Close()
	if errorValue != nil {
		return nil, errorValue
	}
	return service.applyAttendanceEventOverrides(ctx, database, events)
}

func buildCompanyShareWorkMinutesByDate(events []attendanceEvent, startDate string, endDate string, now time.Time, location *time.Location) map[string]int {
	windowStart, startError := time.ParseInLocation("2006-01-02", startDate, location)
	windowEndDate, endError := time.ParseInLocation("2006-01-02", endDate, location)
	if startError != nil || endError != nil {
		return map[string]int{}
	}
	windowEnd := windowEndDate.AddDate(0, 0, 1)
	orderedEvents := append([]attendanceEvent{}, events...)
	sort.SliceStable(orderedEvents, func(leftIndex int, rightIndex int) bool {
		return orderedEvents[leftIndex].OccurredAt < orderedEvents[rightIndex].OccurredAt
	})
	openByMemberID := map[string]time.Time{}
	workMinutesByDate := map[string]int{}
	for _, event := range orderedEvents {
		occurredAt, errorValue := parseAttendanceEventTime(event.OccurredAt)
		if errorValue != nil || occurredAt.After(now) {
			continue
		}
		memberID := companySharePersonID(firstNonEmpty(event.Email, event.MattermostUserID))
		if event.Kind == attendanceKindClockIn {
			if startedAt, found := openByMemberID[memberID]; found {
				addCompanyShareWorkMinutes(workMinutesByDate, startedAt, occurredAt, windowStart, windowEnd, location)
			}
			openByMemberID[memberID] = occurredAt
			continue
		}
		startedAt, found := openByMemberID[memberID]
		if event.Kind != attendanceKindClockOut || !found {
			continue
		}
		addCompanyShareWorkMinutes(workMinutesByDate, startedAt, occurredAt, windowStart, windowEnd, location)
		delete(openByMemberID, memberID)
	}
	for _, startedAt := range openByMemberID {
		addCompanyShareWorkMinutes(workMinutesByDate, startedAt, now, windowStart, windowEnd, location)
	}
	return workMinutesByDate
}

func addCompanyShareWorkMinutes(workMinutesByDate map[string]int, startedAt time.Time, endedAt time.Time, windowStart time.Time, windowEnd time.Time, location *time.Location) {
	segmentStart := maxTime(startedAt, windowStart)
	segmentEnd := minTime(endedAt, windowEnd)
	for segmentStart.Before(segmentEnd) {
		localStart := segmentStart.In(location)
		nextMidnight := time.Date(localStart.Year(), localStart.Month(), localStart.Day()+1, 0, 0, 0, 0, location)
		dayEnd := minTime(segmentEnd, nextMidnight)
		date := localStart.Format("2006-01-02")
		workMinutesByDate[date] += int(math.Round(dayEnd.Sub(segmentStart).Minutes()))
		segmentStart = dayEnd
	}
}

func minTime(left time.Time, right time.Time) time.Time {
	if left.Before(right) {
		return left
	}
	return right
}

func maxTime(left time.Time, right time.Time) time.Time {
	if left.After(right) {
		return left
	}
	return right
}

func (service *Service) readCompanyShareWorkActivity(ctx context.Context, startDate string, endDate string) (map[string]int, []companyShareWorkRow, map[string]int, []companyShareMemberSource, error) {
	activity, errorValue := service.readCompanyShareWorkRows(ctx, startDate, endDate)
	if errorValue != nil {
		return nil, nil, nil, nil, errorValue
	}
	counts := map[string]int{}
	statusCounts := map[string]int{}
	members := []companyShareMemberSource{}
	workRows := []companyShareWorkRow{}
	for _, row := range activity {
		row.MemberID = companySharePersonID(row.MemberID)
		row.Title = strings.TrimSpace(row.Title)
		row.Status = publicCompanyShareWorkStatus(row.Status)
		if row.Status == "" {
			continue
		}
		counts[row.Date]++
		statusCounts[row.Status]++
		members = append(members, companyShareMemberSource{MemberID: row.MemberID, Name: row.Name})
		workRows = append(workRows, row)
	}
	return counts, workRows, statusCounts, members, nil
}

func (service *Service) readCompanyShareWorkRows(ctx context.Context, startDate string, endDate string) ([]companyShareWorkRow, error) {
	if service.centralPlane() == nil {
		return service.readTaskShareActivity(ctx, startDate, endDate)
	}
	return service.readCompanyShareBoardActivity(ctx, startDate, endDate)
}

func (service *Service) readCompanyShareBoardActivity(ctx context.Context, startDate string, endDate string) ([]companyShareWorkRow, error) {
	adminEmail := service.claimedAdminEmail()
	if adminEmail == "" {
		log.Printf("the share activity has no claimed admin to read the company board as, so it publishes without work rows")
		return []companyShareWorkRow{}, nil
	}
	tasks, errorValue := service.centralPlane().BoardTasks(ctx, "email", adminEmail)
	if errorValue != nil {
		return nil, errorValue
	}
	sort.SliceStable(tasks, func(first int, second int) bool {
		return tasks[first].UpdatedAt > tasks[second].UpdatedAt
	})
	rows := []companyShareWorkRow{}
	for _, task := range tasks {
		row := companyShareWorkRowOfBoardTask(task)
		if row.Date < startDate || row.Date > endDate {
			continue
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func companyShareWorkRowOfBoardTask(task centralplane.BoardTask) companyShareWorkRow {
	memberMail := task.RequesterMail
	if len(task.ParticipantMails) > 0 {
		memberMail = task.ParticipantMails[0]
	}
	return companyShareWorkRow{
		MemberID:  memberMail,
		Title:     task.Title,
		Business:  task.Business,
		Type:      task.Type,
		Size:      task.Size,
		Status:    task.Status,
		StartDate: taskDayOfInstant(task.StartsAt),
		EndDate:   taskDayOfInstant(firstFilled(task.EndsAt, task.DueAt)),
		Date:      taskDayOfInstant(task.UpdatedAt),
	}
}

func buildCompanyShareActivityDays(now time.Time, attendanceByDate map[string]int, workByDate map[string]int, workMinutesByDate map[string]int) []companyShareActivityDay {
	days := make([]companyShareActivityDay, 0, companyShareActivityWindowDays)
	for offset := companyShareActivityWindowDays - 1; offset >= 0; offset-- {
		date := now.AddDate(0, 0, -offset).Format("2006-01-02")
		days = append(days, companyShareActivityDay{
			Date: date, AttendanceCount: attendanceByDate[date], WorkCount: workByDate[date], WorkMinutes: workMinutesByDate[date],
		})
	}
	return days
}

func publicCompanyShareWorkStatus(status string) string {
	switch cleanTaskStatus(status) {
	case taskStatusRequested, taskStatusPlanned:
		return "planned"
	case taskStatusInProgress:
		return "inProgress"
	case taskStatusCompleted:
		return "completed"
	case taskStatusPaused:
		return "paused"
	case taskStatusStopped, taskStatusRejected:
		return "closed"
	default:
		return ""
	}
}

func (service *Service) companyShareMembers(sources []companyShareMemberSource) ([]companyShareMember, map[string]string, error) {
	key, errorValue := service.companyShareSigningKey()
	if errorValue != nil {
		return nil, nil, errorValue
	}
	sourceByMemberID := map[string]companyShareMemberSource{}
	for _, source := range sources {
		memberID := companySharePersonID(source.MemberID)
		if memberID == "" {
			continue
		}
		current := sourceByMemberID[memberID]
		current.MemberID = memberID
		current.Name = firstNonEmpty(current.Name, strings.TrimSpace(source.Name))
		current.JobTitle = firstNonEmpty(current.JobTitle, strings.TrimSpace(source.JobTitle))
		sourceByMemberID[memberID] = current
	}
	members := make([]companyShareMember, 0, len(sourceByMemberID))
	seedByMemberID := make(map[string]string, len(sourceByMemberID))
	for memberID, source := range sourceByMemberID {
		seed := companyShareMemberSeed(key, memberID)
		members = append(members, companyShareMember{
			Seed: seed, Surname: publicCompanyShareSurname(source.Name), JobTitle: source.JobTitle,
		})
		seedByMemberID[memberID] = seed
	}
	sort.Slice(members, func(leftIndex int, rightIndex int) bool { return members[leftIndex].Seed < members[rightIndex].Seed })
	return members, seedByMemberID, nil
}

func (service *Service) enrichCompanyShareMemberSources(ctx context.Context, sources []companyShareMemberSource) []companyShareMemberSource {
	fleetID := strings.ToLower(strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)))
	fleetSecret := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetSecretPath))
	var records []adminUserMutation
	if fleetID != "" && fleetSecret != "" {
		if fetched, errorValue := service.lookupUserRecords(ctx, fleetID, fleetSecret); errorValue == nil {
			records = fetched
		}
	}
	records = mergeUserRecordsByEmail(records, service.blueclawPolicyUserRecords(ctx))
	directoryByMemberID := make(map[string]companyShareMemberSource, len(records))
	for _, record := range records {
		source := companyShareMemberSource{
			MemberID: stableTaskID(record.Email), Name: record.Name, JobTitle: record.JobTitle,
		}
		directoryByMemberID[source.MemberID] = source
	}
	result := append([]companyShareMemberSource{}, sources...)
	for index, source := range result {
		directory := directoryByMemberID[companySharePersonID(source.MemberID)]
		result[index].Name = firstNonEmpty(directory.Name, source.Name)
		result[index].JobTitle = firstNonEmpty(directory.JobTitle, source.JobTitle)
	}
	return result
}

func companyShareMemberSeed(key []byte, memberID string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("member\x00" + companySharePersonID(memberID)))
	return hex.EncodeToString(mac.Sum(nil)[:8])
}

func companySharePersonID(value string) string {
	trimmedValue := strings.TrimSpace(value)
	if strings.Contains(trimmedValue, "@") {
		return stableTaskID(strings.ToLower(trimmedValue))
	}
	return trimmedValue
}

func publicCompanyShareSurname(name string) string {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return ""
	}
	compactName := strings.ReplaceAll(trimmedName, " ", "")
	runes := []rune(compactName)
	if len(runes) > 0 && unicode.In(runes[0], unicode.Hangul) {
		for _, compoundSurname := range []string{"남궁", "황보", "제갈", "선우", "서문", "독고", "사공"} {
			if strings.HasPrefix(compactName, compoundSurname) {
				return compoundSurname
			}
		}
		return string(runes[0])
	}
	parts := strings.Fields(trimmedName)
	return parts[len(parts)-1]
}

func companyShareWorkStatuses(statusCounts map[string]int) []companyShareWorkStatus {
	order := []string{"completed", "paused", "closed", "inProgress", "planned"}
	statuses := []companyShareWorkStatus{}
	for _, status := range order {
		if count := statusCounts[status]; count > 0 {
			statuses = append(statuses, companyShareWorkStatus{Status: status, Count: count})
		}
	}
	return statuses
}

func companyShareRecentWorkRows(rows []companyShareWorkRow, seedByMemberID map[string]string) []companyShareRecentWork {
	recent := []companyShareRecentWork{}
	for _, row := range rows {
		if row.Status != "inProgress" && row.Status != "completed" {
			continue
		}
		recent = append(recent, companyShareRecentWork{
			MemberSeed: seedByMemberID[row.MemberID], Title: row.Title,
			Business: row.Business, Type: row.Type, Size: row.Size,
			Status: row.Status, StartDate: row.StartDate, EndDate: row.EndDate, Date: row.Date,
		})
		if len(recent) == 5 {
			break
		}
	}
	return recent
}

func sumCompanyShareActivity(counts map[string]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}
