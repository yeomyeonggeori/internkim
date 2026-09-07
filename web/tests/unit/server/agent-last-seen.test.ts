import { describe, expect, test } from 'bun:test';
import { isLastSeenStale } from '$lib/server/control-plane';

const now = Date.parse('2026-09-08T09:00:00.000Z');

describe('isLastSeenStale', () => {
	test('a machine never seen, or seen a minute ago, is written down', () => {
		expect(isLastSeenStale(null, now)).toBe(true);
		expect(isLastSeenStale('2026-09-08T08:59:00.000Z', now)).toBe(true);
		expect(isLastSeenStale('not a time', now)).toBe(true);
	});

	test('a machine seen seconds ago is not written down again', () => {
		expect(isLastSeenStale('2026-09-08T08:59:30.000Z', now)).toBe(false);
		expect(isLastSeenStale('2026-09-08T09:00:00.000Z', now)).toBe(false);
	});
});
