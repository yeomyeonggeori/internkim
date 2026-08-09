import { error, json } from '@sveltejs/kit';
import { environmentOf } from '$lib/server/agent-request';
import { callingMember } from '$lib/server/member-request';
import { memberCredential } from '$lib/server/member-credential';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ request, url, platform }) => {
	const { record, memberID } = await callingMember(request, environmentOf(platform));

	const kind = url.searchParams.get('kind') ?? 'mattermost';
	const credential = await memberCredential(record, memberID, kind);
	if (!credential) error(404, 'no messenger credential yet');
	return json(credential);
};
