import { json, error } from '@sveltejs/kit';
import { callingAgent, environmentOf } from '$lib/server/agent-request';
import { connectMessengerAccount, MemberOfAnotherCompany } from '$lib/server/member-credential';
import { messengerIdentityCredentialKinds } from '$lib/server/public-api/catalog/credential';
import { messengerPlatformNames } from '$lib/server/public-api/catalog/protocol';
import type { RequestHandler } from './$types';

type ConnectRequest = {
	platform?: unknown;
	kind?: unknown;
	memberID?: unknown;
	externalID?: unknown;
	name?: unknown;
	secret?: unknown;
};

function required(value: unknown, field: string): string {
	if (typeof value !== 'string' || !value.trim()) error(400, `${field} is required`);
	return value.trim();
}

function declaredMessengerPlatform(value: unknown): string {
	const platform = required(value, 'platform');
	if (!messengerPlatformNames.some((declared) => declared === platform)) {
		error(400, `no messenger named ${platform} is adapted here`);
	}
	return platform;
}

function declaredMessengerIdentityKind(value: unknown): string {
	const kind = required(value, 'kind');
	if (!messengerIdentityCredentialKinds.some((declared) => declared === kind)) {
		error(400, `no credential kind named ${kind} carries a messenger identity here`);
	}
	return kind;
}

export const POST: RequestHandler = async ({ request, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));
	const body = (await request.json().catch(() => ({}))) as ConnectRequest;

	const account = {
		memberID: required(body.memberID, 'memberID'),
		platform: declaredMessengerPlatform(body.platform),
		kind: declaredMessengerIdentityKind(body.kind),
		externalID: required(body.externalID, 'externalID'),
		name: typeof body.name === 'string' ? body.name : '',
		secret: required(body.secret, 'secret')
	};

	try {
		await connectMessengerAccount(client, companyID, account);
	} catch (refusal) {
		if (refusal instanceof MemberOfAnotherCompany) error(403, 'that member belongs to another company');
		throw refusal;
	}

	return json({ connected: true });
};
