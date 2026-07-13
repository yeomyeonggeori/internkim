import { describe, expect, test } from 'bun:test';
import { formatDisplayTime } from '../../../src/lib/components/time-text';

describe('formatDisplayTime', () => {
	test('displays local times as HH:mm', () => {
		expect(formatDisplayTime('08:30')).toBe('08:30');
		expect(formatDisplayTime('08:30:45')).toBe('08:30');
		expect(formatDisplayTime('24:00:00')).toBe('24:00');
	});

	test('preserves non-time display values', () => {
		expect(formatDisplayTime('ongoing')).toBe('ongoing');
		expect(formatDisplayTime('8:30:00')).toBe('8:30:00');
		expect(formatDisplayTime('25:00:00')).toBe('25:00:00');
	});
});
