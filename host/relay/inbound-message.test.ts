import { describe, expect, test } from 'bun:test';
import { displayNameForRequester, inboundMessageKey, readInboundMessage } from './inbound-message';

function aChatdBody(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		platform: 'buzz',
		conversationID: 'conversation-1',
		messageID: 'message-7',
		prompt: '박예시한테 DM 보내줘',
		replyTargetID: 'message-6',
		isThread: true,
		context: {
			conversationType: 'direct',
			responseLanguage: 'ko',
			sender: {
				email: 'Sample@Example.com',
				name: '이샘플',
				handle: 'sample'
			}
		},
		...overrides
	};
}

describe('readInboundMessage', () => {
	test('localizes a stored Korean name for the requested response language', () => {
		expect(displayNameForRequester('지우 박', 'en', 'ko')).toBe('박지우');
		expect(displayNameForRequester('지우 박', 'en', 'ko-KR')).toBe('박지우');
		expect(displayNameForRequester('지우 박', 'en', 'korean')).toBe('지우 박');
		expect(displayNameForRequester('지우 박', 'ko', 'en')).toBe('박지우');
		expect(displayNameForRequester('지우 박', 'en', 'en')).toBe('지우 박');
	});

	test('the body chatd posts carries the requester, the addressing and the words', () => {
		const inbound = readInboundMessage(aChatdBody());

		expect(inbound?.requester).toEqual({
			email: 'sample@example.com',
			name: '이샘플',
			handle: 'sample'
		});
		expect(inbound?.addressing).toEqual({
			platform: 'buzz',
			conversationID: 'conversation-1',
			conversationType: 'direct',
			replyTargetID: 'message-6',
			isThread: true,
			responseLanguage: 'ko'
		});
		expect(inbound?.messageID).toBe('message-7');
		expect(inbound?.message).toBe('박예시한테 DM 보내줘');
		expect(inbound?.key).toBe('buzz:conversation-1:message-7');
		expect(inbound?.key).toBe(inboundMessageKey('buzz', 'conversation-1', 'message-7'));
	});

	test('the whole context comes through, so the turn keeps facts this file does not read', () => {
		const inbound = readInboundMessage(
			aChatdBody({ context: { sender: { email: 'sample@example.com' }, attachments: ['a-picture.png'] } })
		);

		expect(inbound?.context).toEqual({
			sender: { email: 'sample@example.com' },
			attachments: ['a-picture.png']
		});
	});

	test('a body with no sender address is nobody the agent can answer', () => {
		expect(readInboundMessage(aChatdBody({ context: { conversationType: 'direct' } }))).toBeNull();
		expect(readInboundMessage(aChatdBody({ context: { sender: { name: '이샘플' } } }))).toBeNull();
		expect(readInboundMessage(aChatdBody({ context: { sender: { email: '   ' } } }))).toBeNull();
	});

	test('a body with no words is not a turn', () => {
		expect(readInboundMessage(aChatdBody({ prompt: undefined }))).toBeNull();
		expect(readInboundMessage(aChatdBody({ prompt: '   ' }))).toBeNull();
	});

	test('a body with no conversation has nowhere to answer', () => {
		expect(readInboundMessage(aChatdBody({ conversationID: undefined }))).toBeNull();
		expect(readInboundMessage(aChatdBody({ conversationID: '  ' }))).toBeNull();
	});

	test('something that is not an object at all is not a message', () => {
		expect(readInboundMessage(null)).toBeNull();
		expect(readInboundMessage('박예시한테 DM 보내줘')).toBeNull();
	});

	test('the platform falls back to the one the sender arrived on', () => {
		const inbound = readInboundMessage(
			aChatdBody({
				platform: undefined,
				context: { sender: { email: 'sample@example.com', platform: 'slack' } }
			})
		);

		expect(inbound?.addressing.platform).toBe('slack');
		expect(inbound?.key).toBe('slack:conversation-1:message-7');
	});

	test('whitespace around every field is trimmed before it is used', () => {
		const inbound = readInboundMessage(
			aChatdBody({
				platform: '  buzz  ',
				conversationID: '  conversation-1  ',
				messageID: '  message-7  ',
				prompt: '  박예시한테 DM 보내줘  ',
				replyTargetID: '  message-6  ',
				context: {
					conversationType: '  direct  ',
					sender: { email: '  Sample@Example.com  ', name: '  이샘플  ', handle: '  sample  ' }
				}
			})
		);

		expect(inbound?.requester).toEqual({
			email: 'sample@example.com',
			name: '이샘플',
			handle: 'sample'
		});
		expect(inbound?.message).toBe('박예시한테 DM 보내줘');
		expect(inbound?.addressing.conversationType).toBe('direct');
		expect(inbound?.addressing.replyTargetID).toBe('message-6');
		expect(inbound?.key).toBe('buzz:conversation-1:message-7');
	});

	test('a name, a handle and a reply target nobody sent are left undefined', () => {
		const inbound = readInboundMessage(
			aChatdBody({ replyTargetID: undefined, context: { sender: { email: 'sample@example.com' } } })
		);

		expect(inbound?.requester.name).toBeUndefined();
		expect(inbound?.requester.handle).toBeUndefined();
		expect(inbound?.addressing.replyTargetID).toBeUndefined();
		expect(inbound?.addressing.conversationType).toBeUndefined();
		expect(inbound?.addressing.isThread).toBe(true);
	});
});
