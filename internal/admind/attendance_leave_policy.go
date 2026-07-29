package admind

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	attendanceLeavePolicyVersion               = 1
	attendanceAnnualStatutoryGrantMilliDays    = 15000
	attendanceLeavePolicyReferenceCalendarYear = 2024
)

type attendanceLeavePolicy struct {
	Version              int                   `json:"version"`
	FiscalYearStartMonth int                   `json:"fiscalYearStartMonth"`
	FiscalYearStartDay   int                   `json:"fiscalYearStartDay"`
	LeaveTypes           []attendanceLeaveType `json:"leaveTypes"`
	UpdatedAt            string                `json:"updatedAt"`
}

type attendanceLeaveType struct {
	ID                      string   `json:"id"`
	SystemKind              string   `json:"systemKind"`
	Name                    string   `json:"name"`
	Paid                    bool     `json:"paid"`
	BalanceMode             string   `json:"balanceMode"`
	GrantCadence            string   `json:"grantCadence"`
	GrantAmountMilliDays    int      `json:"grantAmountMilliDays"`
	ExpiryMode              string   `json:"expiryMode"`
	ExpiryMonths            *int     `json:"expiryMonths"`
	CarryoverEnabled        bool     `json:"carryoverEnabled"`
	CarryoverLimitMilliDays *int     `json:"carryoverLimitMilliDays"`
	AllowedUnits            []string `json:"allowedUnits"`
	LegacyEvidenceGuidance  string   `json:"evidenceGuidance,omitempty"`
	IsActive                bool     `json:"isActive"`
	IsSystem                bool     `json:"isSystem"`
	SortOrder               int      `json:"sortOrder"`
}

func defaultAttendanceLeavePolicy() attendanceLeavePolicy {
	return attendanceLeavePolicy{Version: attendanceLeavePolicyVersion, FiscalYearStartMonth: 1, FiscalYearStartDay: 1, LeaveTypes: []attendanceLeaveType{
		{ID: "annual", SystemKind: "annual", Name: "연차", Paid: true, BalanceMode: "annual", GrantCadence: "statutory", GrantAmountMilliDays: attendanceAnnualStatutoryGrantMilliDays, ExpiryMode: "fiscalYearEnd", AllowedUnits: attendanceDefaultPartialLeaveUnits(), IsActive: true, IsSystem: true, SortOrder: 0},
		{ID: "sick", SystemKind: "sick", Name: "병가", BalanceMode: "none", GrantCadence: "none", ExpiryMode: "none", AllowedUnits: attendanceDefaultPartialLeaveUnits(), IsActive: true, IsSystem: true, SortOrder: 1},
		{ID: "bereavement", SystemKind: "bereavement", Name: "경조휴가", BalanceMode: "separate", GrantCadence: "manual", ExpiryMode: "none", AllowedUnits: []string{"fullDay"}, IsActive: true, IsSystem: true, SortOrder: 2},
		{ID: "unpaid", SystemKind: "unpaid", Name: "무급휴가", BalanceMode: "none", GrantCadence: "none", ExpiryMode: "none", AllowedUnits: attendanceDefaultPartialLeaveUnits(), IsActive: true, IsSystem: true, SortOrder: 3},
		{ID: "other", SystemKind: "other", Name: "기타 휴가", BalanceMode: "none", GrantCadence: "none", ExpiryMode: "none", AllowedUnits: []string{"fullDay"}, IsActive: true, IsSystem: true, SortOrder: 4},
	}}
}

func attendanceDefaultPartialLeaveUnits() []string {
	return []string{"fullDay", "halfDay", "quarterDay"}
}

func newAttendanceLeaveTypeID() (string, error) {
	bytesValue := make([]byte, 12)
	if _, errorValue := rand.Read(bytesValue); errorValue != nil {
		return "", errorValue
	}
	return "custom-" + hex.EncodeToString(bytesValue), nil
}

