import { error, json } from '@sveltejs/kit';
import { callingAgent, environmentOf } from '$lib/server/agent-request';
import { memberMessengerCredential } from '$lib/server/member-credential';
import type { RequestHandler } from './$types';
import type { SupabaseClient } from '@supabase/supabase-js';

// The company's own server asks for the credential it needs to act as a person
// on their messenger. This is the only caller that may: the browser was handed
// the same secret once, to name itself with, and no longer is.
export const GET: RequestHandler = async ({ request, url, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));

	const memberID = url.searchParams.get('memberID') ?? '';
	if (!memberID) error(400, 'which member');
	if (!(await belongsToCompany(client, memberID, companyID))) {
		error(403, 'that member belongs to another company');
	}

	return json({ credential: await memberMessengerCredential(client, memberID) });
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
