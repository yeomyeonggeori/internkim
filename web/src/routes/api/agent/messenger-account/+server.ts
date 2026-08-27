import { json, error } from '@sveltejs/kit';
import { callingAgent, environmentOf } from '$lib/server/agent-request';
import { connectMessengerAccount } from '$lib/server/member-credential';
import type { RequestHandler } from './$types';

type ConnectRequest = {
	platform?: unknown;
	kind?: unknown;
	memberID?: unknown;
	externalID?: unknown;
	name?: unknown;
	secret?: unknown;
};

function required(value: unknown, field: string): string {
	if (typeof value !== 'string' || !value.trim()) error(400, `${field} is required`);
	return value.trim();
}

export const POST: RequestHandler = async ({ request, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));
	const body = (await request.json().catch(() => ({}))) as ConnectRequest;

	await connectMessengerAccount(client, companyID, {
		memberID: required(body.memberID, 'memberID'),
		platform: required(body.platform, 'platform'),
		kind: required(body.kind, 'kind'),
		externalID: required(body.externalID, 'externalID'),
		name: typeof body.name === 'string' ? body.name : '',
		secret: required(body.secret, 'secret')
	});

	return json({ connected: true });
};
