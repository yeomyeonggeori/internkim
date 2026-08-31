import { describe, expect, test } from 'bun:test';
import {
	companyDateOf,
	companyDateTimeOf,
	companyInstantOf,
	companyTimeOf,
	isValidTimeZone
} from '../../src/lib/company-time';

describe('companyDateOf', () => {
	test('reads the day the company was on, not the UTC day', () => {
		expect(companyDateOf(new Date('2026-08-31T23:30:00.000Z'), 'Asia/Seoul')).toBe('2026-09-01');
		expect(companyDateOf(new Date('2026-08-31T23:30:00.000Z'), 'UTC')).toBe('2026-08-31');
	});
});

describe('companyTimeOf', () => {
	test('writes midnight as 00:00 rather than 24:00', () => {
		expect(companyTimeOf(new Date('2026-08-31T15:00:00.000Z'), 'Asia/Seoul')).toBe('00:00');
	});

	test('keeps the clock at 24 hours', () => {
		expect(companyTimeOf(new Date('2026-09-01T10:05:00.000Z'), 'Asia/Seoul')).toBe('19:05');
	});
});

describe('companyDateTimeOf', () => {
	test('reports every field as a number in the company zone', () => {
		expect(companyDateTimeOf(new Date('2026-09-01T04:05:06.000Z'), 'Asia/Seoul')).toEqual({
			year: 2026,
			month: 9,
			day: 1,
			hour: 13,
			minute: 5,
			second: 6
		});
	});
});

describe('companyInstantOf', () => {
	test('resolves a wall clock time to the instant it names', () => {
		expect(companyInstantOf('2026-09-01', '00:00', 'Asia/Seoul')).toBe('2026-08-31T15:00:00.000Z');
	});

	test('follows the offset across a daylight saving change', () => {
		expect(companyInstantOf('2026-03-07', '12:00', 'America/New_York')).toBe(
			'2026-03-07T17:00:00.000Z'
		);
		expect(companyInstantOf('2026-03-09', '12:00', 'America/New_York')).toBe(
			'2026-03-09T16:00:00.000Z'
		);
	});

	test('refuses a wall clock time the company never passed through', () => {
		expect(companyInstantOf('2026-03-08', '02:30', 'America/New_York')).toBeUndefined();
	});

	test('refuses a day that does not exist', () => {
		expect(companyInstantOf('not-a-day', '00:00', 'Asia/Seoul')).toBeUndefined();
	});
});

describe('isValidTimeZone', () => {
	test('separates a real zone from a made up one', () => {
		expect(isValidTimeZone('Asia/Seoul')).toBe(true);
		expect(isValidTimeZone('Asia/Atlantis')).toBe(false);
	});
});
