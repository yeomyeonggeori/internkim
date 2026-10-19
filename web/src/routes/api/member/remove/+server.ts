import { asMember, controlPlane, planeCredentialsOf } from '$lib/server/control-plane';
import { removeMemberAsAdministrator } from '$lib/server/member-administration';
import { RecordRefusedTheWrite } from '$lib/server/public-api/record/tasks';
import { env } from '$env/dynamic/private';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { signedInAccessTokenOf } from '$lib/server/member-request';

export const POST: RequestHandler = async ({ request, platform }) => {
	const plane = planeCredentialsOf({ ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) });
	if (!plane) error(500, 'the central plane is not configured');

	const accessToken = await signedInAccessTokenOf(request, plane);
	const body = (await request.json().catch(() => ({}))) as { memberID?: unknown; purge?: unknown };
	const memberID = typeof body.memberID === 'string' ? body.memberID.trim() : '';

	try {
		return json(
			await removeMemberAsAdministrator(asMember(plane, accessToken), controlPlane(plane), memberID, {
				purge: body.purge === true
			})
		);
	} catch (refusal) {
		if (refusal instanceof RecordRefusedTheWrite) error(refusal.status, refusal.message);
		throw refusal;
	}
};
