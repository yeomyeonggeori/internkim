import { env } from '$env/dynamic/private';
import { error, json } from '@sveltejs/kit';
import { planeCredentialsOf, sessionForHost } from '$lib/server/control-plane';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform }) => {
	const plane = planeCredentialsOf({ ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) });
	if (!plane) error(500, 'the control plane is not configured');

	const authorization = request.headers.get('authorization') ?? '';
	const apiKey = authorization.startsWith('Bearer ') ? authorization.slice('Bearer '.length) : '';
	if (!apiKey) error(401, 'no agent key');

	try {
		return json(await sessionForHost(plane, apiKey));
	} catch {
		error(403, 'refused');
	}
};
