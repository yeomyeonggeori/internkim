import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { callingAgent, environmentOf } from '$lib/server/agent-request';

export const GET: RequestHandler = async ({ request, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));

	const { data, error: queryError } = await client
		.from('company')
		.select('name, profile_image, timezone, locale')
		.eq('id', companyID)
		.maybeSingle();
	if (queryError) return json({ error: queryError.message }, { status: 502 });
	if (!data) return json({ company: null });

	return json({
		company: {
			name: data.name,
			profileImage: data.profile_image ?? '',
			timezone: data.timezone ?? '',
			locale: data.locale ?? ''
		}
	});
};
