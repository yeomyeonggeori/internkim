import { describe, expect, test } from 'bun:test';
import { companyEventOf } from '$lib/company-event';


describe('companyEventOf', () => {
	test('reads what the company machine says arrived', () => {
		expect(
			companyEventOf({ kind: 'message.arrived', conversationID: 'channel-1', messageID: 'event-1' })
		).toEqual({ kind: 'message.arrived', conversationID: 'channel-1', messageID: 'event-1' });
	});

	test('reads who is typing where', () => {
		expect(
			companyEventOf({ kind: 'typing.started', conversationID: 'channel-1', authorExternalID: 'person-1' })
		).toEqual({ kind: 'typing.started', conversationID: 'channel-1', authorExternalID: 'person-1' });
	});

	test('keeps only the fields it knows, and needs a kind', () => {
		expect(companyEventOf({ kind: 'message.arrived', conversationID: 'channel-1', extra: 1 })).toEqual({
			kind: 'message.arrived',
			conversationID: 'channel-1'
		});
		expect(companyEventOf({ conversationID: 'channel-1' })).toBeNull();
		expect(companyEventOf({ kind: '' })).toBeNull();
		expect(companyEventOf('message.arrived')).toBeNull();
		expect(companyEventOf(null)).toBeNull();
	});
});
