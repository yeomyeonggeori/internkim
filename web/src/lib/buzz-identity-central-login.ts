import { callCompanyApp } from './host-bridge';

// A person's Buzz key is derived from a seed and their email
// (internal/buzzidentity/identity.go), which is what makes the agent's messages
// carry their name. A company browser has no seed, so it asks the machine that
// holds one; minting a key here would give the same person a second identity
// and an empty history.
// Signing in must not depend on this. A company that cannot answer the claim —
// no machine reachable, an older relay that does not forward the capability —
// still has a person who needs to get to their work; they lose the Buzz app
// until it can, and nothing else.
export async function claimCentralBuzzSecret(owner?: { accountID: string; companyID: string; accessToken: string }): Promise<string | null> {
	try {
		const answer = await callCompanyApp({ capability: 'person.buzz.claim' }, owner);
		if (answer.status !== 200) {
			console.warn('buzz claim answered', answer.status);
			return null;
		}
		return secretHexOf(answer.body) ?? null;
	} catch (error) {
		console.warn('buzz claim did not answer', error);
		return null;
	}
}

function secretHexOf(body: unknown): string | undefined {
	if (typeof body !== 'object' || body === null || !('secretHex' in body)) return undefined;
	const secretHex = body.secretHex;
	return typeof secretHex === 'string' && secretHex.length > 0 ? secretHex : undefined;
}
