import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { normalizeFleetID } from '$lib/device-auth';
import { kv } from '$lib/kv';

export const POST: RequestHandler = async ({ request, platform }) => {
	const KV = platform?.env?.KV;
	if (!KV) throw error(500, 'KV not available');

	const { fleet_id, component, version, success } = (await request.json()) as {
		fleet_id?: string;
		component: string;
		version: string;
		success: boolean;
	};
	const fleetID = normalizeFleetID(fleet_id ?? '');
	if (!fleetID || !component) throw error(400, 'fleet_id and component required');

	if (success) {
		const device = await kv.getDevice(KV, fleetID);
		if (device) {
			if (component === 'blueclaw') device.versions.blueclaw = version;
			if (component === 'cli') device.versions.cli = version;
			await kv.putDevice(KV, fleetID, device);
		}
	}

	return json({ ok: true });
};
