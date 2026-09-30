import { describe, expect, test } from 'bun:test';
import { arrivalsWatchCapability, renewArrivalWatches } from './arrival-watchers';

const arrivalsURL = 'http://127.0.0.1:18091/arrived';
const typingURL = 'http://127.0.0.1:18091/typing';

describe('renewing arrival watches', () => {
	test('asks chatd to watch every active member who holds a credential, telling it where to post', async () => {
		const asked: { capability: string; body: Record<string, unknown> }[] = [];
		const renewal = await renewArrivalWatches({
			activeMemberIDs: async () => ['member-1', 'member-2', 'member-3'],
			credentialOf: async (memberID) =>
				memberID === 'member-2' ? null : { kind: 'buzz-secret', secret: `secret-of-${memberID}` },
			askChatd: async (capability, body) => {
				asked.push({ capability, body });
				const actor = body.actor as { secret: string };
				return actor.secret === 'secret-of-member-3'
					? { status: 502, body: { error: 'the relay refused' } }
					: { status: 200, body: {} };
			},
			arrivalsURL,
			typingURL,
			report: () => {}
		});

		expect(asked).toEqual([
			{ capability: arrivalsWatchCapability, body: { actor: { kind: 'buzz-secret', secret: 'secret-of-member-1' }, arrivalsURL, typingURL } },
			{ capability: arrivalsWatchCapability, body: { actor: { kind: 'buzz-secret', secret: 'secret-of-member-3' }, arrivalsURL, typingURL } }
		]);
		expect(renewal).toEqual({
			watched: 1,
			withoutCredential: 1,
			refusals: ['member member-3: chatd answered 502: {"error":"the relay refused"}']
		});
	});
});
