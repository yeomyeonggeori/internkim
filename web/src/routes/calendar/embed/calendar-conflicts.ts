import { isSupabaseConfigured } from '$lib/supabase';
export type CalendarConflict = {
	id: number;
	eventID: string;
	eventUID: string;
	field: string;
	localValue: string;
	remoteValue: string;
	detectedAt: string;
};

export async function fetchCalendarConflicts(): Promise<CalendarConflict[]> {
	if (isSupabaseConfigured()) return [];
	const response = await fetch('/calendar/api/conflicts', { credentials: 'include' });
	if (!response.ok) return [];
	const payload = (await response.json()) as { conflicts?: CalendarConflict[] };
	return payload.conflicts ?? [];
}

export async function dismissCalendarConflictOnServer(conflictID: number): Promise<void> {
	const response = await fetch(`/calendar/api/conflicts/${conflictID}/dismiss`, {
		method: 'POST',
		credentials: 'include'
	});
	if (!response.ok) throw new Error(await response.text());
}

export async function dismissCalendarConflictsOnServer(conflictIDs: number[]): Promise<void> {
	await Promise.all(conflictIDs.map((id) => dismissCalendarConflictOnServer(id)));
}
