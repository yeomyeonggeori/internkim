import { describe, expect, test } from 'bun:test';
import { asChannel } from '../../../src/lib/messenger/messenger-api';

describe('a direct conversation arriving from the company app', () => {
	test('carries the people in it, so the sidebar can name them', () => {
		const channel = asChannel(
			{
				id: 'channel-1',
				name: 'person-a__person-b',
				kind: 'dm',
				participantExternalIDs: ['person-a', 'person-b']
			},
			0
		);

		expect(channel.participants).toEqual([{ externalID: 'person-a' }, { externalID: 'person-b' }]);
		expect(channel.isDirect).toBe(true);
	});

	test('a conversation naming nobody leaves the list empty rather than inventing a person', () => {
		const channel = asChannel({ id: 'channel-2', name: 'Announcements', kind: 'group' }, 1);

		expect(channel.participants).toEqual([]);
		expect(channel.isDirect).toBe(false);
	});
});
