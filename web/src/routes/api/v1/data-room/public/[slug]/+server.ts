import { error, json } from '@sveltejs/kit';
import { createClient } from '@supabase/supabase-js';
import { z } from 'zod';
import { environmentOf } from '$lib/server/agent-request';
import { dataRoomPlane } from '$lib/server/data-room-link-request';
import { isUsableCompanyAddress } from '$lib/company-path';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ platform, params }) => {
	if (!isUsableCompanyAddress(params.slug)) error(404, 'no company has this address');
	const plane = dataRoomPlane(environmentOf(platform));
	const anyone = createClient(plane.projectURL, plane.publishableKey, { auth: { persistSession: false, autoRefreshToken: false } });
	const { data, error: refusal } = await anyone.rpc('data_room_public_company', { target_slug: params.slug });
	if (refusal) error(502, 'the public page could not be read');
	if (!data) error(404, 'no company has this address');
	return json({ companyID: z.guid().parse(data) });
};
