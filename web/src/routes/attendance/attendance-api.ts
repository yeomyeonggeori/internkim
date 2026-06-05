import type { AttendanceEvent, AttendanceKind, AttendanceSummary } from './attendance-context.svelte';

export type AttendanceSummaryRequest = {
	month: string;
	selectedEmail: string;
};

export async function fetchAttendanceSummary(request: AttendanceSummaryRequest): Promise<AttendanceSummary> {
	const path = attendanceSummaryPath(request);
	const response = await fetch(path, { credentials: 'include' });
	if (!response.ok) throw new Error(await response.text());
	return (await response.json()) as AttendanceSummary;
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

export async function updateAttendanceEventLocation(eventID: string, locationID: string): Promise<AttendanceEvent> {
	const response = await fetch(`/attendance/api/events/${encodeURIComponent(eventID)}/location`, {
		method: 'PATCH',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ locationID })
	});
	if (!response.ok) throw new Error(await response.text());
	return (await response.json()) as AttendanceEvent;
}

export async function toggleAttendanceOnServer(kind?: AttendanceKind, locationID?: string): Promise<void> {
	const response = await fetch('/attendance/api/clock', {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ kind: kind ?? '', locationID: locationID ?? '' })
	});
	if (!response.ok) throw new Error(await response.text());
}

function attendanceSummaryPath(request: AttendanceSummaryRequest): string {
	const query = new URLSearchParams();
	if (request.month) query.set('month', request.month);
	if (request.selectedEmail) query.set('email', request.selectedEmail);
	const queryString = query.toString();
	return queryString ? `/attendance/api/summary?${queryString}` : '/attendance/api/summary';
}
