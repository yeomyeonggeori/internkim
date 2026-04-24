import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import type { OTAInfo } from '$lib/types';

export const GET: RequestHandler = async ({ platform }) => {
	const KV = platform?.env?.KV;
	if (!KV) throw error(500, 'KV not available');

	const latest = await KV.get('ota:latest', 'json') as OTAInfo | null;
	if (!latest) return json({ blueclaw: null, cli: null });

	return json(latest);
};
