import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { kv } from '$lib/kv';

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

	const device_id = url.searchParams.get('device_id');
	const admin_token = url.searchParams.get('admin_token') ?? '';
	const email = decodeURIComponent(params.email);
	if (!device_id || !email) throw error(400, 'device_id and email required');

	if (admin_token !== env.REGISTER_SECRET) throw error(403, 'Invalid admin token');

	const users = await kv.getUsers(env.KV, device_id);
	const filtered = users.filter((u) => u !== email);
	await kv.putUsers(env.KV, device_id, filtered);

	return json({ users: filtered }, { headers: corsHeaders });
};
