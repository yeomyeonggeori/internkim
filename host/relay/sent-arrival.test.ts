import { describe, expect, test } from 'bun:test';
import type { ArrivedMessage } from './arrived';
import { arrivalOfSentMessage, tellAboutSentMessage, type SentMessageDispatch } from './sent-arrival';

const actor = { kind: 'buzz-secret', secret: 'a-held-secret' };
const sent = { conversationID: 'channel-1', body: '오늘 회의 30분 미뤄도 될까요' };

function aMessenger(options: { identityStatus?: number; conversationsStatus?: number } = {}) {
	const asked: { capability: string; body: Record<string, unknown> }[] = [];
	const told: ArrivedMessage[] = [];
	const dispatch: SentMessageDispatch = {
		askChatd: async (capability, body) => {
			asked.push({ capability, body });
			if (capability === 'person.identity') {
				return { status: options.identityStatus ?? 200, body: { externalID: 'U-author' } };
			}
			return {
				status: options.conversationsStatus ?? 200,
				body: {
					conversations: [
						{ id: 'channel-1', participantExternalIDs: ['U-author', 'U-first', 'U-second', 7] },
						{ id: 'channel-2', participantExternalIDs: ['U-elsewhere'] }
					]
				}
			};
		},
		tellThoseAddressed: async (arrived) => {
			told.push(arrived);
			return arrived.recipientExternalIDs.length;
		}
	};
	return { asked, told, dispatch };
}

describe('arrivalOfSentMessage', () => {
	test('the others in the conversation are the ones told, under the sender name', async () => {
		const { asked, dispatch } = aMessenger();

		const arrived = await arrivalOfSentMessage(dispatch, sent, actor, 'event-1');

		expect(arrived).toEqual({
			conversationID: 'channel-1',
			messageID: 'event-1',
			authorExternalID: 'U-author',
			authorName: '',
			recipientExternalIDs: ['U-first', 'U-second'],
			preview: '오늘 회의 30분 미뤄도 될까요'
		});
		expect(asked.map((entry) => entry.capability)).toEqual(['person.identity', 'person.conversations.list']);
		expect(asked.every((entry) => entry.body.actor === actor)).toBe(true);
	});

	test('a message the messenger gave no id for is not an arrival', async () => {
		const { asked, dispatch } = aMessenger();

		expect(await arrivalOfSentMessage(dispatch, sent, actor, '')).toBeNull();
		expect(asked).toHaveLength(0);
	});

	test('a sender the messenger cannot identify tells nobody', async () => {
		const { dispatch } = aMessenger({ identityStatus: 409 });

		expect(await arrivalOfSentMessage(dispatch, sent, actor, 'event-1')).toBeNull();
	});

	test('a conversation the messenger cannot list leaves nobody to tell', async () => {
		const { dispatch } = aMessenger({ conversationsStatus: 502 });

		expect((await arrivalOfSentMessage(dispatch, sent, actor, 'event-1'))?.recipientExternalIDs).toEqual([]);
	});
});

describe('tellAboutSentMessage', () => {
	test('what was sent is what the others are told about', async () => {
		const { told, dispatch } = aMessenger();

		expect(await tellAboutSentMessage(dispatch, sent, actor, 'event-1')).toBe(2);
		expect(told.map((arrived) => arrived.recipientExternalIDs)).toEqual([['U-first', 'U-second']]);
	});

	test('nothing is told when there is no arrival to tell about', async () => {
		const { told, dispatch } = aMessenger({ identityStatus: 409 });

		expect(await tellAboutSentMessage(dispatch, sent, actor, 'event-1')).toBe(0);
		expect(told).toEqual([]);
	});
});
