import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { kv } from '$lib/kv';

export const GET: RequestHandler = async ({ url, platform }) => {
	const KV = platform?.env?.KV;
	if (!KV) throw error(500, 'KV not available');

	// TODO: verify admin JWT
	const device_id = url.searchParams.get('device_id');
	if (!device_id) throw error(400, 'device_id required');

	const users = await kv.getUsers(KV, device_id);
	return json({ users });
};

export const POST: RequestHandler = async ({ request, platform }) => {
	const KV = platform?.env?.KV;
	if (!KV) throw error(500, 'KV not available');

	// TODO: verify admin JWT
	const { device_id, email } = (await request.json()) as { device_id: string; email: string };
	if (!device_id || !email) throw error(400, 'device_id and email required');

	const users = await kv.getUsers(KV, device_id);
	if (!users.includes(email)) {
		users.push(email);
		await kv.putUsers(KV, device_id, users);
	}

	// TODO: add email to CF Access policy

	return json({ users });
};
