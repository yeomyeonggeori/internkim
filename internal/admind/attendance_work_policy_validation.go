package admind

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func validateAndNormalizeAttendanceWorkPolicy(policy *attendanceWorkPolicy) error {
	if policy.Version != attendanceWorkPolicyVersion {
		return fmt.Errorf("version must be %d", attendanceWorkPolicyVersion)
	}
	if _, errorValue := time.Parse(time.RFC3339, policy.UpdatedAt); errorValue != nil {
		return fmt.Errorf("updatedAt must use RFC3339: %w", errorValue)
	}
	if len(policy.Revisions) == 0 {
		return fmt.Errorf("revisions must not be empty")
	}
	revisions := append([]attendanceWorkPolicyRevision(nil), policy.Revisions...)
	for index := range revisions {
		if errorValue := validateAndNormalizeAttendanceWorkPolicyRevision(&revisions[index]); errorValue != nil {
			return fmt.Errorf("invalid revision %d: %w", index, errorValue)
		}
	}
	sort.Slice(revisions, func(left int, right int) bool {
		return revisions[left].EffectiveDate < revisions[right].EffectiveDate
	})
	for index := 1; index < len(revisions); index++ {
		if revisions[index-1].EffectiveDate == revisions[index].EffectiveDate {
			return fmt.Errorf("revision effective dates must be unique")
		}
	}
	if revisions[0].EffectiveDate != attendanceWorkPolicyInitialEffectiveDate {
		return fmt.Errorf("initial revision must start on %s", attendanceWorkPolicyInitialEffectiveDate)
	}
	policy.Revisions = revisions
	return nil
}

func validateAndNormalizeAttendanceWorkPolicyRevision(revision *attendanceWorkPolicyRevision) error {
	revision.EffectiveDate = strings.TrimSpace(revision.EffectiveDate)
	revision.WorkMode = strings.TrimSpace(revision.WorkMode)
	revision.ReferenceStartTime = strings.TrimSpace(revision.ReferenceStartTime)
	revision.FixedStartTime = strings.TrimSpace(revision.FixedStartTime)
	revision.FixedEndTime = strings.TrimSpace(revision.FixedEndTime)
	revision.CoreStartTime = strings.TrimSpace(revision.CoreStartTime)
	revision.CoreEndTime = strings.TrimSpace(revision.CoreEndTime)
	revision.NightStartTime = strings.TrimSpace(revision.NightStartTime)
	revision.NightEndTime = strings.TrimSpace(revision.NightEndTime)
	if _, errorValue := attendanceWorkScheduleDate(revision.EffectiveDate, time.UTC); errorValue != nil {
		return fmt.Errorf("invalid effectiveDate: %w", errorValue)
	}
	if revision.WorkMode != attendanceWorkModeAutonomous &&
		revision.WorkMode != attendanceWorkModeFlexible &&
		revision.WorkMode != attendanceWorkModeFixed {
		return fmt.Errorf("workMode must be autonomous, flexible, or fixed")
	}
	if revision.DailyTargetMinutes <= 0 ||
		revision.DailyTargetMinutes > attendanceWorkScheduleMaximumMinutes {
		return fmt.Errorf("dailyTargetMinutes must be between 1 and %d", attendanceWorkScheduleMaximumMinutes)
	}
	schedule := attendanceWorkSchedule{
		Version:              attendanceWorkScheduleVersion,
		WorkMode:             attendanceWorkModeFlexible,
		WorkingWeekdays:      revision.WorkingWeekdays,
		ScheduledWorkMinutes: revision.DailyTargetMinutes,
		ReferenceStartTime:   revision.ReferenceStartTime,
		BreakPeriods:         revision.BreakPeriods,
	}
	if revision.WorkMode == attendanceWorkModeFixed {
		schedule.WorkMode = attendanceWorkModeFixed
		schedule.FixedStartTime = revision.FixedStartTime
		schedule.FixedEndTime = revision.FixedEndTime
	}
	if errorValue := validateAndNormalizeAttendanceWorkSchedule(&schedule); errorValue != nil {
		return errorValue
	}
	revision.WorkingWeekdays = schedule.WorkingWeekdays
	revision.ReferenceStartTime = schedule.ReferenceStartTime
	revision.FixedStartTime = schedule.FixedStartTime
	revision.FixedEndTime = schedule.FixedEndTime
	revision.BreakPeriods = schedule.BreakPeriods
	switch revision.WorkMode {
	case attendanceWorkModeAutonomous:
		revision.WeeklyTargetMinutes = 0
		revision.CoreTimeEnabled = false
		revision.CoreStartTime = ""
		revision.CoreEndTime = ""
		revision.FixedStartTime = ""
		revision.FixedEndTime = ""
	case attendanceWorkModeFlexible:
		expectedWeeklyMinutes := revision.DailyTargetMinutes * len(revision.WorkingWeekdays)
		if revision.WeeklyTargetMinutes != expectedWeeklyMinutes {
			return fmt.Errorf("weeklyTargetMinutes must equal dailyTargetMinutes multiplied by working weekdays")
		}
		revision.FixedStartTime = ""
		revision.FixedEndTime = ""
		if errorValue := validateAttendanceCoreTime(revision); errorValue != nil {
			return errorValue
		}
	case attendanceWorkModeFixed:
		expectedWeeklyMinutes := revision.DailyTargetMinutes * len(revision.WorkingWeekdays)
		if revision.WeeklyTargetMinutes != expectedWeeklyMinutes {
			return fmt.Errorf("weeklyTargetMinutes must equal dailyTargetMinutes multiplied by working weekdays")
		}
		revision.CoreTimeEnabled = false
		revision.CoreStartTime = ""
		revision.CoreEndTime = ""
	}
	nightStartMinute, startError := attendanceWorkScheduleTimeMinutes(revision.NightStartTime)
	nightEndMinute, endError := attendanceWorkScheduleTimeMinutes(revision.NightEndTime)
	if startError != nil || endError != nil || nightStartMinute == nightEndMinute {
		return fmt.Errorf("night time must contain two different HH:mm values")
	}
	return nil
}

func validateAttendanceCoreTime(revision *attendanceWorkPolicyRevision) error {
	if !revision.CoreTimeEnabled {
		revision.CoreStartTime = ""
		revision.CoreEndTime = ""
		return nil
	}
	startMinute, startError := attendanceWorkScheduleTimeMinutes(revision.CoreStartTime)
	endMinute, endError := attendanceWorkScheduleTimeMinutes(revision.CoreEndTime)
	if startError != nil || endError != nil || startMinute >= endMinute {
		return fmt.Errorf("core time must be a valid same-day range")
	}
	return nil
}

func attendanceWorkPolicyRevisionForDate(
	policy attendanceWorkPolicy,
	date string,
) (attendanceWorkPolicyRevision, error) {
	if _, errorValue := attendanceWorkScheduleDate(date, time.UTC); errorValue != nil {
		return attendanceWorkPolicyRevision{}, errorValue
	}
	if errorValue := validateAndNormalizeAttendanceWorkPolicy(&policy); errorValue != nil {
		return attendanceWorkPolicyRevision{}, errorValue
	}
	selected := policy.Revisions[0]
	for _, revision := range policy.Revisions {
		if revision.EffectiveDate > date {
			break
		}
		selected = revision
	}
	return selected, nil
}
