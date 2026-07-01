import { describe, expect, test } from 'bun:test';
import { readPersistedAttendanceFilters } from '../../../src/routes/attendance/attendance-storage';

describe('readPersistedAttendanceFilters', () => {
	test('does not restore a previously selected month for first page load', () => {
		const originalWindow = globalThis.window;
		const localStorage = {
			getItem: (key: string) => key === 'attendance.filters' ? JSON.stringify({ selectedMonth: '2026-06', chartMode: 'month' }) : null,
		};

		try {
			Object.defineProperty(globalThis, 'window', {
				value: { localStorage },
				configurable: true,
			});

			expect(readPersistedAttendanceFilters()).toEqual({ chartMode: 'month' });
		} finally {
			Object.defineProperty(globalThis, 'window', {
				value: originalWindow,
				configurable: true,
			});
		}
	});
});
