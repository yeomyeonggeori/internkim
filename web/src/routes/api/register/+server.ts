import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { kv } from '$lib/kv';
import type { Device } from '$lib/types';

export const POST: RequestHandler = async ({ request, platform }) => {
	const KV = platform?.env?.KV;
	if (!KV) throw error(500, 'KV not available');

	const { device_id } = (await request.json()) as { device_id: string };
	if (!device_id) throw error(400, 'device_id required');

	const existing = await kv.getDevice(KV, device_id);
	if (existing) throw error(409, 'Device already registered');

	// TODO: Cloudflare API calls to create tunnel + DNS + Access policy
	const device: Device = {
		device_id,
		tunnel_id: '',
		tunnel_token: '',
		dns_record_id: '',
		admin_email: '',
		created_at: new Date().toISOString(),
		versions: { picoclaw: '0.2.5', cli: '0.0.1' }
	};

	await kv.putDevice(KV, device_id, device);

	return json({ device_id, tunnel_token: device.tunnel_token, url: `https://${device_id}.quickclaw.io` });
};
