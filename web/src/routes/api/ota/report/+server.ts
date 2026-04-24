import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { kv } from '$lib/kv';

export const POST: RequestHandler = async ({ request, platform }) => {
	const KV = platform?.env?.KV;
	if (!KV) throw error(500, 'KV not available');

	const { device_id, component, version, success } = (await request.json()) as {
		device_id: string;
		component: string;
		version: string;
		success: boolean;
	};
	if (!device_id || !component) throw error(400, 'device_id and component required');

	if (success) {
		const device = await kv.getDevice(KV, device_id);
		if (device) {
			if (component === 'blueclaw') device.versions.blueclaw = version;
			if (component === 'cli') device.versions.cli = version;
			await kv.putDevice(KV, device_id, device);
		}
	}

	return json({ ok: true });
};
