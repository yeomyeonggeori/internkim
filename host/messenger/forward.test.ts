import { describe, expect, test } from 'bun:test';
import { actorOf, isPersonCapability, reportableTopic } from './forward';

describe('actorOf', () => {
	test('reads the credential the caller carried', () => {
		expect(actorOf({ body: { actor: { kind: 'mattermost-token', secret: 'abc' } } })).toEqual({
			kind: 'mattermost-token',
			secret: 'abc'
		});
	});

	test('a call with no actor carries nobody', () => {
		expect(actorOf({ body: { conversationID: 'c' } })).toBeNull();
		expect(actorOf({})).toBeNull();
	});

	test('a half-written actor is nobody', () => {
		expect(actorOf({ body: { actor: { kind: 'mattermost-token' } } })).toBeNull();
		expect(actorOf({ body: { actor: { secret: 'abc' } } })).toBeNull();
		expect(actorOf({ body: { actor: { kind: '', secret: 'abc' } } })).toBeNull();
		expect(actorOf({ body: { actor: 'mattermost-token' } })).toBeNull();
	});
});

describe('isPersonCapability', () => {
	test('person capabilities need an actor, assets do not', () => {
		expect(isPersonCapability('person.conversations.list')).toBe(true);
		expect(isPersonCapability('person.message.send')).toBe(true);
		expect(isPersonCapability('asset.emoji')).toBe(false);
		expect(isPersonCapability('asset.picture')).toBe(false);
	});
});

describe('reportableTopic', () => {
	test('an error can be reported to the address the caller named', () => {
		expect(reportableTopic({ replyTo: '00000000-0000-0000-0000-00000000000a' })).toBe(
			'00000000-0000-0000-0000-00000000000a'
		);
	});

	test('anything that is not a member id is nowhere', () => {
		expect(reportableTopic({ replyTo: 'company:1:call' })).toBeNull();
		expect(reportableTopic({ replyTo: '../../etc' })).toBeNull();
		expect(reportableTopic({})).toBeNull();
	});
});
