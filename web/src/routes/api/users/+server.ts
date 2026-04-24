import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { syncAccessPolicyEmails } from '$lib/cloudflare';
import { isBoardRequest, normalizeDeviceID } from '$lib/device-auth';
import { kv } from '$lib/kv';
import type { Device } from '$lib/types';

const corsHeaders = {
	'Access-Control-Allow-Origin': '*',
	'Access-Control-Allow-Methods': 'GET, POST, DELETE, OPTIONS',
	'Access-Control-Allow-Headers': 'Content-Type, X-InternKim-Device-ID, X-InternKim-Device-Secret'
};

export const OPTIONS: RequestHandler = async () => {
	return new Response(null, { headers: corsHeaders });
};

function normalizeEmail(email: string): string {
	return email.trim().toLowerCase();
}

async function usersRevision(users: string[]): Promise<string> {
	const encodedUsers = new TextEncoder().encode(JSON.stringify(users));
	const digest = await crypto.subtle.digest('SHA-256', encodedUsers);
	return Array.from(new Uint8Array(digest))
		.map((byte) => byte.toString(16).padStart(2, '0'))
		.join('');
}

function cfEnv(env: App.Platform['env']) {
	return {
		CF_API_TOKEN: env.CF_API_TOKEN,
		CF_ACCOUNT_ID: env.CF_ACCOUNT_ID,
		CF_ZONE_ID: env.CF_ZONE_ID,
		CF_DOMAIN: env.CF_DOMAIN
	};
}

function callerEmail(request: Request): string {
	return normalizeEmail(request.headers.get('Cf-Access-Authenticated-User-Email') ?? '');
}

function isAdminRequest(request: Request, device: Device, users: string[], adminToken: string, registerSecret: string): boolean {
	if (adminToken && adminToken === registerSecret) return true;
	const adminEmail = users[0] ?? normalizeEmail(device.admin_email);
	return callerEmail(request) === adminEmail;
}

export const GET: RequestHandler = async ({ request, url, platform }) => {
	const env = platform?.env;
	if (!env?.KV) throw error(500, 'KV not available');

	const device_id = normalizeDeviceID(url.searchParams.get('device_id') ?? '');
	if (!device_id) throw error(400, 'device_id required');

	const device = await kv.getDevice(env.KV, device_id);
	if (!device) throw error(404, 'Device not found');

	const users = await kv.getUsers(env.KV, device_id);
	const admin_token = url.searchParams.get('admin_token') ?? '';
	const isAuthorizedBoard = await isBoardRequest(request, device, device_id);
	if (!isAuthorizedBoard && !isAdminRequest(request, device, users, admin_token, env.INTERNKIM_REGISTER_SECRET)) {
		throw error(403, 'Admin only');
	}

	return json({ users, revision: await usersRevision(users) }, { headers: corsHeaders });
};

export const POST: RequestHandler = async ({ request, platform }) => {
	const env = platform?.env;
	if (!env?.KV) throw error(500, 'KV not available');

	const { device_id, email, admin_token } = (await request.json()) as {
		device_id: string;
		email: string;
		admin_token: string;
	};
	const deviceID = normalizeDeviceID(device_id ?? '');
	if (!deviceID || !email) throw error(400, 'device_id and email required');

	const device = await kv.getDevice(env.KV, deviceID);
	if (!device) throw error(404, 'Device not found');

	const users = await kv.getUsers(env.KV, deviceID);
	if (!isAdminRequest(request, device, users, admin_token, env.INTERNKIM_REGISTER_SECRET)) {
		throw error(403, 'Admin only');
	}

	const normalizedEmail = normalizeEmail(email);
	if (!users.includes(normalizedEmail)) {
		users.push(normalizedEmail);
		await kv.putUsers(env.KV, deviceID, users);
	}
	if (device.access_app_id) {
		await syncAccessPolicyEmails(cfEnv(env), device.access_app_id, users);
	}

	return json({ users, revision: await usersRevision(users) }, { headers: corsHeaders });
};
