import { describe, expect, test } from 'bun:test';
import { actorOf, isPersonCapability, reportableTopic, serveCall } from './forward';

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

describe('serveCall', () => {
	function dispatchThatKnows(externalIDs: Record<string, string>) {
		const asked: { capability: string; body: Record<string, unknown> }[] = [];
		return {
			asked,
			dispatch: {
				serveAsset: async (capability: string) => ({ served: capability }),
				askChatd: async (capability: string, body: Record<string, unknown>) => {
					asked.push({ capability, body });
					if (capability === 'person.identity') {
						const actor = body.actor as { secret?: string } | undefined;
						const externalID = actor?.secret === 'known' ? 'U-known' : 'U-stranger';
						return { status: 200, body: { externalID } };
					}
					return { status: 200, body: { conversations: [] } };
				},
				memberOfExternalID: async (externalID: string) => externalIDs[externalID] ?? null
			}
		};
	}

	const callFromTheBrowser = {
		callID: 'c1',
		capability: 'person.conversations.list',
		replyTo: '00000000-0000-0000-0000-0000000000ff',
		body: { actor: { kind: 'mattermost-token', secret: 'known' } }
	};

	test('the payload the web sends reaches chatd with its actor intact', async () => {
		const { asked, dispatch } = dispatchThatKnows({ 'U-known': 'member-1' });

		const served = await serveCall(dispatch, callFromTheBrowser);

		expect(served.status).toBe(200);
		expect(asked.map((entry) => entry.capability)).toEqual([
			'person.identity',
			'person.conversations.list'
		]);
		expect(asked[1]?.body.actor).toEqual({ kind: 'mattermost-token', secret: 'known' });
	});

	test('the answer is addressed to the member the credential belongs to', async () => {
		const { dispatch } = dispatchThatKnows({ 'U-known': 'member-1' });

		const served = await serveCall(dispatch, callFromTheBrowser);

		expect(served.replyTo).toBe('member-1');
	});

	test('a credential nobody here holds is refused before the work is done', async () => {
		const { asked, dispatch } = dispatchThatKnows({});

		const served = await serveCall(dispatch, callFromTheBrowser);

		expect(served.status).toBe(403);
		expect(asked.map((entry) => entry.capability)).toEqual(['person.identity']);
	});

	test('a call with no actor never reaches chatd', async () => {
		const { asked, dispatch } = dispatchThatKnows({ 'U-known': 'member-1' });

		const served = await serveCall(dispatch, { callID: 'c1', capability: 'person.message.send' });

		expect(served.status).toBe(400);
		expect(asked).toEqual([]);
	});

	test('an asset needs no actor and is answered without addressing anyone', async () => {
		const { asked, dispatch } = dispatchThatKnows({});

		const served = await serveCall(dispatch, { callID: 'c1', capability: 'asset.emoji' });

		expect(served).toEqual({ status: 200, body: { served: 'asset.emoji' }, replyTo: null });
		expect(asked).toEqual([]);
	});
});
