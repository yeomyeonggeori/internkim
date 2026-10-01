import { error, json } from '@sveltejs/kit';
import { connectedAppPermissionOf } from '$lib/public-api-permission';
import { environmentOf } from '$lib/server/agent-request';
import { forgetConnectedAppPermission, keepConnectedAppPermission, planeCredentialsOf } from '$lib/server/control-plane';
import { callingMember, signedInAccessTokenOf } from '$lib/server/member-request';
import type { RequestHandler } from './$types';

async function signedInMember(request: Request, platform: App.Platform | undefined) {
	const environment = environmentOf(platform);
	const plane = planeCredentialsOf(environment);
	if (!plane) error(500, 'the central plane is not configured');
	await signedInAccessTokenOf(request, plane);
	return callingMember(request, environment);
}

export const PUT: RequestHandler = async ({ request, platform }) => {
	const member = await signedInMember(request, platform);
	const asked = (await request.json().catch(() => null)) as { clientID?: unknown; permission?: unknown } | null;
	const clientID = typeof asked?.clientID === 'string' ? asked.clientID.trim() : '';
	if (!clientID) error(400, 'name the app by its client id');
	const permission = connectedAppPermissionOf(asked?.permission);
	if (!permission) error(400, 'a connected app reads or writes');

	await keepConnectedAppPermission(member.record, member.memberID, clientID, permission);
	return json({ clientID, permission });
};

export const DELETE: RequestHandler = async ({ request, url, platform }) => {
	const member = await signedInMember(request, platform);
	const clientID = (url.searchParams.get('clientID') ?? '').trim();
	if (!clientID) error(400, 'name the app by its client id');

	await forgetConnectedAppPermission(member.record, member.memberID, clientID);
	return json({ forgotten: clientID });
};
