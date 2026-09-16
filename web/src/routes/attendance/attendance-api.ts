import {
	addSupabaseAttendanceEvent,
	correctSupabaseAttendanceEvents,
	recordSupabaseAttendance,
	removeSupabaseAttendanceEvent,
	setSupabaseTeamViewVisibility,
	supabaseAttendanceSummary
} from '$lib/attendance/supabase-attendance';
import type { AttendanceWriteResult } from '$lib/attendance/attendance-write';
import {
	attendanceWorkStatusFrom,
	rangeCovers,
	supabaseWorkStatus,
	supabaseWorkStatusInputs,
	type SupabaseWorkStatusInputs
} from '$lib/attendance/supabase-work-status';
import type { AttendanceWorkMode } from '$lib/attendance/work-mode';
import type { AttendanceKind, AttendanceSummary } from './attendance-context.svelte';

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
	referenceDailyMinutes?: number;
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

export type AddAttendanceEventRequest = {
	email: string;
	kind: AttendanceKind;
	localDate: string;
	localTime: string;
	locationID: string;
	reason: string;
};

export type RemoveAttendanceEventRequest = {
	eventID: string;
	reason: string;
};

export function fetchAttendanceSummary(
	request: AttendanceSummaryRequest
): Promise<AttendanceSummary> {
	return supabaseAttendanceSummary(request.month);
}

export type AttendanceWorkStatusPair = {
	period: AttendanceWorkStatus;
	month: AttendanceWorkStatus;
	rows?: SupabaseWorkStatusInputs;
};

export function attendanceWorkStatusPairFrom(
	rows: SupabaseWorkStatusInputs,
	period: AttendanceWorkStatusRequest,
	month: AttendanceWorkStatusRequest
): AttendanceWorkStatusPair | undefined {
	const asked = { ...rows, now: new Date() };
	if (!rangeCovers(rows.coveredDays, [period, month], rows.timeZone, asked.now)) return undefined;
	return {
		period: attendanceWorkStatusFrom(asked, period),
		month: attendanceWorkStatusFrom(asked, month),
		rows
	};
}

export async function fetchAttendanceWorkStatusPair(
	period: AttendanceWorkStatusRequest,
	month: AttendanceWorkStatusRequest
): Promise<AttendanceWorkStatusPair> {
	const rows = await supabaseWorkStatusInputs([period, month]);
	return {
		period: attendanceWorkStatusFrom(rows, period),
		month: attendanceWorkStatusFrom(rows, month),
		rows
	};
}

export function fetchAttendanceWorkStatus(
	request: AttendanceWorkStatusRequest
): Promise<AttendanceWorkStatus> {
	return supabaseWorkStatus(request);
}

export async function updateAttendanceEvent(
	eventID: string,
	request: UpdateAttendanceEventRequest
): Promise<AttendanceWriteResult> {
	return updateAttendanceEvents([{ eventID, request }]);
}

export async function updateAttendanceEvents(
	updates: UpdateAttendanceEvent[]
): Promise<AttendanceWriteResult> {
	if (updates.length === 0) throw new Error('at least one attendance correction is required');
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

export function addAttendanceEvent(
	request: AddAttendanceEventRequest
): Promise<AttendanceWriteResult> {
	return addSupabaseAttendanceEvent(request);
}

export function removeAttendanceEvent(
	request: RemoveAttendanceEventRequest
): Promise<AttendanceWriteResult> {
	return removeSupabaseAttendanceEvent(request.eventID, request.reason);
}

export function updateAttendanceTeamViewVisibility(visible: boolean): Promise<void> {
	return setSupabaseTeamViewVisibility(visible);
}

export function toggleAttendanceOnServer(
	kind: AttendanceKind,
	locationID?: string,
	confirmedEarlyReturn = false
): Promise<AttendanceWriteResult | void> {
	return recordSupabaseAttendance(kind, locationID, confirmedEarlyReturn);
}
