import { describe, expect, test } from 'bun:test';
import { coveredDaysOf, rangeCovers } from '../../../src/lib/attendance/supabase-work-status';

const timeZone = 'Asia/Seoul';
const now = new Date('2026-08-20T00:00:00Z');

const august = { period: 'month', anchor: '2026-08-05' } as const;
const dayInAugust = { period: 'day', anchor: '2026-08-20' } as const;
const weekInAugust = { period: 'week', anchor: '2026-08-19' } as const;
const weekCrossingIntoSeptember = { period: 'week', anchor: '2026-08-31' } as const;
const september = { period: 'month', anchor: '2026-09-05' } as const;

const augustAndItsDay = coveredDaysOf([august, dayInAugust], timeZone, now);

describe('rangeCovers', () => {
	test('a period already inside the fetched range needs no fetch', () => {
		expect(rangeCovers(augustAndItsDay, [august, dayInAugust], timeZone, now)).toBe(true);
	});

	test('a different day of the same month needs no fetch', () => {
		expect(
			rangeCovers(augustAndItsDay, [august, { period: 'day', anchor: '2026-08-03' }], timeZone, now)
		).toBe(true);
	});

	test('a week wholly inside the month needs no fetch', () => {
		expect(rangeCovers(augustAndItsDay, [august, weekInAugust], timeZone, now)).toBe(true);
	});

	test('a week running past the end of the month needs one', () => {
		expect(rangeCovers(augustAndItsDay, [august, weekCrossingIntoSeptember], timeZone, now)).toBe(
			false
		);
	});

	test('moving to the next month needs one', () => {
		expect(rangeCovers(augustAndItsDay, [september, dayInAugust], timeZone, now)).toBe(false);
	});

	test('an empty range never covers anything', () => {
		expect(rangeCovers([], [august], timeZone, now)).toBe(false);
	});

	test('a range fetched for a crossing week covers the plain month inside it', () => {
		const wider = coveredDaysOf([august, weekCrossingIntoSeptember], timeZone, now);
		expect(rangeCovers(wider, [august, dayInAugust], timeZone, now)).toBe(true);
	});
});
