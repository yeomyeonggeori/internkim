import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { normalizeFleetID } from '$lib/device-auth';
import { kv } from '$lib/kv';
import { environmentOfPlatform, fleetDirectory } from '$lib/server/agent-request';
import { fleetUserRecords } from '$lib/server/fleet-user-directory';

export const POST: RequestHandler = async ({ request, platform }) => {
	const env = platform?.env;
	if (!env?.KV) throw error(500, 'KV not available');

	const callerEmail = request.headers.get('Cf-Access-Authenticated-User-Email') ?? '';
	const { fleet_id } = (await request.json()) as { fleet_id?: string };
	const fleetID = normalizeFleetID(fleet_id ?? '');
	if (!fleetID) throw error(400, 'fleet_id required');

	const directory = await fleetDirectory(environmentOfPlatform(env), fleetID);
	if (!directory) throw error(404, 'this fleet belongs to no company yet');
	const admins = (await fleetUserRecords(directory))
		.filter((record) => record.role === 'admin')
		.map((record) => record.email);
	if (!admins.includes(callerEmail.trim().toLowerCase())) throw error(403, 'Admin only');

	const KV = env.KV;

	const token = crypto.randomUUID().replace(/-/g, '').slice(0, 12);
	const expires_at = Date.now() + 24 * 60 * 60 * 1000;

	await kv.putInvite(KV, token, { fleet_id: fleetID, expires_at });

	return json({ token, expires_at, url: `/invite/${token}` });
};
