import { describe, expect, test } from 'bun:test';
import { asChannel } from '../../../src/lib/messenger/messenger-api';

describe('whether a room is private survives the trip to the sidebar', () => {
	test('a private room stays private', () => {
		expect(asChannel({ id: 'c1', name: 'HR', kind: 'group', isPrivate: true }, 0).isPrivate).toBe(true);
	});

	test('an open room stays open', () => {
		expect(asChannel({ id: 'c2', name: '광장', kind: 'group', isPrivate: false }, 1).isPrivate).toBe(false);
	});

	test('a platform that says nothing draws the open icon rather than the lock', () => {
		expect(asChannel({ id: 'c3', name: '광장', kind: 'group' }, 2).isPrivate).toBe(false);
	});
});
