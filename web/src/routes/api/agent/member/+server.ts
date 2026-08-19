import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { callingAgent, environmentOf } from '$lib/server/agent-request';

export const GET: RequestHandler = async ({ request, url, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));

	const email = (url.searchParams.get('email') ?? '').trim().toLowerCase();
	if (!email) error(400, 'email required');

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
