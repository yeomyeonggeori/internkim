import { adminCallerOf, asMember, controlPlane, planeCredentialsOf, removeMember } from '$lib/server/control-plane';
import { env } from '$env/dynamic/private';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { signedInAccessTokenOf } from '$lib/server/member-request';

export const POST: RequestHandler = async ({ request, platform }) => {
	const plane = planeCredentialsOf({ ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) });
	if (!plane) error(500, 'the central plane is not configured');

	const accessToken = await signedInAccessTokenOf(request, plane);

	const caller = await adminCallerOf(asMember(plane, accessToken));
	if (!caller) error(403, 'only an admin removes people');

	const body = (await request.json().catch(() => ({}))) as { memberID?: unknown; purge?: unknown };
	const memberID = typeof body.memberID === 'string' ? body.memberID.trim() : '';
	if (!memberID) error(400, 'which member to remove');
	if (memberID === caller.memberID) error(400, 'an admin does not remove themselves');

	const client = controlPlane(plane);
	const { wasRemoved } = await removeMember(client, caller.companyID, memberID, { purge: body.purge === true });
	return json({ memberID, wasRemoved });
};
