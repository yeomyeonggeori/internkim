import { env } from '$env/dynamic/private';
import { error, json } from '@sveltejs/kit';
import { sessionForHost } from '$lib/server/control-plane';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = { ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) };
	const projectURL = environment.SUPABASE_URL ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? '';
	if (!projectURL || !serviceRoleKey) error(500, 'the control plane is not configured');

	const authorization = request.headers.get('authorization') ?? '';
	const apiKey = authorization.startsWith('Bearer ') ? authorization.slice('Bearer '.length) : '';
	if (!apiKey) error(401, 'no agent key');

	try {
		return json(await sessionForHost({ projectURL, serviceRoleKey }, apiKey));
	} catch {
		error(403, 'refused');
	}
};
