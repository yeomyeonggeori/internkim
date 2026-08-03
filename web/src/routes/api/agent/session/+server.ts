// An agent asks: "somebody with this platform identity spoke — let me act as them."
// It proves which company it belongs to, and gets back that member's session. The
// service key never leaves here.
import { error, json } from '@sveltejs/kit';
import { sessionForPlatformIdentity } from '$lib/server/control-plane';
import type { RequestHandler } from './$types';

type SessionRequest = { kind?: unknown; externalID?: unknown };

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = (platform?.env ?? process.env) as Record<string, string | undefined>;
	const projectURL = environment.SUPABASE_URL ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? '';
	if (!projectURL || !serviceRoleKey) error(500, 'the control plane is not configured');

	const authorization = request.headers.get('authorization') ?? '';
	const apiKey = authorization.startsWith('Bearer ') ? authorization.slice('Bearer '.length) : '';
	if (!apiKey) error(401, 'no agent key');

	const body = (await request.json().catch(() => ({}))) as SessionRequest;
	const kind = typeof body.kind === 'string' ? body.kind.trim() : '';
	const externalID = typeof body.externalID === 'string' ? body.externalID.trim() : '';
	if (!kind || !externalID) error(400, 'kind and externalID are required');

	try {
		const session = await sessionForPlatformIdentity(
			{ projectURL, serviceRoleKey },
			apiKey,
			kind,
			externalID,
		);
		return json(session);
	} catch (errorValue) {
		// Which of "wrong key", "unknown identity" and "another company's member"
		// happened is not the caller's business; saying so would let an agent enumerate.
		error(403, 'refused');
	}
};
