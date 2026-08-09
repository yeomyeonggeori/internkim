import { mintUserAccessToken, type MattermostSession, type MattermostSettings } from './mattermost';

export type PersonToProvision = { externalID: string };

export type MintedCredential = { externalID: string; secret: string };

export type ProvisionReport = {
	alreadyHeld: number;
	minted: number;
	refused: string[];
};

export async function mintMissingTokens(
	settings: MattermostSettings,
	session: MattermostSession,
	people: PersonToProvision[],
	alreadyHeld: Set<string>
): Promise<{ credentials: MintedCredential[]; report: ProvisionReport }> {
	const credentials: MintedCredential[] = [];
	const refused: string[] = [];
	let held = 0;

	for (const person of people) {
		if (alreadyHeld.has(person.externalID)) {
			held += 1;
			continue;
		}
		const secret = await mintUserAccessToken(settings, session, person.externalID).catch(() => null);
		if (!secret) {
			refused.push(person.externalID);
			continue;
		}
		credentials.push({ externalID: person.externalID, secret });
	}

	return { credentials, report: { alreadyHeld: held, minted: credentials.length, refused } };
}
