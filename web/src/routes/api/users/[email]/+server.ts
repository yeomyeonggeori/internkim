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

export const DELETE: RequestHandler = async ({ params, url, platform }) => {
	const env = platform?.env;
	if (!env?.KV) throw error(500, 'KV not available');

	// TODO: verify admin JWT
	const device_id = url.searchParams.get('device_id');
	const email = decodeURIComponent(params.email);
	if (!device_id || !email) throw error(400, 'device_id and email required');

	const device = await kv.getDevice(env.KV, device_id);
	if (!device) throw error(404, 'Device not found');

	const users = await kv.getUsers(env.KV, device_id);
	const filtered = users.filter((u) => u !== email);
	await kv.putUsers(env.KV, device_id, filtered);

	await addEmailToPolicy(
		{
			CF_API_TOKEN: env.CF_API_TOKEN,
			CF_ACCOUNT_ID: env.CF_ACCOUNT_ID,
			CF_ZONE_ID: env.CF_ZONE_ID,
			CF_DOMAIN: env.CF_DOMAIN
		},
		device.access_app_id,
		filtered
	);

	return json({ users: filtered }, { headers: corsHeaders });
};
