import { json } from '@sveltejs/kit';
import { callingAgent, environmentOf } from '$lib/server/agent-request';
import { mailAccountOfMember } from '$lib/server/mail-account';
import { mailAccountCredentialKind } from '$lib/server/public-api/catalog/credential';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ request, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));

	const members = await client
		.from('member')
		.select('id')
		.eq('company_id', companyID)
		.returns<{ id: string }[]>();
	if (members.error) throw new Error(members.error.message);
	const worksHere = new Set(members.data.map((member) => member.id));

	const credentials = await client
		.from('credential')
		.select('member_id')
		.eq('kind', mailAccountCredentialKind)
		.returns<{ member_id: string | null }[]>();
	if (credentials.error) throw new Error(credentials.error.message);

	const accounts = [];
	for (const credential of credentials.data) {
		if (!credential.member_id || !worksHere.has(credential.member_id)) continue;
		const account = await mailAccountOfMember(client, credential.member_id);
		if (account?.IMAPHost) accounts.push(account);
	}
	return json({ accounts });
};
