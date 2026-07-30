package admind

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	attendanceLeavePolicyVersion               = 2
	attendanceLeavePolicyReferenceCalendarYear = 2024
	attendanceLeaveBalanceTrackingManaged      = "managed"
	attendanceLeaveBalanceTrackingUnlimited    = "unlimited"
)

type attendanceLeavePolicy struct {
	Version              int                   `json:"version"`
	BalanceTrackingMode  string                `json:"balanceTrackingMode"`
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
	IncludeInSummary        bool     `json:"includeInSummary"`
	LegacyEvidenceGuidance  string   `json:"evidenceGuidance,omitempty"`
	IsActive                bool     `json:"isActive"`
	IsSystem                bool     `json:"isSystem"`
	SortOrder               int      `json:"sortOrder"`
}

func newAttendanceLeaveTypeID() (string, error) {
	bytesValue := make([]byte, 12)
	if _, errorValue := rand.Read(bytesValue); errorValue != nil {
		return "", errorValue
	}
	return "custom-" + hex.EncodeToString(bytesValue), nil
}

func validateAttendanceLeavePolicy(policy *attendanceLeavePolicy, _ *attendanceLeavePolicy) error {
	if policy.Version != attendanceLeavePolicyVersion || !attendanceLeavePolicyFiscalDateIsValid(policy.FiscalYearStartMonth, policy.FiscalYearStartDay) {
		return fmt.Errorf("invalid policy header")
	}
	if policy.BalanceTrackingMode != attendanceLeaveBalanceTrackingManaged &&
		policy.BalanceTrackingMode != attendanceLeaveBalanceTrackingUnlimited {
		return fmt.Errorf("invalid balanceTrackingMode")
	}
	if policy.UpdatedAt != "" {
		if _, errorValue := time.Parse(time.RFC3339, policy.UpdatedAt); errorValue != nil {
			return fmt.Errorf("invalid policy update time")
		}
	}
	systems := defaultAttendanceLeavePolicy().LeaveTypes
	byID := map[string]attendanceLeaveType{}
	byName := map[string]bool{}
	hasSharedAnnualBalance := false
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
		if leaveType.IsActive && leaveType.BalanceMode == "annual" && leaveType.ID != attendanceAnnualLeaveTypeID {
			hasSharedAnnualBalance = true
		}
		if leaveType.GrantCadence != "annual" && leaveType.GrantCadence != "monthly" && leaveType.GrantCadence != "none" {
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
		if !attendanceLeaveTypeOwnsBalance(*leaveType) &&
			(leaveType.GrantCadence != "none" ||
				leaveType.GrantAmountMilliDays != 0 ||
				leaveType.ExpiryMode != "none" ||
				leaveType.CarryoverEnabled ||
				leaveType.IncludeInSummary) {
			return fmt.Errorf("shared or untracked leave type cannot define a separate balance policy")
		}
		if !leaveType.IsActive && leaveType.IncludeInSummary {
			return fmt.Errorf("inactive leave type cannot be included in summary")
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
	if hasSharedAnnualBalance {
		annualType, found := byID[attendanceAnnualLeaveTypeID]
		if !found || !annualType.IsActive || !attendanceLeaveTypeOwnsBalance(annualType) {
			return fmt.Errorf("annual balance owner is required")
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

func attendanceLeaveTypeRequiresHireDate(leaveType attendanceLeaveType) bool {
	return leaveType.IsActive &&
		leaveType.BalanceMode != "none" &&
		leaveType.GrantCadence != "none"
}

func attendanceLeavePolicyRequiresHireDate(policy attendanceLeavePolicy) bool {
	if policy.BalanceTrackingMode == attendanceLeaveBalanceTrackingUnlimited {
		return false
	}
	for _, leaveType := range policy.LeaveTypes {
		if attendanceLeaveTypeRequiresHireDate(leaveType) {
			return true
		}
	}
	return false
}
