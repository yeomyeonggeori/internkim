import { callCompanyAppByChannel } from '$lib/host-bridge';

export type CredentialField = { name: string; label: string; isSecret: boolean };

export type CredentialRequirement = {
	kind: 'sign-in' | 'secret' | 'redirect';
	fields: CredentialField[];
	redirectURL?: string;
};

export type ConnectedIdentity = { externalID: string; name: string };

async function askTheCompanyApp<Value>(
	capability: string,
	body?: Record<string, unknown>
): Promise<Value> {
	const answer = await callCompanyAppByChannel({ capability, body });
	if (answer.status >= 300) {
		const refusal = (answer.body as { error?: string } | null)?.error;
		throw new Error(refusal ?? `the company app answered ${answer.status}`);
	}
	return answer.body as Value;
}

export function whatTheMessengerNeeds(): Promise<CredentialRequirement> {
	return askTheCompanyApp<CredentialRequirement>('person.credential.requirement');
}

export function connectMessengerAccount(
	answers: Record<string, string>
): Promise<ConnectedIdentity> {
	return askTheCompanyApp<ConnectedIdentity>('person.credential.issue', { answers });
}
