import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { kv } from '$lib/kv';
import { createTunnel, configureTunnel, createDNSRecord } from '$lib/cloudflare';
import type { Device } from '$lib/types';

export const POST: RequestHandler = async ({ request, platform }) => {
	const env = platform?.env;
	if (!env?.KV) throw error(500, 'KV not available');

	const auth = request.headers.get('authorization');
	if (auth !== `Bearer ${env.REGISTER_SECRET}`) {
		throw error(401, 'Invalid registration secret');
	}

	const { device_id, admin_email } = (await request.json()) as {
		device_id: string;
		admin_email: string;
	};
	if (!device_id || !admin_email) throw error(400, 'device_id and admin_email required');

	const existing = await kv.getDevice(env.KV, device_id);
	if (existing) throw error(409, 'Device already registered');

	const cfEnv = {
		CF_API_TOKEN: env.CF_API_TOKEN,
		CF_ACCOUNT_ID: env.CF_ACCOUNT_ID,
		CF_ZONE_ID: env.CF_ZONE_ID,
		CF_DOMAIN: env.CF_DOMAIN
	};

	const { tunnelId, tunnelToken } = await createTunnel(cfEnv, device_id);
	// Tunnel points to Mattermost — Mattermost handles all auth
	await configureTunnel(cfEnv, tunnelId, device_id);
	const dnsRecordId = await createDNSRecord(cfEnv, tunnelId, device_id);

	const device: Device = {
		device_id,
		tunnel_id: tunnelId,
		tunnel_token: tunnelToken,
		dns_record_id: dnsRecordId,
		admin_email,
		created_at: new Date().toISOString()
	};

	await kv.putDevice(env.KV, device_id, device);

	return json({
		device_id,
		tunnel_token: tunnelToken,
		url: `https://${device_id}.${env.CF_DOMAIN}`
	});
};
