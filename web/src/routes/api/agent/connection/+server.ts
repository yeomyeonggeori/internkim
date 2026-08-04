import { env } from '$env/dynamic/private';
import { agentOfKey, controlPlane } from '$lib/server/control-plane';
import { companyConnections, secretOfConnection } from '$lib/server/company-credential';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ request, platform, url }) => {
	const environment = { ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) };
	const projectURL = environment.SUPABASE_URL ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? '';
	if (!projectURL || !serviceRoleKey) error(500, 'the central plane is not configured');

	const authorization = request.headers.get('authorization') ?? '';
	const apiKey = authorization.startsWith('Bearer ') ? authorization.slice('Bearer '.length) : '';
	if (!apiKey) error(401, 'no agent key');

	const kind = url.searchParams.get('kind') ?? '';
	if (!kind) error(400, 'which connection');

	const client = controlPlane({ projectURL, serviceRoleKey });
	const agent = await agentOfKey(client, apiKey);
	if (!agent) error(403, 'refused');

	const connections = await companyConnections(client, agent.companyID);
	const connection = connections.find((entry) => entry.kind === kind);
	if (!connection) error(404, `this company has no ${kind} connection`);

	const secret = await secretOfConnection(client, agent.companyID, kind);
	return json({ host: connection.host, settings: connection.settings, secret });
};
