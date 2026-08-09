import { describe, expect, test } from 'bun:test';
import { supabaseLeavePreview } from '../../../src/lib/attendance/supabase-leave';

describe('supabaseLeavePreview', () => {
	test('counts one occurrence a day across the range', async () => {
		const preview = await supabaseLeavePreview({
			leaveTypeID: 'leave',
			unit: 'fullDay',
			startDate: '2026-08-03',
			endDate: '2026-08-05'
		});
		expect(preview.occurrences.map((occurrence) => occurrence.date)).toEqual([
			'2026-08-03',
			'2026-08-04',
			'2026-08-05'
		]);
		expect(preview.totalDeductionMilliDays).toBe(3000);
	});

	test('a single half day is half a day', async () => {
		const preview = await supabaseLeavePreview({
			leaveTypeID: 'leave',
			unit: 'halfDay',
			startDate: '2026-08-03'
		});
		expect(preview.occurrences).toHaveLength(1);
		expect(preview.totalDeductionMilliDays).toBe(500);
	});

	test('crosses a month boundary', async () => {
		const preview = await supabaseLeavePreview({
			leaveTypeID: 'leave',
			unit: 'fullDay',
			startDate: '2026-07-30',
			endDate: '2026-08-02'
		});
		expect(preview.occurrences).toHaveLength(4);
	});
});
