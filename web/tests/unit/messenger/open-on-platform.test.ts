import { describe, expect, test } from 'bun:test';
import { asChannel } from '../../../src/lib/messenger/messenger-api';

describe('the link that opens a conversation in its own messenger', () => {
	test('survives the trip from the adapter to the sidebar', () => {
		const channel = asChannel(
			{
				id: 'channel-1',
				name: 'Announcements',
				kind: 'group',
				webURL: 'https://mattermost.test/internkim/channels/announcements'
			},
			0
		);

		expect(channel.webURL).toBe('https://mattermost.test/internkim/channels/announcements');
	});

	test('stays absent when the platform has no web client to open', () => {
		const channel = asChannel({ id: 'channel-2', name: 'Announcements', kind: 'group' }, 1);

		expect(channel.webURL).toBeUndefined();
	});
});
