import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { kv } from '$lib/kv';
import { addEmailToPolicy } from '$lib/cloudflare';

const corsHeaders = {
	'Access-Control-Allow-Origin': '*',
	'Access-Control-Allow-Methods': 'GET, POST, DELETE, OPTIONS',
	'Access-Control-Allow-Headers': 'Content-Type'
};

export const OPTIONS: RequestHandler = async () => {
	return new Response(null, { headers: corsHeaders });
};

export const GET: RequestHandler = async ({ request, url, platform }) => {
	const env = platform?.env;
	if (!env?.KV) throw error(500, 'KV not available');

	const device_id = url.searchParams.get('device_id');
	if (!device_id) throw error(400, 'device_id required');

	const callerEmail = request.headers.get('Cf-Access-Authenticated-User-Email') ?? '';
	const users = await kv.getUsers(env.KV, device_id);

	// First visitor: auto-register as admin
	if (users.length === 0 && callerEmail) {
		const device = await kv.getDevice(env.KV, device_id);
		if (device) {
			users.push(callerEmail);
			await kv.putUsers(env.KV, device_id, users);
			await addEmailToPolicy(
				{
					CF_API_TOKEN: env.CF_API_TOKEN,
					CF_ACCOUNT_ID: env.CF_ACCOUNT_ID,
					CF_ZONE_ID: env.CF_ZONE_ID,
					CF_DOMAIN: env.CF_DOMAIN
				},
				device.access_app_id,
				users
			);
		}
	}

	const isAdmin = users[0] === callerEmail;
	return json({ users, isAdmin }, { headers: corsHeaders });
};

export const POST: RequestHandler = async ({ request, platform }) => {
	const env = platform?.env;
	if (!env?.KV) throw error(500, 'KV not available');

	const callerEmail = request.headers.get('Cf-Access-Authenticated-User-Email') ?? '';
	const { device_id, email } = (await request.json()) as { device_id: string; email: string };
	if (!device_id || !email) throw error(400, 'device_id and email required');

	const users = await kv.getUsers(env.KV, device_id);
	if (users[0] !== callerEmail) throw error(403, 'Admin only');

	const device = await kv.getDevice(env.KV, device_id);
	if (!device) throw error(404, 'Device not found');

	if (!users.includes(email)) {
		users.push(email);
		await kv.putUsers(env.KV, device_id, users);
		await addEmailToPolicy(
			{
				CF_API_TOKEN: env.CF_API_TOKEN,
				CF_ACCOUNT_ID: env.CF_ACCOUNT_ID,
				CF_ZONE_ID: env.CF_ZONE_ID,
				CF_DOMAIN: env.CF_DOMAIN
			},
			device.access_app_id,
			users
		);
	}

	return json({ users }, { headers: corsHeaders });
};
