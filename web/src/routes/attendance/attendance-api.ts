import {
	correctSupabaseAttendanceEvents,
	recordSupabaseAttendance,
	setSupabaseTeamViewVisibility,
	supabaseAttendanceSummary
} from '$lib/attendance/supabase-attendance';
import { supabaseWorkStatus } from '$lib/attendance/supabase-work-status';
import type { AttendanceWorkMode } from '$lib/attendance/work-mode';
import { isSupabaseConfigured } from '$lib/supabase';
import type {
	AttendanceAbsence,
	AttendanceAbsenceKind,
	AttendanceKind,
	AttendanceSummary
} from './attendance-context.svelte';

export type AttendanceSummaryRequest = {
	month: string;
};

export type AttendanceWorkStatusPeriod = 'day' | 'week' | 'month';

export type AttendanceWorkDayStatus = {
	date: string;
	workMode: AttendanceWorkMode;
	hasBaseline: boolean;
	workingDate: boolean;
	targetMinutes: number;
	actualMinutes: number;
	actualSeconds: number;
	provisionalMinutes: number;
	provisionalSeconds: number;
	leaveMinutes: number;
	fulfilledMinutes: number;
	differenceMinutes: number;
	remainingMinutes: number;
	overtimeMinutes: number;
	nightMinutes: number;
	isWorking: boolean;
	needsReview: boolean;
	coreTimeMissed: boolean;
	late: boolean;
	earlyLeave: boolean;
	hasLeaveWorkOverlap: boolean;
	hasIncompleteWorkRecord: boolean;
	status: string;
	workSegments: { startTime: string; endTime: string; provisional: boolean }[];
	leaveSegments: { startTime: string; endTime: string; paid: boolean }[];
};

export type AttendanceEmployeeWorkStatus = {
	email: string;
	displayName: string;
	periodStart: string;
	periodEnd: string;
	workMode: AttendanceWorkMode;
	hasBaseline: boolean;
	targetMinutes: number;
	actualMinutes: number;
	actualSeconds: number;
	provisionalMinutes: number;
	provisionalSeconds: number;
	workingCapacitySeconds: number;
	calendarCapacitySeconds: number;
	leaveMinutes: number;
	fulfilledMinutes: number;
	differenceMinutes: number;
	remainingMinutes: number;
	overtimeMinutes: number;
	nightMinutes: number;
	isWorking: boolean;
	needsReview: boolean;
	coreTimeMissed: boolean;
	late: boolean;
	earlyLeave: boolean;
	hasLeaveWorkOverlap: boolean;
	hasIncompleteRecords: boolean;
	status: string;
	days: AttendanceWorkDayStatus[];
};

export type AttendanceWorkStatus = {
	period: AttendanceWorkStatusPeriod;
	anchor: string;
	periodStart: string;
	periodEnd: string;
	timeZone: string;
	isAdmin: boolean;
	personal?: AttendanceEmployeeWorkStatus;
	employees: AttendanceEmployeeWorkStatus[];
};

export type AttendanceWorkStatusRequest = {
	period: AttendanceWorkStatusPeriod;
	anchor: string;
};

export type CreateAttendanceAbsenceRequest = {
	kind: AttendanceAbsenceKind;
	startDate: string;
	endDate: string;
	reason: string;
};

export type UpdateAttendanceEventRequest = {
	localDate: string;
	localTime: string;
	locationID: string;
	reason: string;
};

export type UpdateAttendanceEvent = {
	eventID: string;
	request: UpdateAttendanceEventRequest;
};

export async function fetchAttendanceSummary(request: AttendanceSummaryRequest): Promise<AttendanceSummary> {
	if (isSupabaseConfigured()) return supabaseAttendanceSummary(request.month);
	const path = attendanceSummaryPath(request);
	const response = await fetch(path, { credentials: 'include', cache: 'no-store' });
	if (!response.ok) throw new Error(await response.text());
	return (await response.json()) as AttendanceSummary;
}

export async function fetchAttendanceWorkStatus(
	request: AttendanceWorkStatusRequest
): Promise<AttendanceWorkStatus> {
	if (isSupabaseConfigured()) return supabaseWorkStatus(request);
	const query = new URLSearchParams({
		period: request.period,
		anchor: request.anchor
	});
	const response = await fetch(`/attendance/api/work-status?${query.toString()}`, {
		credentials: 'include',
		cache: 'no-store'
	});
	if (!response.ok) throw new Error(await response.text());
	return (await response.json()) as AttendanceWorkStatus;
}

export async function createAttendanceAbsence(request: CreateAttendanceAbsenceRequest): Promise<AttendanceAbsence[]> {
	const response = await fetch('/attendance/api/absences', {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(request)
	});
	if (!response.ok) throw new Error(await response.text());
	const payload = (await response.json()) as { absences: AttendanceAbsence[] };
	return payload.absences;
}

export async function deleteAttendanceAbsence(absenceID: string): Promise<void> {
	const response = await fetch(`/attendance/api/absences/${encodeURIComponent(absenceID)}`, {
		method: 'DELETE',
		credentials: 'include'
	});
	if (!response.ok) throw new Error(await response.text());
}

export async function updateAttendanceEvent(eventID: string, request: UpdateAttendanceEventRequest): Promise<void> {
	return updateAttendanceEvents([{ eventID, request }]);
}

export async function updateAttendanceEvents(updates: UpdateAttendanceEvent[]): Promise<void> {
	if (updates.length === 0) return;
	if (isSupabaseConfigured()) {
		const reasons = new Set(updates.map((update) => update.request.reason.trim()));
		if (reasons.size !== 1) throw new Error('attendance corrections must share one reason');
		return correctSupabaseAttendanceEvents(
			updates.map((update) => ({
				eventID: update.eventID,
				localDate: update.request.localDate,
				localTime: update.request.localTime,
				locationID: update.request.locationID
			})),
			[...reasons][0]
		);
	}
	await Promise.all(updates.map((update) => patchAttendanceEvent(update.eventID, update.request)));
}

async function patchAttendanceEvent(eventID: string, request: UpdateAttendanceEventRequest): Promise<void> {
	const response = await fetch(`/attendance/api/events/${encodeURIComponent(eventID)}`, {
		method: 'PATCH',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(request)
	});
	if (!response.ok) throw new Error(await response.text());
}

export async function updateAttendanceTeamViewVisibility(visible: boolean): Promise<void> {
	if (isSupabaseConfigured()) return setSupabaseTeamViewVisibility(visible);
	const response = await fetch('/attendance/api/settings', {
		method: 'PATCH',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ teamViewVisibleToAll: visible })
	});
	if (!response.ok) throw new Error(await response.text());
}

export async function toggleAttendanceOnServer(
	kind?: AttendanceKind,
	locationID?: string,
	confirmEarlyReturn = false
): Promise<void> {
	if (isSupabaseConfigured()) return recordSupabaseAttendance(kind, locationID);
	const response = await fetch('/attendance/api/clock', {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			kind: kind ?? '',
			locationID: locationID ?? '',
			confirmEarlyReturn
		})
	});
	if (!response.ok) throw new Error(await response.text());
}

function attendanceSummaryPath(request: AttendanceSummaryRequest): string {
	const query = new URLSearchParams();
	if (request.month) query.set('month', request.month);
	const queryString = query.toString();
	return queryString ? `/attendance/api/summary?${queryString}` : '/attendance/api/summary';
}
