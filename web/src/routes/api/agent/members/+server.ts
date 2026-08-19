import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { callingAgent, environmentOf } from '$lib/server/agent-request';

export const GET: RequestHandler = async ({ request, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));

	const { data, error: queryError } = await client
		.from('member')
		.select('id, email, name, is_admin, status')
		.eq('company_id', companyID)
		.order('email');
	if (queryError) return json({ error: queryError.message }, { status: 502 });

	const members = (data ?? [])
		.filter((member) => typeof member.email === 'string' && member.email.trim() !== '')
		.map((member) => ({
			memberID: member.id,
			email: member.email.trim().toLowerCase(),
			name: typeof member.name === 'string' ? member.name.trim() : '',
			role: member.is_admin ? 'admin' : 'member',
			status: member.status
		}));

	return json({ members });
};
