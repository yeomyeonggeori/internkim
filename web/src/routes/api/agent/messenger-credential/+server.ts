import { error, json } from '@sveltejs/kit';
import { callingAgent, environmentOf } from '$lib/server/agent-request';
import { keepMemberCredential, memberCredential } from '$lib/server/member-credential';
import type { RequestHandler } from './$types';
import type { SupabaseClient } from '@supabase/supabase-js';

// The company's own server asks for the credential it needs to act as a person
// on their messenger. This is the only caller that may: the browser was handed
// the same secret once, to name itself with, and no longer is.
export const GET: RequestHandler = async ({ request, url, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));

	const memberID = url.searchParams.get('memberID') ?? '';
	const kind = url.searchParams.get('kind')?.trim() ?? '';
	if (!memberID) error(400, 'which member');
	if (!kind) error(400, 'which credential kind the messenger accepts');
	if (!(await belongsToCompany(client, memberID, companyID))) {
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
	const kind = typeof asked.kind === 'string' ? asked.kind.trim() : '';
	const externalID = typeof asked.externalID === 'string' ? asked.externalID.trim() : '';
	const secret = typeof asked.secret === 'string' ? asked.secret : '';
	if (!memberID) error(400, 'which member');
	if (!kind) error(400, 'a credential has a kind');
	if (!secret) error(400, 'a credential has a secret');
	if (!(await belongsToCompany(client, memberID, companyID))) {
		error(403, 'that member belongs to another company');
	}

	await keepMemberCredential(client, memberID, { kind, externalID, secret });
	return json({ kept: { memberID, kind } });
};

async function belongsToCompany(
	client: SupabaseClient,
	memberID: string,
	companyID: string
): Promise<boolean> {
	const member = await client
		.from('member')
		.select('company_id')
		.eq('id', memberID)
		.maybeSingle<{ company_id: string }>();
	if (member.error) throw new Error(member.error.message);
	return member.data?.company_id === companyID;
}
