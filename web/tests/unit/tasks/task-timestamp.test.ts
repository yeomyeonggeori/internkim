import { describe, expect, test } from 'bun:test';
import { formatTaskTimestamp } from '../../../src/routes/runs/runs-view';

describe('formatTaskTimestamp', () => {
	const now = new Date(2026, 8, 24, 15, 0);

	test('shows only the time for today', () => {
		expect(formatTaskTimestamp(new Date(2026, 8, 24, 9, 5).toISOString(), now, 'ko-KR')).toBe('오전 9:05');
	});

	test('drops the year within the current year', () => {
		expect(formatTaskTimestamp(new Date(2026, 5, 17, 9, 24).toISOString(), now, 'ko-KR')).toBe('6월 17일 오전 9:24');
	});

	test('shows the date with its year for an earlier year', () => {
		expect(formatTaskTimestamp(new Date(2025, 11, 31, 23, 0).toISOString(), now, 'ko-KR')).toBe('2025년 12월 31일');
	});

	test('returns an unparseable value unchanged', () => {
		expect(formatTaskTimestamp('not a date', now)).toBe('not a date');
	});
});
