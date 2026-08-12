import { describe, expect, test } from 'bun:test';
import { callPayload, channelPayload } from '../../../src/lib/host-bridge';

const actor = { kind: 'mattermost-token', secret: 'the-person-token' };

describe('callPayload', () => {
	test('the relay is handed the names it reads', () => {
		const payload = callPayload('c1', 'member-1', { capability: 'person.message.send' }, actor);

		expect(Object.keys(payload).sort()).toEqual(['body', 'callID', 'capability', 'replyTo']);
		expect(payload.callID).toBe('c1');
		expect(payload.capability).toBe('person.message.send');
	});

	test('the actor rides inside the body, where the relay looks for it', () => {
		const payload = callPayload('c1', 'member-1', { capability: 'person.message.send' }, actor);

		expect((payload.body as Record<string, unknown>).actor).toEqual(actor);
	});

	test('a call carries its own arguments alongside the actor', () => {
		const payload = callPayload(
			'c1',
			'member-1',
			{ capability: 'person.message.send', body: { conversationID: 'channel-1', body: 'hello' } },
			actor
		);

		expect(payload.body).toEqual({ conversationID: 'channel-1', body: 'hello', actor });
	});

	test('the reply address is the caller, never something the caller chose', () => {
		const payload = callPayload('c1', 'member-1', { capability: 'person.identity' }, actor);

		expect(payload.replyTo).toBe('member-1');
	});
});

describe('channelPayload', () => {
	test('registration carries no actor, because the channel is what proves who is asking', () => {
		const payload = channelPayload('c1', 'member-1', {
			capability: 'person.credential.issue',
			body: { answers: { password: 'a-password' } }
		});

		expect(payload.body).toEqual({ answers: { password: 'a-password' } });
		expect(JSON.stringify(payload)).not.toContain('actor');
	});

	test('the reply address is still the caller, never something the caller chose', () => {
		const payload = channelPayload('c1', 'member-1', {
			capability: 'person.credential.requirement'
		});

		expect(payload.replyTo).toBe('member-1');
		expect(payload.body).toEqual({});
	});
});
