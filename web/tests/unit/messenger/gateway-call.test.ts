import { describe, expect, test } from 'bun:test';

// The browser used to name itself in every call, by attaching the messenger
// credential it had just been handed. The gateway verifies a Supabase token
// before it routes anything, so the envelope carries no identity at all and
// there is nothing in it for a caller to overstate.
function envelopeOf(requestID: string, capability: string, body?: Record<string, unknown>) {
	return { kind: 'call', requestID, capability, body: body ?? {} };
}

describe('the call a browser sends the gateway', () => {
	test('names the request and the capability, and nothing about who is asking', () => {
		const envelope = envelopeOf('r1', 'person.message.send', {
			conversationID: 'channel-1',
			body: 'hello'
		});

		expect(Object.keys(envelope).sort()).toEqual(['body', 'capability', 'kind', 'requestID']);
		expect(envelope.body).toEqual({ conversationID: 'channel-1', body: 'hello' });
	});

	test('carries no actor, no credential and no reply address', () => {
		const written = JSON.stringify(
			envelopeOf('r1', 'person.credential.issue', { answers: { password: 'a-password' } })
		);

		expect(written).not.toContain('actor');
		expect(written).not.toContain('replyTo');
		expect(written).not.toContain('secret');
	});

	test('a call with no arguments still carries a body the server can read', () => {
		expect(envelopeOf('r1', 'person.identity').body).toEqual({});
	});
});
