import { describe, expect, test } from 'bun:test';
import { onlineMemberIDs } from '../../../src/lib/messenger/presence-roster';

describe('onlineMemberIDs', () => {
	test('counts a member once however many windows they have open', () => {
		const online = onlineMemberIDs({
			first: [{ memberID: 'member-a', presence_ref: '1' }],
			second: [
				{ memberID: 'member-a', presence_ref: '2' },
				{ memberID: 'member-b', presence_ref: '3' }
			]
		});
		expect([...online].sort()).toEqual(['member-a', 'member-b']);
	});

	test('leaves out a presence that names no member', () => {
		const online = onlineMemberIDs({
			first: [{ presence_ref: '1' }, { memberID: '', presence_ref: '2' }, { memberID: 7, presence_ref: '3' }]
		});
		expect(online.size).toBe(0);
	});

	test('nobody is online when nobody is present', () => {
		expect(onlineMemberIDs({}).size).toBe(0);
	});
});
