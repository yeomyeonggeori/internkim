import { describe, expect, test } from 'bun:test';
import { summarizeSupabaseLeave } from '../../../src/lib/attendance/supabase-leave-summary';

describe('summarizeSupabaseLeave', () => {
	test('counts current-year approved and requested deducted leave', () => {
		const balance = summarizeSupabaseLeave(
			[
				{ days: 2, status: 'approved', isDeducted: true, localStartDate: '2026-08-04' },
				{ days: 0.5, status: 'requested', isDeducted: true, localStartDate: '2026-09-01' }
			],
			2026,
			8
		);

		expect(balance).toEqual({
			trackingMode: 'managed',
			summary: {
				usedMilliDays: 2000,
				reservedMilliDays: 500,
				availableMilliDays: 7500
			}
		});
	});

	test('excludes rejected, non-deducted, and other-year leave', () => {
		const balance = summarizeSupabaseLeave(
			[
				{ days: 1, status: 'rejected', isDeducted: true, localStartDate: '2026-08-04' },
				{ days: 1, status: 'approved', isDeducted: false, localStartDate: '2026-08-05' },
				{ days: 1, status: 'approved', isDeducted: true, localStartDate: '2025-12-31' }
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

	test('excludes cancelled deducted leave from used and reserved balances', () => {
		const balance = summarizeSupabaseLeave(
			[
				{ days: 1, status: 'approved', isDeducted: true, localStartDate: '2026-08-04', cancelled: true },
				{ days: 0.5, status: 'requested', isDeducted: true, localStartDate: '2026-08-05', cancelled: true }
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
				{ days: 1, status: 'approved', isDeducted: true, localStartDate: '2026-08-04' },
				{ days: 0.25, status: 'requested', isDeducted: true, localStartDate: '2026-08-05' }
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
