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

export const GET: RequestHandler = async ({ url, platform }) => {
	const env = platform?.env;
	if (!env?.KV) throw error(500, 'KV not available');

	const device_id = url.searchParams.get('device_id');
	if (!device_id) throw error(400, 'device_id required');

	const users = await kv.getUsers(env.KV, device_id);
	return json({ users }, { headers: corsHeaders });
};

export const POST: RequestHandler = async ({ request, platform }) => {
	const env = platform?.env;
	if (!env?.KV) throw error(500, 'KV not available');

	const { device_id, email, admin_token } = (await request.json()) as {
		device_id: string;
		email: string;
		admin_token: string;
	};
	if (!device_id || !email) throw error(400, 'device_id and email required');

	const device = await kv.getDevice(env.KV, device_id);
	if (!device) throw error(404, 'Device not found');
	if (admin_token !== env.REGISTER_SECRET) throw error(403, 'Invalid admin token');

	const users = await kv.getUsers(env.KV, device_id);
	if (!users.includes(email)) {
		users.push(email);
		await kv.putUsers(env.KV, device_id, users);
	}

	return json({ users }, { headers: corsHeaders });
};
