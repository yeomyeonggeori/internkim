import { createClient } from '@supabase/supabase-js';
import { error, json } from '@sveltejs/kit';
import { z } from 'zod';
import { environmentOf } from '$lib/server/agent-request';
import { planeCredentialsOf } from '$lib/server/control-plane';
import { dataRoomCaller } from '$lib/server/data-room-request';
import { defaultZone } from '$lib/server/fleet-domain';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform, params, url }) => {
	if (!z.string().uuid().safeParse(params.shareID).success) error(404, 'invitation not found');
	const environment = environmentOf(platform);
	const caller = await dataRoomCaller(request, environment);
	const { data, error: refusal } = await caller.from('data_room_share')
		.select('company_id,email,expires_at,revoked_at').eq('id', params.shareID).eq('audience', 'email').maybeSingle();
	if (refusal || !data) error(404, 'invitation not found');
	const invitation = z.object({ company_id: z.string(), email: z.string(),
		expires_at: z.string().nullable(), revoked_at: z.string().nullable() }).parse(data);
	const administrator = await caller.rpc('data_room_administrator', { target_company: invitation.company_id });
	if (administrator.error || administrator.data !== true) error(403, 'only an administrator sends invitations');
	if (invitation.revoked_at || (invitation.expires_at && new Date(invitation.expires_at) <= new Date())) error(403, 'invitation is no longer active');
	const plane = planeCredentialsOf(environment);
	if (!plane) error(503, 'the data room is not configured');
	const origin = ['localhost', '127.0.0.1'].includes(url.hostname) ? url.origin : `https://${environment.CLOUDFLARE_DOMAIN || defaultZone}`;
	const sender = createClient(plane.projectURL, plane.publishableKey, { auth: { persistSession: false, autoRefreshToken: false } });
	const sent = await sender.auth.signInWithOtp({ email: invitation.email,
		options: { shouldCreateUser: true, emailRedirectTo: `${origin}/share/invitations/${params.shareID}` } });
	if (sent.error) error(sent.error.status ?? 502, sent.error.message);
	return json({ sent: true });
};
