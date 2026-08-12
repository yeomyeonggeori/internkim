import { describe, expect, mock, test } from 'bun:test';

let isCentralPlane = true;
const sentThroughBridge: { message: string; channelID?: string }[] = [];

mock.module('../../../src/lib/supabase', () => ({
	isSupabaseConfigured: () => isCentralPlane
}));

mock.module('../../../src/lib/messenger/channel-over-bridge', () => ({
	bridgeConversations: async () => [],
	bridgePeople: async () => [],
	bridgeDirectMessage: async () => '',
	bridgeConversation: async () => ({ messages: [] }),
	bridgeSendMessage: async (message: string, channelID?: string) => {
		sentThroughBridge.push({ message, channelID });
	}
}));

const { canSendAttachments, sendChannelMessage } = await import(
	'../../../src/lib/components/channel/channel-api'
);

const attachment = { filename: 'evidence.png', contentType: 'image/png', contentBase64: 'AAAA' };

describe('an attachment the company app cannot carry', () => {
	test('is refused by name, instead of the message going without it', async () => {
		isCentralPlane = true;
		sentThroughBridge.length = 0;

		await expect(sendChannelMessage('here it is', [attachment], 'channel-1')).rejects.toThrow(
			'the company app carries no attachment yet, so this message was not sent'
		);
		expect(sentThroughBridge).toHaveLength(0);
	});

	test('a message carrying none still goes', async () => {
		isCentralPlane = true;
		sentThroughBridge.length = 0;

		await sendChannelMessage('just words', [], 'channel-1');

		expect(sentThroughBridge).toEqual([{ message: 'just words', channelID: 'channel-1' }]);
	});

	test('the composer is told, so nobody picks a file that cannot be sent', () => {
		isCentralPlane = true;
		expect(canSendAttachments()).toBe(false);

		isCentralPlane = false;
		expect(canSendAttachments()).toBe(true);
	});
});
