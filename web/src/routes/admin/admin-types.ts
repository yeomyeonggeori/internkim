import type { adminText } from './text';

import type { UserRole } from '$lib/types';
import type { PageText } from '$lib/i18n/page-text.svelte';

export type { UserRecord } from '$lib/organization/types';
export type { UserRole } from '$lib/types';

export type AdminPageText = PageText<typeof adminText>;

export type CircleRecord = {
	circleID: string;
	displayName: string;
};

export type AdminSession = {
	email: string;
	image?: string;
	claimedAdminEmail: string;
	isAdmin: boolean;
	role?: UserRole;
	canViewTasks?: boolean;
};

export type AgentToneRegister = 'formal' | 'polite' | 'casual';

export type AgentSoul = {
	schemaVersion: 1;
	values?: string[];
	boundaries?: string[];
	workingStyle?: string[];
	tone?: { register?: AgentToneRegister; traits?: string[] };
	language?: { default?: string; matchRequester?: boolean };
};

export type AgentUser = {
	schemaVersion: 1;
	callMe?: string;
	about?: string;
	preferences?: string[];
	tone?: { register?: AgentToneRegister; traits?: string[] };
	language?: { default?: string };
	morningBriefing?: { enabled?: boolean; time?: string };
};

export type CalendarHolidaySyncState = 'healthy' | 'degraded' | 'neverSynced';

export type LeaveBalanceMode = 'annual' | 'separate' | 'none';
export type LeaveBalanceTrackingMode = 'managed' | 'unlimited';
export type LeaveGrantCadence = 'annual' | 'monthly' | 'none';
export type LeaveExpiryMode = 'fiscalYearEnd' | 'monthsAfterGrant' | 'none';
export type LeaveAllowedUnit = 'fullDay' | 'halfDay' | 'quarterDay';

export type LeaveType = {
	id: string;
	systemKind: string;
	name: string;
	paid: boolean;
	balanceMode: LeaveBalanceMode;
	grantCadence: LeaveGrantCadence;
	grantAmountMilliDays: number;
	expiryMode: LeaveExpiryMode;
	expiryMonths?: number;
	carryoverEnabled: boolean;
	carryoverLimitMilliDays?: number;
	usageLimitMilliDays?: number;
	allowedUnits: LeaveAllowedUnit[];
	includeInSummary: boolean;
	isActive: boolean;
	isSystem: boolean;
	sortOrder: number;
};

export type AttendanceLeavePolicy = {
	version: 2;
	balanceTrackingMode: LeaveBalanceTrackingMode;
	fiscalYearStartMonth: number;
	fiscalYearStartDay: number;
	leaveTypes: LeaveType[];
	updatedAt: string;
};

export type CompanyHoliday = {
	id: string;
	title: string;
	date: string;
	recursAnnually: boolean;
	createdAt: string;
	updatedAt: string;
};

export type CompanyHolidayInput = Pick<CompanyHoliday, 'title' | 'date' | 'recursAnnually'>;

export type CompanyHolidaysResponse = {
	holidays: CompanyHoliday[];
};

export type AttendanceWorkMode = 'autonomous' | 'flexible' | 'fixed';

export type AttendanceWorkBreakPeriod = {
	startTime: string;
	endTime: string;
};

export type AttendanceWorkPolicyRevision = {
	effectiveDate: string;
	workMode: AttendanceWorkMode;
	workingWeekdays: number[];
	dailyTargetMinutes: number;
	weeklyTargetMinutes: number;
	referenceStartTime: string;
	fixedStartTime: string;
	fixedEndTime: string;
	coreTimeEnabled: boolean;
	coreStartTime: string;
	coreEndTime: string;
	breakPeriods: AttendanceWorkBreakPeriod[];
	nightStartTime: string;
	nightEndTime: string;
};

export type AttendanceWorkPolicy = {
	version: 1;
	updatedAt: string;
	revisions: AttendanceWorkPolicyRevision[];
};

export type AttendanceWorkPolicyResponse = {
	policy: AttendanceWorkPolicy;
	currentMonth: string;
	holidayDates: string[];
	timeZone: string;
};
