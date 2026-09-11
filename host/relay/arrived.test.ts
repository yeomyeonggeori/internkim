import { describe, expect, test } from 'bun:test';
import { notifyRequestOf, readArrivedMessage, tellingOf } from './arrived';

const posted = {
	conversationID: 'channel-1',
	messageID: 'post-7',
	authorExternalID: 'U-author',
	authorName: '이샘플',
	recipientExternalIDs: ['U-first', 'U-second'],
	preview: '오늘 회의 30분 미뤄도 될까요'
};

describe('readArrivedMessage', () => {
	test('what the messenger reported is what gets notified', () => {
		expect(readArrivedMessage(posted)).toEqual(posted);
	});

	test('nobody is told about their own message', () => {
		const arrived = readArrivedMessage({ ...posted, recipientExternalIDs: ['U-author', 'U-first'] });

		expect(arrived?.recipientExternalIDs).toEqual(['U-first']);
	});

	test('a name repeated in the channel list is one person', () => {
		const arrived = readArrivedMessage({ ...posted, recipientExternalIDs: ['U-first', 'U-first', ' U-first '] });

		expect(arrived?.recipientExternalIDs).toEqual(['U-first']);
	});

	test('an older messenger that reports no name still reports an arrival', () => {
		const { authorName: _absent, ...withoutTheName } = posted;

		expect(readArrivedMessage(withoutTheName)?.authorName).toBe('');
	});

	test('a report with no author or no message is not one', () => {
		expect(readArrivedMessage({ ...posted, authorExternalID: '' })).toBeNull();
		expect(readArrivedMessage({ ...posted, messageID: '' })).toBeNull();
		expect(readArrivedMessage(null)).toBeNull();
		expect(readArrivedMessage('a message')).toBeNull();
	});

	test('a long message is cut before it travels, because a push has a ceiling', () => {
		const arrived = readArrivedMessage({ ...posted, preview: '가'.repeat(500) });

		expect(arrived?.preview.length).toBe(140);
	});

	test('a recipient list of the wrong shape leaves nobody to tell', () => {
		expect(readArrivedMessage({ ...posted, recipientExternalIDs: 'U-first' })?.recipientExternalIDs).toEqual([]);
		expect(readArrivedMessage({ ...posted, recipientExternalIDs: [7, null] })?.recipientExternalIDs).toEqual([]);
	});
});

describe('tellingOf', () => {
	test('the notification says who spoke and what they said', () => {
		const arrived = readArrivedMessage(posted);

		expect(tellingOf(arrived!, '이샘플')).toEqual({
			title: '이샘플',
			body: '오늘 회의 30분 미뤄도 될까요',
			openPath: '/messenger/',
			tag: 'message:channel-1'
		});
	});

	test('messages in one conversation replace each other rather than stacking', () => {
		const first = tellingOf(readArrivedMessage(posted)!, '이샘플');
		const second = tellingOf(readArrivedMessage({ ...posted, messageID: 'post-8' })!, '박예시');

		expect(first.tag).toBe(second.tag);
	});

	test('a sender we cannot name still says something', () => {
		expect(tellingOf(readArrivedMessage(posted)!, '').title).toBe('internkim');
	});
});

describe('notifyRequestOf', () => {
	test('the project is told who wrote the message, so it can show their picture', () => {
		expect(notifyRequestOf(readArrivedMessage(posted)!, '이샘플', 'buzz')).toEqual({
			platform: 'buzz',
			externalIDs: ['U-first', 'U-second'],
			senderExternalID: 'U-author',
			category: 'message',
			conversationID: 'channel-1',
			title: '이샘플',
			body: '오늘 회의 30분 미뤄도 될까요',
			openPath: '/messenger/',
			tag: 'message:channel-1'
		});
	});
});
