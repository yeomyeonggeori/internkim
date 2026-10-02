import { afterAll, beforeAll, beforeEach, describe, expect, test } from 'bun:test';

import {
	clearCachedAttendanceSummaries,
	readCachedAttendanceSummary,
	writeCachedAttendanceSummary
} from '../../../src/routes/attendance/attendance-summary-cache';
import type { AttendanceSummary } from '../../../src/routes/attendance/attendance-context.svelte';

const summary = { month: '2026-07', timeZone: 'Asia/Seoul', events: [] } as unknown as AttendanceSummary;

function createMemoryStorage(): Storage {
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
		value: { localStorage: createMemoryStorage() },
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
		window.localStorage.clear();
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

	test('ignores an entry that is not a summary', () => {
		window.localStorage.setItem('attendance.summary.2026-07', '{"cachedAt":1}');
		window.localStorage.setItem('attendance.summary.2026-06', 'not json');

		expect(readCachedAttendanceSummary('2026-07')).toBe(null);
		expect(readCachedAttendanceSummary('2026-06')).toBe(null);
	});

	test('keeps the same month separate across account and company scopes', () => {
		writeCachedAttendanceSummary('2026-07', summary, 'plane:company-a:member-a');
		expect(readCachedAttendanceSummary('2026-07', 'plane:company-a:member-a')).toEqual(summary);
		expect(readCachedAttendanceSummary('2026-07', 'plane:company-a:member-b')).toBe(null);
		expect(readCachedAttendanceSummary('2026-07', 'plane:company-b:member-a')).toBe(null);
	});

	test('clears every cached month', () => {
		writeCachedAttendanceSummary('2026-07', summary);
		writeCachedAttendanceSummary('2026-06', { ...summary, month: '2026-06' } as AttendanceSummary);

		clearCachedAttendanceSummaries();

		expect(readCachedAttendanceSummary('2026-07')).toBe(null);
		expect(readCachedAttendanceSummary('2026-06')).toBe(null);
	});
});
