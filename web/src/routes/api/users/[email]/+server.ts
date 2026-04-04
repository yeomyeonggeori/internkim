import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { kv } from '$lib/kv';

export const DELETE: RequestHandler = async ({ params, url, platform }) => {
	const KV = platform?.env?.KV;
	if (!KV) throw error(500, 'KV not available');

	// TODO: verify admin JWT
	const device_id = url.searchParams.get('device_id');
	const email = decodeURIComponent(params.email);
	if (!device_id || !email) throw error(400, 'device_id and email required');

	const users = await kv.getUsers(KV, device_id);
	const filtered = users.filter((u) => u !== email);
	await kv.putUsers(KV, device_id, filtered);

	// TODO: remove email from CF Access policy

	return json({ users: filtered });
};
