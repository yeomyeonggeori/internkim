import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { isNodeRequest, normalizeFleetID } from '$lib/device-auth';
import { fleetUserRecords, withdrawFleetUser } from '$lib/server/fleet-user-directory';
import { adminEmailsOf, askedDirectory, isAdminRequest, usersResponse } from '$lib/server/fleet-user-request';

const corsHeaders = {
	'Access-Control-Allow-Origin': '*',
	'Access-Control-Allow-Methods': 'GET, POST, DELETE, OPTIONS',
	'Access-Control-Allow-Headers': 'Content-Type, X-INTERNKIM-FLEET-ID, X-INTERNKIM-FLEET-SECRET, X-INTERNKIM-DEVICE-ID, X-INTERNKIM-DEVICE-SECRET'
};

export const OPTIONS: RequestHandler = async () => {
	return new Response(null, { headers: corsHeaders });
};

export const DELETE: RequestHandler = async ({ params, request, url, platform }) => {
	const fleetID = normalizeFleetID(url.searchParams.get('fleet_id') ?? '');
	const adminToken = url.searchParams.get('admin_token') ?? '';
	const email = decodeURIComponent(params.email).trim().toLowerCase();
	if (!fleetID || !email) throw error(400, 'fleet_id and email required');

	const { device, directory } = await askedDirectory(platform, fleetID);
	const records = await fleetUserRecords(directory);
	const isAuthorizedNode = await isNodeRequest(request, device, fleetID);
	if (!isAuthorizedNode && !isAdminRequest(request, device, adminEmailsOf(records), adminToken, platform?.env?.INTERNKIM_REGISTER_SECRET ?? '')) {
		throw error(403, 'Admin only');
	}
	const record = records.find((item) => item.email === email);
	if (!isAuthorizedNode && record?.role === 'admin' && adminEmailsOf(records).length <= 1) {
		throw error(400, 'Cannot remove the last admin user');
	}

	const remaining = await withdrawFleetUser(directory, email);
	return json(await usersResponse(remaining), { headers: corsHeaders });
};
