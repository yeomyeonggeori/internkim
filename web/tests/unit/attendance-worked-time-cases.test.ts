import { describe, expect, test } from 'bun:test';
import cases from '../fixtures/attendance-today-cases.json';
import { closedWorkedMinutesOn } from '../../../supabase/functions/_shared/attendance-worked-time.ts';

describe('the minutes a Live Activity starts from, read from the cases the card and the widget answer', () => {
	for (const day of cases.cases) {
		test(day.name, () => {
			const clocks = day.rows.map((row) => ({ kind: row.kind, occurred_at: row.occurredAt }));
			expect(closedWorkedMinutesOn(day.today, clocks, day.timeZone)).toBe(day.expected.workedMinutes);
		});
	}
});
