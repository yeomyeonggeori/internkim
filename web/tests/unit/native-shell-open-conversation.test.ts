import { describe, expect, test } from 'bun:test';
import { messageNotificationTag } from '$lib/native-shell/open-conversation';
import { tellingOf } from '../../../host/relay/arrived.ts';

describe('the tag the shell holds back while a conversation is open', () => {
	test('is the tag the relay puts on that conversation\'s message notifications', () => {
		const told = tellingOf(
			{
				conversationID: 'channel-a',
				messageID: 'message-1',
				authorExternalID: 'author-1',
				authorName: '이샘플',
				recipientExternalIDs: ['reader-1'],
				preview: '보냈어요'
			},
			'이샘플'
		);
		expect(messageNotificationTag('channel-a')).toBe(told.tag);
	});
});
