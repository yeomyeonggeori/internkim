import { env } from '$env/dynamic/private';
import { asMember, claimMemberFor, controlPlane, planeCredentialsOf } from '$lib/server/control-plane';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { memberAccessTokenOf } from '$lib/server/member-request';

export const GET: RequestHandler = async ({ request, platform }) => {
	const plane = planeCredentialsOf({ ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) });
	if (!plane) error(500, 'the central plane is not configured');

	const { accessToken } = await memberAccessTokenOf(request, plane);

	const { data: account } = await asMember(plane, accessToken).auth.getUser();
	const email = account.user?.email?.trim().toLowerCase();
	if (!account.user || !email) error(401, 'sign in first');

	const claimed = await claimMemberFor(controlPlane(plane), account.user.id, email);
	if (!claimed) return json({ member: null });
	return json({ member: claimed });
};
