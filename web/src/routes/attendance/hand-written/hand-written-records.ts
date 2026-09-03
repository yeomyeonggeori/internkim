import { invokeTool } from '$lib/public-api-call';
import type { AttendanceKind } from '../attendance-context.svelte';

export type HandWrittenRecord = {
	eventID: string;
	person: string;
	kind: AttendanceKind;
	date: string;
	time: string;
	originalDate: string | null;
	originalTime: string | null;
	reason: string | null;
};

export type HandWrittenDayRange = { from: string; to: string };

type AnsweredHandWrittenRecords = { attendance: HandWrittenRecord[] };

export function currentAndPreviousMonth(today: string): HandWrittenDayRange {
	const [year, month] = today.split('-').map(Number);
	const firstDay = new Date(Date.UTC(year, month - 2, 1));
	const previousYear = String(firstDay.getUTCFullYear()).padStart(4, '0');
	const previousMonth = String(firstDay.getUTCMonth() + 1).padStart(2, '0');
	return { from: `${previousYear}-${previousMonth}-01`, to: today };
}

export async function fetchHandWrittenRecords(
	dayRange: HandWrittenDayRange
): Promise<HandWrittenRecord[]> {
	const answered = await invokeTool<AnsweredHandWrittenRecords>('attendance_list', {
		scope: 'all',
		from: dayRange.from,
		to: dayRange.to,
		handWrittenOnly: true
	});
	return answered.attendance;
}

export async function undoHandWrittenRecord(
	record: HandWrittenRecord,
	reason: string
): Promise<void> {
	if (record.originalDate && record.originalTime) {
		await invokeTool('attendance_update', {
			corrections: [
				{ eventHint: record.eventID, date: record.originalDate, time: record.originalTime }
			],
			reason
		});
		return;
	}
	await invokeTool('attendance_delete', { eventHint: record.eventID, reason });
}
