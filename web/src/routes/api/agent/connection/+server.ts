import { callingAgent, environmentOf } from '$lib/server/agent-request';
import { companyConnections, secretOfConnection } from '$lib/server/company-credential';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ request, platform, url }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));

	const kind = url.searchParams.get('kind') ?? '';
	if (!kind) error(400, 'which connection');

	const connections = await companyConnections(client, companyID);
	const connection = connections.find((entry) => entry.kind === kind);
	if (!connection) error(404, `this company has no ${kind} connection`);

	const secret = await secretOfConnection(client, companyID, kind);
	return json({ host: connection.host, settings: connection.settings, secret });
};
