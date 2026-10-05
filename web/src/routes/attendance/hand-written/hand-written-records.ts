import { invokeTool } from '$lib/public-api-call';
import type { AttendanceKind } from '../attendance-context.svelte';

export type HandWrittenRecord = {
	eventID: string;
	person: string;
	personID?: string;
	personEmail?: string | null;
	changedByID?: string | null;
	location?: string | null;
	originalLocation?: string | null;
	previousRecorded?: boolean;
	teamName?: string;
	changedByName?: string | null;
	changedByEmail?: string | null;
	changedBySource?: string;
	kind: AttendanceKind;
	date: string;
	time: string;
	originalDate: string | null;
	originalTime: string | null;
	reason: string | null;
};

export type HandWrittenDayRange = { from: string; to: string };

export type AnsweredHandWrittenRecords = { attendance: HandWrittenRecord[]; totalCount: number };

export function currentAndPreviousMonth(today: string): HandWrittenDayRange {
	const [year, month] = today.split('-').map(Number);
	const firstDay = new Date(Date.UTC(year, month - 2, 1));
	const previousYear = String(firstDay.getUTCFullYear()).padStart(4, '0');
	const previousMonth = String(firstDay.getUTCMonth() + 1).padStart(2, '0');
	return { from: `${previousYear}-${previousMonth}-01`, to: today };
}

export async function fetchHandWrittenRecords(
 dayRange: HandWrittenDayRange, pageOffset = 0, selectedTeamKey = '', selectedChangedByID = ''
): Promise<AnsweredHandWrittenRecords> {
 return await invokeTool<AnsweredHandWrittenRecords>('attendance_list', {
  scope: 'all', from: dayRange.from, to: dayRange.to, handWrittenOnly: true,
  pageOffset, pageLimit: 24, ...(selectedTeamKey ? {selectedTeamKey} : {}), ...(selectedChangedByID ? {selectedChangedByID} : {})
 });
}

export function canUndoHandWrittenRecord(record: HandWrittenRecord): boolean {
 // Historical location-only edits and historical additions have the same old shape.
 // Refuse destructive inference when no previous state was actually observed.
 return !!(record.originalDate && record.originalTime) || record.changedBySource === 'observed';
}

export async function undoHandWrittenRecord(
	record: HandWrittenRecord,
	reason: string
): Promise<void> {
	if (!canUndoHandWrittenRecord(record)) throw new Error('Previous attendance state is unknown');
	if (record.originalDate && record.originalTime) {
		await invokeTool('attendance_update', {
			corrections: [
				{ eventHint: record.eventID, date: record.originalDate, time: record.originalTime, ...(record.previousRecorded && record.kind === 'clock_in' ? {location: record.originalLocation ?? undefined} : {}) }
			],
			reason, undoOnly: true
		});
		return;
	}
	await invokeTool('attendance_delete', { eventHint: record.eventID, reason, undoOnly: true });
}
