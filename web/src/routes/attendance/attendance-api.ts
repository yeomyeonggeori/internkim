import type {
	AttendanceAbsence,
	AttendanceAbsenceKind,
	AttendanceKind,
	AttendanceSummary
} from './attendance-context.svelte';

export type AttendanceSummaryRequest = {
	month: string;
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

export async function fetchAttendanceSummary(request: AttendanceSummaryRequest): Promise<AttendanceSummary> {
	const path = attendanceSummaryPath(request);
	const response = await fetch(path, { credentials: 'include', cache: 'no-store' });
	if (!response.ok) throw new Error(await response.text());
	return (await response.json()) as AttendanceSummary;
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
	const response = await fetch(`/attendance/api/events/${encodeURIComponent(eventID)}`, {
		method: 'PATCH',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(request)
	});
	if (!response.ok) throw new Error(await response.text());
}

export async function updateAttendanceTeamViewVisibility(visible: boolean): Promise<void> {
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
