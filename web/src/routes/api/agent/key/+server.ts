import { env } from '$env/dynamic/private';
import { error, json } from '@sveltejs/kit';
import { companyOfFleet, controlPlane, replaceFleetAgentKey } from '$lib/server/control-plane';
import { isNodeRequest, normalizeFleetID } from '$lib/device-auth';
import { kv } from '$lib/kv';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform, url }) => {
	const environment = { ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) };
	const projectURL = environment.SUPABASE_URL ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? '';
	if (!projectURL || !serviceRoleKey) error(500, 'the control plane is not configured');

	const store = platform?.env?.KV;
	if (!store) error(500, 'the fleet register is not available');

	const fleetID = normalizeFleetID(url.searchParams.get('fleet_id') ?? '');
	if (!fleetID) error(400, 'fleet_id required');

	const device = await kv.getDevice(store, fleetID);
	if (!device || !(await isNodeRequest(request, device, fleetID))) error(403, 'not this fleet');

	const client = controlPlane({ projectURL, serviceRoleKey });
	const companyID = await companyOfFleet(client, fleetID);
	if (!companyID) error(404, 'this fleet belongs to no company yet');

	const issued = await replaceFleetAgentKey(client, companyID, fleetID);
	return json({ companyID: issued.companyID, apiKey: issued.apiKey });
};
