import { describe, expect, test } from 'bun:test';
import {
	leaveDisplayRange,
	leavePreviewPeriod,
	leaveTimestampRange
} from '../../../src/lib/attendance/supabase-leave-range';

describe('leavePreviewPeriod', () => {
	test('uses the configured morning and afternoon half-day ranges', () => {
		expect(
			leavePreviewPeriod({
				leaveTypeID: 'leave',
				unit: 'halfDay',
				startDate: '2026-08-03',
				partialPeriod: 'morning'
			})
		).toEqual({ startTime: '09:00', endTime: '14:00', deductionMilliDays: 500 });
		expect(
			leavePreviewPeriod({
				leaveTypeID: 'leave',
				unit: 'halfDay',
				startDate: '2026-08-03',
				partialPeriod: 'afternoon'
			})
		).toEqual({ startTime: '14:00', endTime: '18:00', deductionMilliDays: 500 });
	});

	test('adds custom work time across lunch', () => {
		expect(
			leavePreviewPeriod({
				leaveTypeID: 'leave',
				unit: 'quarterDay',
				startDate: '2026-08-03',
				partialPeriod: 'custom',
				startTime: '11:30'
			})
		).toEqual({ startTime: '11:30', endTime: '14:30', deductionMilliDays: 250 });
	});

	test('does not count lunch as work when custom leave starts during lunch', () => {
		expect(
			leavePreviewPeriod({
				leaveTypeID: 'leave',
				unit: 'quarterDay',
				startDate: '2026-08-03',
				partialPeriod: 'custom',
				startTime: '12:00'
			})
		).toEqual({ startTime: '12:00', endTime: '15:00', deductionMilliDays: 250 });
		expect(
			leavePreviewPeriod({
				leaveTypeID: 'leave',
				unit: 'quarterDay',
				startDate: '2026-08-03',
				partialPeriod: 'custom',
				startTime: '12:30'
			})
		).toEqual({ startTime: '12:30', endTime: '15:00', deductionMilliDays: 250 });
	});
});

describe('leaveTimestampRange', () => {
	test('stores a full-day range as company-local exclusive midnights', () => {
		expect(
			leaveTimestampRange(
				{
					leaveTypeID: 'leave',
					unit: 'fullDay',
					startDate: '2026-08-03',
					endDate: '2026-08-05'
				},
				'Asia/Seoul'
			)
		).toEqual({
			startsAt: '2026-08-02T15:00:00.000Z',
			endsAt: '2026-08-05T15:00:00.000Z'
		});
	});

	test('stores partial leave at its submitted company-local times', () => {
		expect(
			leaveTimestampRange(
				{
					leaveTypeID: 'leave',
					unit: 'halfDay',
					startDate: '2026-08-03',
					partialPeriod: 'afternoon'
				},
				'Asia/Seoul'
			)
		).toEqual({
			startsAt: '2026-08-03T05:00:00.000Z',
			endsAt: '2026-08-03T09:00:00.000Z'
		});
	});
});

describe('leaveDisplayRange', () => {
	test('reconstructs full-day dates from company-local boundaries', () => {
		expect(
			leaveDisplayRange(
				'2026-08-02T15:00:00.000Z',
				'2026-08-05T15:00:00.000Z',
				1,
				'Asia/Seoul'
			)
		).toEqual({ startDate: '2026-08-03', endDate: '2026-08-05' });
	});

	test('reconstructs a standard half-day period and its times', () => {
		expect(
			leaveDisplayRange(
				'2026-08-03T00:00:00.000Z',
				'2026-08-03T05:00:00.000Z',
				0.5,
				'Asia/Seoul'
			)
		).toEqual({
			startDate: '2026-08-03',
			partialPeriod: 'morning',
			startTime: '09:00',
			endTime: '14:00'
		});
	});
});
