import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { kv } from '$lib/kv';
import { addEmailToPolicy } from '$lib/cloudflare';

export const GET: RequestHandler = async ({ url, platform }) => {
	const env = platform?.env;
	if (!env?.KV) throw error(500, 'KV not available');

	// TODO: verify admin JWT
	const device_id = url.searchParams.get('device_id');
	if (!device_id) throw error(400, 'device_id required');

	const users = await kv.getUsers(env.KV, device_id);
	return json({ users });
};

export const POST: RequestHandler = async ({ request, platform }) => {
	const env = platform?.env;
	if (!env?.KV) throw error(500, 'KV not available');

	// TODO: verify admin JWT
	const { device_id, email } = (await request.json()) as { device_id: string; email: string };
	if (!device_id || !email) throw error(400, 'device_id and email required');

	const device = await kv.getDevice(env.KV, device_id);
	if (!device) throw error(404, 'Device not found');

	const users = await kv.getUsers(env.KV, device_id);
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

	return json({ users });
};
