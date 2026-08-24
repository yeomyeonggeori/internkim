import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { callingAgent, environmentOf } from '$lib/server/agent-request';

export const GET: RequestHandler = async ({ request, url, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));

	// Without an address this answers the whole directory. Who works here is one
	// question with one answer, and a caller that has to ask name by name cannot
	// know about somebody it has never heard of - which is every person invited
	// since it last looked.
	const email = (url.searchParams.get('email') ?? '').trim().toLowerCase();
	if (!email) {
		const everyone = await client
			.from('member')
			.select('id, email, is_admin, status')
			.eq('company_id', companyID);
		if (everyone.error) return json({ error: everyone.error.message }, { status: 502 });
		return json({
			members: (everyone.data ?? []).map((member) => ({
				memberID: member.id,
				email: member.email,
				role: member.is_admin ? 'admin' : 'member',
				status: member.status
			}))
		});
	}

	const { data, error: queryError } = await client
		.from('member')
		.select('id, email, is_admin, status')
		.eq('company_id', companyID)
		.eq('email', email)
		.maybeSingle();
	if (queryError) return json({ error: queryError.message }, { status: 502 });
	if (!data) return json({ member: null });

	return json({
		member: {
			memberID: data.id,
			email,
			role: data.is_admin ? 'admin' : 'member',
			status: data.status
		}
	});
};
