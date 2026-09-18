import { describe, expect, test } from 'bun:test';
import cases from '../fixtures/attendance-today-cases.json';
import type { AttendanceEvent } from '../../src/routes/attendance/attendance-context.svelte';
import { computeDayEvents, elapsedWorkedMinutes } from '../../src/routes/attendance/shared/attendance-day-events';
import { segmentWidthPercent } from '../../src/routes/attendance/shared/day-timeline';

type CaseRow = { kind: string; date: string; time: string; occurredAt: string; location: string | null };

function eventOf(row: CaseRow, index: number, timeZone: string): AttendanceEvent {
	return {
		id: `event-${index}`,
		email: 'member1@example.com',
		displayName: '이샘플',
		kind: row.kind as AttendanceEvent['kind'],
		occurredAt: row.occurredAt,
		localDate: row.date,
		localTime: row.time,
		timeZoneAtEvent: timeZone,
		source: 'test',
		resultPostID: '',
		locationName: row.location ?? undefined
	};
}

describe('the day the attendance card draws, which the iOS widget reads from the same cases', () => {
	for (const day of cases.cases) {
		test(day.name, () => {
			const now = new Date(day.now);
			const events = day.rows.map((row, index) => eventOf(row, index, day.timeZone));
			const computed = computeDayEvents(day.today, events, { currentDate: day.today, now });

			expect(computed.workedMinutes).toBe(day.expected.workedMinutes);
			expect(elapsedWorkedMinutes(computed, now)).toBe(day.expected.elapsedMinutes);
			expect(computed.inProgress).toBe(day.expected.isWorking);
			expect(computed.activeSegment?.locationName ?? null).toBe(day.expected.location);
			expect(computed.clockIn?.localTime ?? null).toBe(day.expected.clockInTime);
			expect(computed.clockOut?.localTime ?? null).toBe(day.expected.clockOutTime);
			expect(
				computed.segments.map((segment) => ({
					location: segment.locationName ?? null,
					widthPercent: Number(segmentWidthPercent(segment, day.currentTime).toFixed(4))
				}))
			).toEqual(day.expected.bars);
		});
	}
});
