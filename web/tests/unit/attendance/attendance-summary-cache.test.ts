import { afterAll, beforeAll, beforeEach, describe, expect, test } from 'bun:test';

import {
	clearCachedAttendanceSummaries,
	readCachedAttendanceSummary,
	writeCachedAttendanceSummary
} from '../../../src/routes/attendance/attendance-summary-cache';
import type { AttendanceSummary } from '../../../src/routes/attendance/attendance-context.svelte';

const summary = { month: '2026-07', timeZone: 'Asia/Seoul', events: [] } as unknown as AttendanceSummary;

function createMemorySessionStorage(): Storage {
	const entries = new Map<string, string>();
	return {
		get length() {
			return entries.size;
		},
		clear: () => entries.clear(),
		getItem: (key: string) => entries.get(key) ?? null,
		key: (index: number) => [...entries.keys()][index] ?? null,
		removeItem: (key: string) => {
			entries.delete(key);
		},
		setItem: (key: string, value: string) => {
			entries.set(key, value);
		}
	};
}

const originalWindowDescriptor = Object.getOwnPropertyDescriptor(globalThis, 'window');

beforeAll(() => {
	Object.defineProperty(globalThis, 'window', {
		value: { sessionStorage: createMemorySessionStorage() },
		configurable: true,
		writable: true
	});
});

afterAll(() => {
	if (originalWindowDescriptor) {
		Object.defineProperty(globalThis, 'window', originalWindowDescriptor);
		return;
	}
	delete (globalThis as { window?: Window }).window;
});

describe('attendance summary cache', () => {
	beforeEach(() => {
		window.sessionStorage.clear();
	});

	test('reads back a summary written for a month', () => {
		writeCachedAttendanceSummary('2026-07', summary);

		expect(readCachedAttendanceSummary('2026-07')).toEqual(summary);
		expect(readCachedAttendanceSummary('2026-06')).toBe(null);
	});

	test('also stores the resolved month when the request had none', () => {
		writeCachedAttendanceSummary('', summary);

		expect(readCachedAttendanceSummary('')).toEqual(summary);
		expect(readCachedAttendanceSummary('2026-07')).toEqual(summary);
	});

	test('ignores entries older than the freshness window', () => {
		writeCachedAttendanceSummary('2026-07', summary);
		const stored = JSON.parse(window.sessionStorage.getItem('attendance.summary.2026-07') ?? '{}');
		stored.cachedAt = Date.now() - 6 * 60 * 1000;
		window.sessionStorage.setItem('attendance.summary.2026-07', JSON.stringify(stored));

		expect(readCachedAttendanceSummary('2026-07')).toBe(null);
	});

	test('clears every cached month', () => {
		writeCachedAttendanceSummary('2026-07', summary);
		writeCachedAttendanceSummary('2026-06', { ...summary, month: '2026-06' } as AttendanceSummary);

		clearCachedAttendanceSummaries();

		expect(readCachedAttendanceSummary('2026-07')).toBe(null);
		expect(readCachedAttendanceSummary('2026-06')).toBe(null);
	});
});
