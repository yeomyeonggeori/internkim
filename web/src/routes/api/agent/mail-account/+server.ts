import { error, json } from '@sveltejs/kit';
import { callingAgent, environmentOf } from '$lib/server/agent-request';
import { mailAccountOfMember } from '$lib/server/mail-account';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ request, url, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));

	const memberID = url.searchParams.get('memberID') ?? '';
	if (!memberID) error(400, 'which member');
	if (!(await belongsToCompany(client, memberID, companyID))) {
		error(403, 'that member belongs to another company');
	}

	return json({ account: await mailAccountOfMember(client, memberID) });
};

async function belongsToCompany(
	client: Parameters<typeof mailAccountOfMember>[0],
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
