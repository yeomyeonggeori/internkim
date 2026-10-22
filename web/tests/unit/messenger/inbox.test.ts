import { afterAll, describe, expect, mock, test } from 'bun:test';

const asked: { capability: string; body?: Record<string, unknown> }[] = [];

const hostBridge = { ...(await import('../../../src/lib/host-bridge')) };

afterAll(() => {
	mock.module('../../../src/lib/host-bridge', () => hostBridge);
});

mock.module('../../../src/lib/host-bridge', () => ({
	...hostBridge,
	onCompanyEvent: () => () => undefined,
	callCompanyApp: async ({ capability, body }: { capability: string; body?: Record<string, unknown> }) => {
		asked.push({ capability, body });
		if (capability !== 'person.inbox.list') return { status: 200, body: {} };
		return {
			status: 200,
			body: {
				items: [
					{
						reason: 'thread_reply',
						isUnread: true,
						message: {
							id: 'reply-1',
							conversationID: 'channel-1',
							parentID: 'root-1',
							authorExternalID: 'person-a',
							body: '고쳤어요',
							postedAt: '2026-10-12T05:00:00.000Z'
						}
					}
				]
			}
		};
	}
}));

const { fetchInbox, markThreadRead } = await import('../../../src/lib/messenger/messenger-api');

describe('the inbox over the company bridge', () => {
	test('asks person.inbox.list and keeps why each message is there, and whether it is unread', async () => {
		asked.length = 0;

		const items = await fetchInbox();

		expect(asked).toEqual([{ capability: 'person.inbox.list', body: undefined }]);
		expect(items).toEqual([
			{
				reason: 'thread_reply',
				isUnread: true,
				message: {
					id: 'reply-1',
					channelID: 'channel-1',
					parentID: 'root-1',
					author: { externalID: 'person-a' },
					body: '고쳤어요',
					postedAt: '2026-10-12T05:00:00.000Z'
				}
			}
		]);
	});

	test('marks a thread read by naming its root alongside its conversation', async () => {
		asked.length = 0;

		await markThreadRead('channel-1', 'root-1', '2026-10-12T05:00:00.000Z');

		expect(asked).toEqual([
			{
				capability: 'person.read_state.mark',
				body: { conversationID: 'channel-1', parentID: 'root-1', readAt: '2026-10-12T05:00:00.000Z' }
			}
		]);
	});
});
