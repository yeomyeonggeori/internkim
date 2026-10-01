import { createClient, type SupabaseClient } from '@supabase/supabase-js';
import { error } from '@sveltejs/kit';
import { asMember, planeCredentialsOf } from './control-plane';
import type { Environment } from './agent-request';

export async function dataRoomCaller(request: Request, environment: Environment): Promise<SupabaseClient> {
	const plane = planeCredentialsOf(environment);
	if (!plane) error(503, 'the data room is not configured');
	const authorization = request.headers.get('authorization');
	if (!authorization) return createClient(plane.projectURL, plane.publishableKey, {
		auth: { persistSession: false, autoRefreshToken: false }
	});
	if (!authorization.startsWith('Bearer ')) error(401, 'use a signed-in session');
	const caller = asMember(plane, authorization.slice(7));
	const { data, error: refusal } = await caller.auth.getUser();
	if (refusal || !data.user) error(401, 'sign in first');
	return caller;
}
