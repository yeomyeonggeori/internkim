import { invokeTool } from '$lib/public-api-call';
import type { OrderableMember } from '$lib/member-order';
import type { AttendanceKind } from '../../routes/attendance/attendance-context.svelte';

export type RecordWorkLocation = { name: string; color: string | null };

export type RecordCompanySettings = {
	name: string;
	locale: string;
	timeZone: string;
	currencyCode: string;
	workLocations: RecordWorkLocation[];
	leaveDays: number | null;
	teamViewVisibleToAll: boolean;
	profileImageURL: string | null;
};

export type RecordPerson = {
	personID: string;
	name: string;
	email: string;
	isAdmin?: boolean;
	hireDate?: string;
	timeZone?: string;
};

export type RecordDirectory = {
	requesterID: string;
	count: number;
	people: RecordPerson[];
};

export type RecordAttendance = {
	eventID: string;
	personID: string;
	person: string;
	kind: AttendanceKind;
	date: string;
	time: string;
	occurredAt: string;
	location: string | null;
	wasCorrected: boolean;
	originalDate: string | null;
	originalTime: string | null;
	originalOccurredAt: string | null;
	reason: string | null;
};

export type RecordAttendanceList = {
	from: string;
	to: string;
	serverTime: string;
	backdatedAfterMinutes: number;
	count: number;
	attendance: RecordAttendance[];
};

export type RecordLeaveStatus = 'requested' | 'approved' | 'rejected';

export type RecordLeave = {
	leaveID: string;
	personID: string;
	person: string;
	kindID: string;
	kind: string;
	days: number;
	status: RecordLeaveStatus;
	isPaid: boolean;
	isDeducted: boolean;
	startDate: string;
	endDate: string;
	startsAt: string;
	endsAt: string;
	note: string | null;
};

export type RecordLeaveList = {
	count: number;
	leave: RecordLeave[];
	registeredKinds: string[];
};

export type RecordLeaveBalance = {
	personID: string;
	personName: string;
	year: number;
	grantedDays: number | null;
	remainingDays: number | null;
	usedDays: number | null;
	tracking: string;
};

export function companySettings(): Promise<RecordCompanySettings> {
	return invokeTool('company_settings_get', {});
}

export function companyDirectory(): Promise<RecordDirectory> {
	return invokeTool('person_list', {});
}

export function attendanceBetween(from: string, to: string): Promise<RecordAttendanceList> {
	return invokeTool('attendance_list', { scope: 'all', from, to });
}

export function approvedLeaveBetween(from: string, to: string): Promise<RecordLeaveList> {
	return invokeTool('leave_list', { scope: 'all', status: 'approved', from, to });
}

export function everyLeaveOfTheCompany(): Promise<RecordLeaveList> {
	return invokeTool('leave_list', { scope: 'all' });
}

export function myLeaveBalance(): Promise<RecordLeaveBalance> {
	return invokeTool('leave_balance', {});
}

export function timeZoneOfPerson(person: RecordPerson | undefined, companyZone: string): string {
	return person?.timeZone || companyZone;
}

export function orderablePersonOf(person: RecordPerson): OrderableMember {
	return {
		id: person.personID,
		name: person.name || null,
		email: person.email || null,
		joined_at: person.hireDate || null
	};
}
