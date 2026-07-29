package admind

import (
	"fmt"
	"time"
)

func attendanceWorkScheduleWallTime(
	date time.Time,
	minute int,
	location *time.Location,
) (time.Time, error) {
	if minute < 0 || minute >= attendanceWorkScheduleMinutesPerDay {
		return time.Time{}, fmt.Errorf("time is outside the day boundary")
	}
	localTime := time.Date(
		date.Year(),
		date.Month(),
		date.Day(),
		minute/60,
		minute%60,
		0,
		0,
		location,
	)
	if !attendanceWorkScheduleSameWallTime(localTime, date, minute) {
		return time.Time{}, fmt.Errorf("time does not exist in the company time zone")
	}
	if attendanceWorkScheduleWallTimeIsAmbiguous(localTime, date, minute, location) {
		return time.Time{}, fmt.Errorf("time is ambiguous in the company time zone")
	}
	return localTime, nil
}

func attendanceWorkScheduleWallTimeIsAmbiguous(
	localTime time.Time,
	date time.Time,
	minute int,
	location *time.Location,
) bool {
	offsets := attendanceWorkScheduleNeighboringOffsets(localTime, location)
	wallTimeInUTC := time.Date(
		date.Year(),
		date.Month(),
		date.Day(),
		minute/60,
		minute%60,
		0,
		0,
		time.UTC,
	)
	matches := 0
	for offset := range offsets {
		candidate := wallTimeInUTC.Add(-time.Duration(offset) * time.Second).In(location)
		if attendanceWorkScheduleSameWallTime(candidate, date, minute) {
			matches++
		}
	}
	return matches > 1
}

func attendanceWorkScheduleNeighboringOffsets(localTime time.Time, location *time.Location) map[int]struct{} {
	offsets := map[int]struct{}{}
	_, currentOffset := localTime.Zone()
	offsets[currentOffset] = struct{}{}
	zoneStart, zoneEnd := localTime.ZoneBounds()
	if !zoneStart.IsZero() {
		_, previousOffset := zoneStart.Add(-time.Second).In(location).Zone()
		offsets[previousOffset] = struct{}{}
	}
	if !zoneEnd.IsZero() {
		_, nextOffset := zoneEnd.In(location).Zone()
		offsets[nextOffset] = struct{}{}
	}
	return offsets
}

func attendanceWorkScheduleSameWallTime(candidate time.Time, date time.Time, minute int) bool {
	return candidate.Year() == date.Year() &&
		candidate.Month() == date.Month() &&
		candidate.Day() == date.Day() &&
		candidate.Hour() == minute/60 &&
		candidate.Minute() == minute%60
}
