import { error, json } from '@sveltejs/kit';
import { callingAgent, environmentOf } from '$lib/server/agent-request';
import { mailAccountOfMember } from '$lib/server/mail-account';
import { memberBelongsToCompany } from '$lib/server/member-credential';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ request, url, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));

	const memberID = url.searchParams.get('memberID') ?? '';
	if (!memberID) error(400, 'which member');
	if (!(await memberBelongsToCompany(client, memberID, companyID))) {
		error(403, 'that member belongs to another company');
	}

	return json({ account: await mailAccountOfMember(client, memberID) });
};
