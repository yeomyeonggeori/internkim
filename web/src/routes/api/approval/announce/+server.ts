import { error, json } from '@sveltejs/kit';
import { environmentOf } from '$lib/server/agent-request';
import { callingMember } from '$lib/server/member-request';
import { announceApproval } from '$lib/server/announce-approval';
import type { RequestHandler } from './$types';

type AnnounceRequest = { approvalID?: unknown };

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOf(platform);
	const { record, memberID } = await callingMember(request, environment);

	const asked = (await request.json().catch(() => ({}))) as AnnounceRequest;
	if (typeof asked.approvalID !== 'string' || asked.approvalID === '') {
		error(400, 'approvalID names the request the company should hear about');
	}
	return json(await announceApproval(environment, record, memberID, asked.approvalID));
};
