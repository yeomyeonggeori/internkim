import type { SupabaseClient } from '@supabase/supabase-js';
import { error } from '@sveltejs/kit';
import { asMember, planeCredentialsOf } from './control-plane';
import type { Environment } from './agent-request';

export async function signedInGuest(request: Request, environment: Environment): Promise<SupabaseClient> {
	const plane = planeCredentialsOf(environment);
	if (!plane) error(503, 'the central plane is not configured');
	const authorization = request.headers.get('authorization') ?? '';
	if (!authorization.startsWith('Bearer ')) error(401, 'sign in first');
	const caller = asMember(plane, authorization.slice('Bearer '.length));
	const { data, error: refusal } = await caller.auth.getUser();
	if (refusal || !data.user) error(401, 'sign in first');
	return caller;
}
