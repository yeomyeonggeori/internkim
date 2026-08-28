import { describe, expect, test } from 'bun:test';
import { coveredDaysOf } from '../../../src/lib/attendance/supabase-work-status';

const seoul = 'Asia/Seoul';
const now = new Date('2026-08-20T00:00:00Z');

describe('coveredDaysOf', () => {
	test('a single day period covers that day alone', () => {
		expect(coveredDaysOf([{ period: 'day', anchor: '2026-08-20' }], seoul, now)).toEqual([
			'2026-08-20'
		]);
	});

	test('a month period covers every day of the month', () => {
		const covered = coveredDaysOf([{ period: 'month', anchor: '2026-08-05' }], seoul, now);
		expect(covered[0]).toBe('2026-08-01');
		expect(covered.at(-1)).toBe('2026-08-31');
		expect(covered.length).toBe(31);
	});

	test('a month and a day inside it cover the month alone', () => {
		const covered = coveredDaysOf(
			[
				{ period: 'month', anchor: '2026-08-05' },
				{ period: 'day', anchor: '2026-08-20' }
			],
			seoul,
			now
		);
		expect(covered[0]).toBe('2026-08-01');
		expect(covered.at(-1)).toBe('2026-08-31');
	});

	test('a week that runs past the end of the month widens the range into the next one', () => {
		const covered = coveredDaysOf(
			[
				{ period: 'month', anchor: '2026-08-05' },
				{ period: 'week', anchor: '2026-08-31' }
			],
			seoul,
			now
		);
		expect(covered[0]).toBe('2026-08-01');
		expect(covered.at(-1)).toBe('2026-09-06');
		expect(covered).toContain('2026-09-01');
	});

	test('a week that starts before the month widens the range backwards', () => {
		const covered = coveredDaysOf(
			[
				{ period: 'month', anchor: '2026-09-15' },
				{ period: 'week', anchor: '2026-09-01' }
			],
			seoul,
			now
		);
		expect(covered[0]).toBe('2026-08-31');
		expect(covered.at(-1)).toBe('2026-09-30');
	});

	test('the covered range is contiguous and sorted', () => {
		const covered = coveredDaysOf(
			[
				{ period: 'day', anchor: '2026-08-03' },
				{ period: 'day', anchor: '2026-08-09' }
			],
			seoul,
			now
		);
		expect(covered).toEqual([
			'2026-08-03',
			'2026-08-04',
			'2026-08-05',
			'2026-08-06',
			'2026-08-07',
			'2026-08-08',
			'2026-08-09'
		]);
	});

	test('asking for no period at all is refused rather than answered with nothing', () => {
		expect(() => coveredDaysOf([], seoul, now)).toThrow();
	});
});
