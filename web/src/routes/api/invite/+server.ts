import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { kv } from '$lib/kv';

export const POST: RequestHandler = async ({ request, platform }) => {
	const KV = platform?.env?.KV;
	if (!KV) throw error(500, 'KV not available');

	// TODO: verify admin JWT
	const { device_id } = (await request.json()) as { device_id: string };
	if (!device_id) throw error(400, 'device_id required');

	const token = crypto.randomUUID().replace(/-/g, '').slice(0, 12);
	const expires_at = Date.now() + 24 * 60 * 60 * 1000;

	await kv.putInvite(KV, token, { device_id, expires_at });

	return json({ token, expires_at, url: `/invite/${token}` });
};
