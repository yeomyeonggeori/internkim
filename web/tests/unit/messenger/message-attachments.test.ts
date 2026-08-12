import { describe, expect, mock, test } from 'bun:test';

const asked: { capability: string; body: Record<string, unknown> }[] = [];

mock.module('../../../src/lib/host-bridge', () => ({
	callCompanyApp: async ({ capability, body }: { capability: string; body: Record<string, unknown> }) => {
		asked.push({ capability, body });
		return {
			status: 200,
			body: {
				id: 'post-1',
				conversationID: 'channel-1',
				authorExternalID: 'person-1',
				body: 'here it is',
				postedAt: '2026-08-13T00:00:00.000Z',
				reactions: [],
				attachments: []
			}
		};
	}
}));

const { writePost } = await import('../../../src/lib/messenger/messenger-api');

const attachment = { filename: 'evidence.png', contentType: 'image/png', contentBase64: 'AAAA' };

describe('what person.message.send is asked to carry', () => {
	test('a file picked in the composer reaches the call, instead of the message going without it', async () => {
		asked.length = 0;

		await writePost('channel-1', 'here it is', undefined, [attachment]);

		const send = asked.find((call) => call.capability === 'person.message.send');
		expect(send?.body.attachments).toEqual([attachment]);
		expect(send?.body.conversationID).toBe('channel-1');
	});

	test('a message with none names an empty list, so the field is never absent', async () => {
		asked.length = 0;

		await writePost('channel-1', 'just words');

		const send = asked.find((call) => call.capability === 'person.message.send');
		expect(send?.body.attachments).toEqual([]);
	});
});
