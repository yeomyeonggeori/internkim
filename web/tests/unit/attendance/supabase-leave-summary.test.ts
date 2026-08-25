import { describe, expect, test } from 'bun:test';
import { summarizeSupabaseLeave } from '../../../src/lib/attendance/supabase-leave-summary';

describe('summarizeSupabaseLeave', () => {
	test('counts current-year approved and requested deducted leave', () => {
		const balance = summarizeSupabaseLeave(
			[
				{
					days: 2,
					status: 'approved',
					isDeducted: true,
					localStartDate: '2026-08-04',
					localEndDate: '2026-08-05'
				},
				{
					days: 0.5,
					status: 'requested',
					isDeducted: true,
					localStartDate: '2026-09-01',
					localEndDate: '2026-09-01'
				}
			],
			2026,
			8
		);

		expect(balance).toEqual({
			trackingMode: 'managed',
			summary: {
				usedMilliDays: 2000,
				reservedMilliDays: 500,
				availableMilliDays: 8000
			}
		});
	});

	test('leaves what is still pending out of the remaining balance', () => {
		const balance = summarizeSupabaseLeave(
			[
				{
					days: 3,
					status: 'requested',
					isDeducted: true,
					localStartDate: '2026-09-01',
					localEndDate: '2026-09-03'
				}
			],
			2026,
			8
		);

		expect(balance.summary).toEqual({
			usedMilliDays: 0,
			reservedMilliDays: 3000,
			availableMilliDays: 8000
		});
	});

	test('splits a leave that spans new year across both years', () => {
		const crossing = {
			days: 4,
			status: 'approved',
			isDeducted: true,
			localStartDate: '2026-12-30',
			localEndDate: '2027-01-02'
		} as const;

		expect(summarizeSupabaseLeave([crossing], 2026, 8).summary.usedMilliDays).toBe(2000);
		expect(summarizeSupabaseLeave([crossing], 2027, 8).summary.usedMilliDays).toBe(2000);
	});

	test('excludes rejected, non-deducted, and other-year leave', () => {
		const balance = summarizeSupabaseLeave(
			[
				{
					days: 1,
					status: 'rejected',
					isDeducted: true,
					localStartDate: '2026-08-04',
					localEndDate: '2026-08-04'
				},
				{
					days: 1,
					status: 'approved',
					isDeducted: false,
					localStartDate: '2026-08-05',
					localEndDate: '2026-08-05'
				},
				{
					days: 1,
					status: 'approved',
					isDeducted: true,
					localStartDate: '2025-12-31',
					localEndDate: '2025-12-31'
				}
			],
			2026,
			8
		);

		expect(balance.summary).toEqual({
			usedMilliDays: 0,
			reservedMilliDays: 0,
			availableMilliDays: 8000
		});
	});

	test('uses unlimited mode when no entitlement is configured', () => {
		const balance = summarizeSupabaseLeave(
			[
				{
					days: 1,
					status: 'approved',
					isDeducted: true,
					localStartDate: '2026-08-04',
					localEndDate: '2026-08-04'
				},
				{
					days: 0.25,
					status: 'requested',
					isDeducted: true,
					localStartDate: '2026-08-05',
					localEndDate: '2026-08-05'
				}
			],
			2026,
			null
		);

		expect(balance).toEqual({
			trackingMode: 'unlimited',
			summary: {
				usedMilliDays: 1000,
				reservedMilliDays: 250,
				availableMilliDays: 0
			}
		});
	});
});
