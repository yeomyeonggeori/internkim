import { afterAll, beforeAll, beforeEach, describe, expect, test } from 'bun:test';

import type { SupabaseWorkStatusInputs } from '$lib/attendance/supabase-work-status';
import type { SupabaseWorkPolicy } from '$lib/attendance/supabase-work-policy';
import {
	clearCachedWorkStatusRows,
	readCachedWorkStatusRows,
	writeCachedWorkStatusRows
} from '../../../src/routes/attendance/work-status/work-status-cache';

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

const scope = 'plane:company-a:sample@example.com';
const member = { id: 'member-1', name: '이샘플', email: 'sample@example.com', is_admin: false, joined_at: null };

const rows: SupabaseWorkStatusInputs = {
	timeZone: 'Asia/Seoul',
	members: [member],
	me: member,
	attendance: [{ id: 'event-1', member_id: 'member-1', kind: 'clock_in', occurred_at: '2026-07-01T00:00:00Z' }],
	leave: [],
	policiesByMember: new Map([['member-1', { memberID: 'member-1' } as SupabaseWorkPolicy]]),
	holidays: new Set(['2026-07-17']),
	coveredDays: ['2026-07-01'],
	now: new Date('2026-07-01T03:00:00Z')
};

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

describe('work status row cache', () => {
	beforeEach(() => {
		window.localStorage.clear();
	});

	test('reads back the rows with their maps, sets and dates restored', () => {
		writeCachedWorkStatusRows(rows, scope);

		expect(readCachedWorkStatusRows(scope)).toEqual(rows);
	});

	test('keeps the rows separate across account and company scopes', () => {
		writeCachedWorkStatusRows(rows, scope);

		expect(readCachedWorkStatusRows('plane:company-a:other@example.com')).toBeUndefined();
		expect(readCachedWorkStatusRows('plane:company-b:sample@example.com')).toBeUndefined();
	});

	test('answers with nothing when no rows were kept or they are unreadable', () => {
		expect(readCachedWorkStatusRows(scope)).toBeUndefined();

		window.localStorage.setItem(`attendance.workStatusRows.${encodeURIComponent(scope)}`, '{"timeZone":"Asia/Seoul"}');
		expect(readCachedWorkStatusRows(scope)).toBeUndefined();
	});

	test('forgets the rows when cleared', () => {
		writeCachedWorkStatusRows(rows, scope);

		clearCachedWorkStatusRows();

		expect(readCachedWorkStatusRows(scope)).toBeUndefined();
	});
});