func validateAttendanceLeavePolicy(policy *attendanceLeavePolicy, existing *attendanceLeavePolicy) error {
	if policy.Version != attendanceLeavePolicyVersion || !attendanceLeavePolicyFiscalDateIsValid(policy.FiscalYearStartMonth, policy.FiscalYearStartDay) {
		return fmt.Errorf("invalid policy header")
	}
	if policy.UpdatedAt != "" {
		if _, errorValue := time.Parse(time.RFC3339, policy.UpdatedAt); errorValue != nil {
			return fmt.Errorf("invalid policy update time")
		}
	}
	systems := defaultAttendanceLeavePolicy().LeaveTypes
	byID := map[string]attendanceLeaveType{}
	byName := map[string]bool{}
	for index := range policy.LeaveTypes {
		leaveType := &policy.LeaveTypes[index]
		leaveType.ID = strings.TrimSpace(leaveType.ID)
		leaveType.Name = strings.TrimSpace(leaveType.Name)
		leaveType.LegacyEvidenceGuidance = ""
		if leaveType.ID == "" {
			id, errorValue := newAttendanceLeaveTypeID()
			if errorValue != nil {
				return errorValue
			}
			leaveType.ID = id
		}
		if leaveType.Name == "" || len(leaveType.AllowedUnits) == 0 || leaveType.GrantAmountMilliDays < 0 || leaveType.SortOrder < 0 {
			return fmt.Errorf("invalid leave type %s", leaveType.ID)
		}
		normalizedName := strings.ToLower(leaveType.Name)
		if _, exists := byID[leaveType.ID]; exists || byName[normalizedName] {
			return fmt.Errorf("duplicate leave type")
		}
		byID[leaveType.ID], byName[normalizedName] = *leaveType, true
		if leaveType.BalanceMode != "annual" && leaveType.BalanceMode != "separate" && leaveType.BalanceMode != "none" {
			return fmt.Errorf("invalid balanceMode")
		}
		if leaveType.GrantCadence != "statutory" && leaveType.GrantCadence != "annual" && leaveType.GrantCadence != "monthly" && leaveType.GrantCadence != "manual" && leaveType.GrantCadence != "none" {
			return fmt.Errorf("invalid grantCadence")
		}
		if leaveType.ExpiryMode != "fiscalYearEnd" && leaveType.ExpiryMode != "monthsAfterGrant" && leaveType.ExpiryMode != "none" {
			return fmt.Errorf("invalid expiryMode")
		}
		if leaveType.ExpiryMode == "monthsAfterGrant" && (leaveType.ExpiryMonths == nil || *leaveType.ExpiryMonths <= 0) {
			return fmt.Errorf("invalid expiryMonths")
		}
		if leaveType.ExpiryMode != "monthsAfterGrant" {
			leaveType.ExpiryMonths = nil
		}
		if leaveType.CarryoverEnabled && leaveType.CarryoverLimitMilliDays != nil && *leaveType.CarryoverLimitMilliDays < 0 {
			return fmt.Errorf("invalid carryover limit")
		}
		if !leaveType.CarryoverEnabled {
			leaveType.CarryoverLimitMilliDays = nil
		}
		seenUnits := map[string]bool{}
		for _, unit := range leaveType.AllowedUnits {
			if (unit != "fullDay" && unit != "halfDay" && unit != "quarterDay") || seenUnits[unit] {
				return fmt.Errorf("invalid allowed unit")
			}
			seenUnits[unit] = true
		}
		isKnownSystem := false
		for _, system := range systems {
			if leaveType.ID == system.ID {
				isKnownSystem = true
				break
			}
		}
		if !isKnownSystem && (leaveType.IsSystem || leaveType.SystemKind != "") {
			return fmt.Errorf("custom leave type identity is immutable")
		}
		if leaveType.IsSystem {
			for _, system := range systems {
				if system.ID == leaveType.ID && (leaveType.SystemKind != system.SystemKind || !leaveType.IsSystem) {
					return fmt.Errorf("system identity cannot change")
				}
			}
		}
	}
	for _, system := range systems {
		leaveType, ok := byID[system.ID]
		if !ok || !leaveType.IsSystem || leaveType.SystemKind != system.SystemKind {
			return fmt.Errorf("system leave type cannot be deleted")
		}
		if system.ID == "annual" && (!leaveType.IsActive || !leaveType.Paid || leaveType.BalanceMode != "annual" || leaveType.GrantCadence != "statutory" || leaveType.GrantAmountMilliDays < attendanceAnnualStatutoryGrantMilliDays) {
			return fmt.Errorf("annual legal floor")
		}
	}
	if existing != nil {
		for _, oldType := range existing.LeaveTypes {
			if _, ok := byID[oldType.ID]; !ok {
				return fmt.Errorf("existing leave type cannot be deleted")
			}
		}
	}
	return nil
}

func attendanceLeavePolicyFiscalDateIsValid(month int, day int) bool {
	if month < 1 || month > 12 || day < 1 {
		return false
	}
	date := time.Date(attendanceLeavePolicyReferenceCalendarYear, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return int(date.Month()) == month && date.Day() == day
}
