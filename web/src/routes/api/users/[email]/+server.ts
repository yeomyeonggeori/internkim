import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { syncFleetAccessPolicies } from '$lib/fleet-access';
import { isNodeRequest, normalizeFleetID } from '$lib/device-auth';
import { adminEmails, kv, userEmails } from '$lib/kv';
import type { Device, UserRecord } from '$lib/types';

const corsHeaders = {
	'Access-Control-Allow-Origin': '*',
	'Access-Control-Allow-Methods': 'GET, POST, DELETE, OPTIONS',
	'Access-Control-Allow-Headers': 'Content-Type, X-INTERNKIM-FLEET-ID, X-INTERNKIM-FLEET-SECRET, X-INTERNKIM-DEVICE-ID, X-INTERNKIM-DEVICE-SECRET'
};

export const OPTIONS: RequestHandler = async () => {
	return new Response(null, { headers: corsHeaders });
};

function normalizeEmail(email: string): string {
	return email.trim().toLowerCase();
}

async function usersRevision(records: UserRecord[]): Promise<string> {
	const encodedUsers = new TextEncoder().encode(JSON.stringify(records));
	const digest = await crypto.subtle.digest('SHA-256', encodedUsers);
	return Array.from(new Uint8Array(digest))
		.map((byte) => byte.toString(16).padStart(2, '0'))
		.join('');
}

function callerEmail(request: Request): string {
	return normalizeEmail(request.headers.get('Cf-Access-Authenticated-User-Email') ?? '');
}

function isAdminRequest(request: Request, device: Device, adminUsers: string[], adminToken: string, registerSecret: string): boolean {
	if (adminToken && adminToken === registerSecret) return true;
	const authorizedAdmins = adminUsers.length > 0 ? adminUsers : [normalizeEmail(device.admin_email)];
	return authorizedAdmins.includes(callerEmail(request));
}

export const DELETE: RequestHandler = async ({ params, request, url, platform }) => {
	const env = platform?.env;
	if (!env?.KV) throw error(500, 'KV not available');

	const fleetID = normalizeFleetID(url.searchParams.get('fleet_id') ?? '');
	const admin_token = url.searchParams.get('admin_token') ?? '';
	const email = normalizeEmail(decodeURIComponent(params.email));
	if (!fleetID || !email) throw error(400, 'fleet_id and email required');

	const device = await kv.getDevice(env.KV, fleetID);
	if (!device) throw error(404, 'Fleet not found');

	const records = await kv.getUserRecords(env.KV, fleetID);
	const isAuthorizedNode = await isNodeRequest(request, device, fleetID);
	if (!isAuthorizedNode && !isAdminRequest(request, device, adminEmails(records), admin_token, env.INTERNKIM_REGISTER_SECRET)) {
		throw error(403, 'Admin only');
	}
	const record = records.find((item) => item.email === email);
	if (!isAuthorizedNode && record?.role === 'admin' && adminEmails(records).length <= 1) {
		throw error(400, 'Cannot remove the last admin user');
	}

	const filtered = records.filter((item) => item.email !== email);
	await kv.putUserRecords(env.KV, fleetID, filtered);
	const syncedDevice = await syncFleetAccessPolicies(env, fleetID, device, filtered);
	if (syncedDevice.access_app_id !== device.access_app_id || syncedDevice.access_policy_id !== device.access_policy_id) {
		await kv.putDevice(env.KV, fleetID, syncedDevice);
	}

	return json({ users: userEmails(filtered), records: filtered, revision: await usersRevision(filtered) }, { headers: corsHeaders });
};
