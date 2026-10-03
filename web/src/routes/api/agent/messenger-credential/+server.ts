import { error, json } from '@sveltejs/kit';
import { callingAgent, environmentOf } from '$lib/server/agent-request';
import {
	connectMessengerAccount,
	keepMemberCredential,
	memberBelongsToCompany,
	memberCredential
} from '$lib/server/member-credential';
import {
	memberCredentialKindSchema,
	messengerPlatformOfCredentialKind
} from '$lib/server/public-api/catalog/credential';
import type { RequestHandler } from './$types';

// The company's own server asks for the credential it needs to act as a person
// on their messenger. This is the only caller that may: the browser was handed
// the same secret once, to name itself with, and no longer is.
export const GET: RequestHandler = async ({ request, url, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));

	const memberID = url.searchParams.get('memberID') ?? '';
	const kind = url.searchParams.get('kind')?.trim() ?? '';
	if (!memberID) error(400, 'which member');
	if (!kind) error(400, 'which credential kind the messenger accepts');
	if (!(await memberBelongsToCompany(client, memberID, companyID))) {
		error(403, 'that member belongs to another company');
	}

	return json({ credential: await memberCredential(client, memberID, kind) });
};

// A key the machine derived is one the record has to hold, or the next thing to
// ask for it - the web messenger, which never sees a seed - is handed whatever
// credential the person had before.
export const POST: RequestHandler = async ({ request, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));

	const asked = (await request.json().catch(() => ({}))) as {
		memberID?: unknown;
		kind?: unknown;
		externalID?: unknown;
		secret?: unknown;
	};
	const memberID = typeof asked.memberID === 'string' ? asked.memberID.trim() : '';
	const kind = memberCredentialKindSchema.safeParse(
		typeof asked.kind === 'string' ? asked.kind.trim() : ''
	);
	const externalID = typeof asked.externalID === 'string' ? asked.externalID.trim() : '';
	const secret = typeof asked.secret === 'string' ? asked.secret : '';
	if (!memberID) error(400, 'which member');
	if (!kind.success) error(400, 'a credential has a kind the record declares');
	if (!secret) error(400, 'a credential has a secret');
	if (!(await memberBelongsToCompany(client, memberID, companyID))) {
		error(403, 'that member belongs to another company');
	}

	const messengerPlatform = messengerPlatformOfCredentialKind(kind.data);
	if (messengerPlatform === null) {
		await keepMemberCredential(client, memberID, { kind: kind.data, externalID, secret });
	} else {
		if (!externalID) error(400, 'a messenger credential names the account it signs in as');
		await connectMessengerAccount(client, companyID, {
			memberID,
			platform: messengerPlatform,
			kind: kind.data,
			externalID,
			name: '',
			secret
		});
	}
	return json({ kept: { memberID, kind: kind.data } });
};
