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

export const POST: RequestHandler = async ({ request, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));

	const asked = (await request.json().catch(() => ({}))) as { email?: unknown; name?: unknown };
	const email = typeof asked.email === 'string' ? asked.email.trim().toLowerCase() : '';
	if (!email) error(400, 'email required');
	const name = typeof asked.name === 'string' ? asked.name.trim() : '';

	const existing = await client
		.from('member')
		.select('id, email, is_admin, status, company_id')
		.eq('email', email)
		.maybeSingle();
	if (existing.error) return json({ error: existing.error.message }, { status: 502 });
	if (existing.data && existing.data.company_id !== companyID) error(409, 'that address belongs to another company');
	if (existing.data) {
		if (name) await client.from('member').update({ name }).eq('id', existing.data.id);
		return json({
			member: {
				memberID: existing.data.id,
				email,
				role: existing.data.is_admin ? 'admin' : 'member',
				status: existing.data.status
			}
		});
	}

	const created = await client
		.from('member')
		.insert({ company_id: companyID, email, ...(name ? { name } : {}) })
		.select('id, is_admin, status')
		.single();
	if (created.error) return json({ error: created.error.message }, { status: 502 });

	return json({
		member: {
			memberID: created.data.id,
			email,
			role: created.data.is_admin ? 'admin' : 'member',
			status: created.data.status
		}
	});
};
