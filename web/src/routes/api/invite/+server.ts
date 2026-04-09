import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { kv } from '$lib/kv';

export const POST: RequestHandler = async ({ request, platform }) => {
	const env = platform?.env;
	if (!env?.KV) throw error(500, 'KV not available');

	const callerEmail = request.headers.get('Cf-Access-Authenticated-User-Email') ?? '';
	const { device_id } = (await request.json()) as { device_id: string };
	if (!device_id) throw error(400, 'device_id required');

	const users = await kv.getUsers(env.KV, device_id);
	if (users[0] !== callerEmail) throw error(403, 'Admin only');

	const KV = env.KV;

	const token = crypto.randomUUID().replace(/-/g, '').slice(0, 12);
	const expires_at = Date.now() + 24 * 60 * 60 * 1000;

	await kv.putInvite(KV, token, { device_id, expires_at });

	return json({ token, expires_at, url: `/invite/${token}` });
};
