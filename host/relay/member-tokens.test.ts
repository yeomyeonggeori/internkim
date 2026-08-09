import { afterEach, describe, expect, test } from 'bun:test';
import { mintMissingTokens } from './member-tokens';

const settings = { baseURL: 'https://mattermost.test', email: 'bot@example.com', password: 'x' };
const session = { token: 'admin-token', userID: 'admin-id' };
const realFetch = globalThis.fetch;

function serveTokens(refuseFor: string[] = []): string[] {
	const asked: string[] = [];
	globalThis.fetch = (async (input: RequestInfo | URL) => {
		const url = String(input);
		const externalID = url.split('/users/')[1]?.split('/')[0] ?? '';
		asked.push(externalID);
		if (refuseFor.includes(externalID)) return new Response('nope', { status: 403 });
		return Response.json({ token: `token-for-${externalID}` });
	}) as typeof fetch;
	return asked;
}

afterEach(() => {
	globalThis.fetch = realFetch;
});

describe('mintMissingTokens', () => {
	test('mints only for the people who have none', async () => {
		const asked = serveTokens();

		const { credentials, report } = await mintMissingTokens(
			settings,
			session,
			[{ externalID: 'U-one' }, { externalID: 'U-two' }, { externalID: 'U-three' }],
			new Set(['U-two'])
		);

		expect(asked).toEqual(['U-one', 'U-three']);
		expect(credentials.map((credential) => credential.externalID)).toEqual(['U-one', 'U-three']);
		expect(report).toEqual({ alreadyHeld: 1, minted: 2, refused: [] });
	});

	test('asks for nothing when everyone already holds one', async () => {
		const asked = serveTokens();

		const { credentials } = await mintMissingTokens(
			settings,
			session,
			[{ externalID: 'U-one' }],
			new Set(['U-one'])
		);

		expect(asked).toEqual([]);
		expect(credentials).toEqual([]);
	});

	test('a refusal is reported and does not stop the others', async () => {
		serveTokens(['U-two']);

		const { credentials, report } = await mintMissingTokens(
			settings,
			session,
			[{ externalID: 'U-one' }, { externalID: 'U-two' }, { externalID: 'U-three' }],
			new Set()
		);

		expect(credentials.map((credential) => credential.externalID)).toEqual(['U-one', 'U-three']);
		expect(report.refused).toEqual(['U-two']);
		expect(report.minted).toBe(2);
	});

	test('the secret carried is the one the messenger returned', async () => {
		serveTokens();

		const { credentials } = await mintMissingTokens(
			settings,
			session,
			[{ externalID: 'U-one' }],
			new Set()
		);

		expect(credentials[0]?.secret).toBe('token-for-U-one');
	});
});
