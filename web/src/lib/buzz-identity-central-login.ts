import { callCompanyApp } from './host-bridge';

// A person's Buzz key is derived from a seed and their email
// (internal/buzzidentity/identity.go), which is what makes the agent's messages
// carry their name. A company browser has no seed, so it asks the machine that
// holds one; minting a key here would give the same person a second identity
// and an empty history.
export async function claimCentralBuzzSecret(): Promise<string> {
	const answer = await callCompanyApp({ capability: 'person.buzz.claim' });
	if (answer.status !== 200) {
		throw new Error(`buzz claim answered ${answer.status}`);
	}
	const secretHex = secretHexOf(answer.body);
	if (!secretHex) {
		throw new Error('buzz claim carried no key');
	}
	return secretHex;
}

function secretHexOf(body: unknown): string | undefined {
	if (typeof body !== 'object' || body === null || !('secretHex' in body)) return undefined;
	const secretHex = body.secretHex;
	return typeof secretHex === 'string' && secretHex.length > 0 ? secretHex : undefined;
}
