import { env } from '$env/dynamic/private';
import { error, json } from '@sveltejs/kit';
import { agentOfKey, controlPlane } from '$lib/server/control-plane';
import { keepMemberCredential, membersOfCompanyByExternalID } from '$lib/server/member-credential';
import type { RequestHandler } from './$types';

type OfferedCredential = { externalID?: unknown; secret?: unknown };
type CredentialsRequest = { kind?: unknown; credentials?: unknown };

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = { ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) };
	const projectURL = environment.SUPABASE_URL ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? '';
	if (!projectURL || !serviceRoleKey) error(500, 'the control plane is not configured');

	const authorization = request.headers.get('authorization') ?? '';
	const apiKey = authorization.startsWith('Bearer ') ? authorization.slice('Bearer '.length) : '';
	if (!apiKey) error(401, 'no agent key');

	const client = controlPlane({ projectURL, serviceRoleKey });
	const agent = await agentOfKey(client, apiKey);
	if (!agent) error(403, 'refused');

	const body = (await request.json().catch(() => ({}))) as CredentialsRequest;
	const kind = typeof body.kind === 'string' && body.kind.trim() ? body.kind.trim() : 'mattermost';
	const offered = Array.isArray(body.credentials) ? (body.credentials as OfferedCredential[]) : [];

	const memberOf = await membersOfCompanyByExternalID(client, agent.companyID, kind);
	let kept = 0;
	for (const credential of offered) {
		if (typeof credential.externalID !== 'string' || typeof credential.secret !== 'string') continue;
		const memberID = memberOf.get(credential.externalID);
		if (!memberID) continue;
		await keepMemberCredential(client, memberID, {
			kind,
			externalID: credential.externalID,
			secret: credential.secret,
		});
		kept += 1;
	}

	return json({ kept, offered: offered.length, linked: memberOf.size });
};
