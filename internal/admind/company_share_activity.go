package admind

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"
)

const companyShareActivityWindowDays = 52 * 7

type companyShareActivityDay struct {
	Date            string `json:"date"`
	AttendanceCount int    `json:"attendanceCount"`
	WorkCount       int    `json:"workCount"`
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
	Image    string `json:"image,omitempty"`
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
	startDate := now.AddDate(0, 0, -(companyShareActivityWindowDays - 1)).Format("2006-01-02")
	endDate := now.Format("2006-01-02")
	attendanceByDate, attendanceMembers, errorValue := service.readCompanyShareAttendanceActivity(ctx, startDate, endDate)
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
	days := buildCompanyShareActivityDays(now, attendanceByDate, workByDate)
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

func (service *Service) readCompanyShareAttendanceActivity(ctx context.Context, startDate string, endDate string) (map[string]int, []companyShareMemberSource, error) {
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT local_date, COUNT(DISTINCT email)
FROM attendance_events
WHERE local_date >= ? AND local_date <= ? AND kind = ? AND canceled_at = ''
GROUP BY local_date`, startDate, endDate, attendanceKindClockIn)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var date string
		var count int
		if errorValue := rows.Scan(&date, &count); errorValue != nil {
			return nil, nil, errorValue
		}
		counts[date] = count
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, nil, errorValue
	}
	memberRows, errorValue := database.QueryContext(ctx, `
SELECT DISTINCT email, display_name
FROM attendance_events
WHERE local_date >= ? AND local_date <= ? AND kind = ? AND canceled_at = ''`, startDate, endDate, attendanceKindClockIn)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	defer memberRows.Close()
	members := []companyShareMemberSource{}
	for memberRows.Next() {
		var email string
		var name string
		if errorValue := memberRows.Scan(&email, &name); errorValue != nil {
			return nil, nil, errorValue
		}
		members = append(members, companyShareMemberSource{MemberID: companySharePersonID(email), Name: name})
	}
	return counts, members, memberRows.Err()
}

func (service *Service) readCompanyShareWorkActivity(ctx context.Context, startDate string, endDate string) (map[string]int, []companyShareWorkRow, map[string]int, []companyShareMemberSource, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return nil, nil, nil, nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT owner_id, owner_name, content, business, type, size, status, start_date, end_date, substr(updated_at, 1, 10)
FROM flow_tasks
WHERE substr(updated_at, 1, 10) >= ? AND substr(updated_at, 1, 10) <= ?
ORDER BY updated_at DESC`, startDate, endDate)
	if errorValue != nil {
		return nil, nil, nil, nil, errorValue
	}
	defer rows.Close()
	counts := map[string]int{}
	statusCounts := map[string]int{}
	members := []companyShareMemberSource{}
	workRows := []companyShareWorkRow{}
	for rows.Next() {
		var row companyShareWorkRow
		if errorValue := rows.Scan(
			&row.MemberID, &row.Name, &row.Title, &row.Business, &row.Type, &row.Size,
			&row.Status, &row.StartDate, &row.EndDate, &row.Date,
		); errorValue != nil {
			return nil, nil, nil, nil, errorValue
		}
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
	return counts, workRows, statusCounts, members, rows.Err()
}

func buildCompanyShareActivityDays(now time.Time, attendanceByDate map[string]int, workByDate map[string]int) []companyShareActivityDay {
	days := make([]companyShareActivityDay, 0, companyShareActivityWindowDays)
	for offset := companyShareActivityWindowDays - 1; offset >= 0; offset-- {
		date := now.AddDate(0, 0, -offset).Format("2006-01-02")
		days = append(days, companyShareActivityDay{Date: date, AttendanceCount: attendanceByDate[date], WorkCount: workByDate[date]})
	}
	return days
}

func publicCompanyShareWorkStatus(status string) string {
	switch cleanFlowStatus(status) {
	case flowStatusRequested, flowStatusPlanned:
		return "planned"
	case flowStatusInProgress:
		return "inProgress"
	case flowStatusCompleted:
		return "completed"
	case flowStatusPaused:
		return "paused"
	case flowStatusStopped, flowStatusRejected:
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
			Image: "/company/api/team/" + seed + "/image",
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
			MemberID: stableFlowID(record.Email), Name: record.Name, JobTitle: record.JobTitle,
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
		return stableFlowID(strings.ToLower(trimmedValue))
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

func (service *Service) serveCompanyShareMemberImage(responseWriter http.ResponseWriter, request *http.Request, seed string) {
	settings, errorValue := service.readCompanyShareSettings()
	if errorValue != nil || !settings.Enabled || !service.hasValidCompanyShareSession(request, settings) {
		http.NotFound(responseWriter, request)
		return
	}
	key, errorValue := service.companyShareSigningKey()
	if errorValue != nil {
		http.NotFound(responseWriter, request)
		return
	}
	for _, member := range service.flowMembers(request) {
		if hmac.Equal([]byte(companyShareMemberSeed(key, member.ID)), []byte(seed)) {
			path := "/participants/" + url.PathEscape(member.ID) + "/image"
			service.serveCalendarParticipantImage(responseWriter, request, path)
			return
		}
	}
	http.NotFound(responseWriter, request)
}

func sumCompanyShareActivity(counts map[string]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}
