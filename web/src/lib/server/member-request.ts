import { error } from '@sveltejs/kit';
import type { SupabaseClient } from '@supabase/supabase-js';
import { asMember, controlPlane } from './control-plane';
import type { Environment } from './agent-request';

export type CallingMember = {
	caller: SupabaseClient;
	record: SupabaseClient;
	memberID: string;
	email: string;
};

export async function callingMember(request: Request, environment: Environment): Promise<CallingMember> {
	const projectURL = environment.SUPABASE_URL ?? '';
	const publishableKey = environment.SUPABASE_PUBLISHABLE_KEY ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? '';
	if (!projectURL || !publishableKey || !serviceRoleKey) {
		error(500, 'the control plane is not configured');
	}

	const authorization = request.headers.get('authorization') ?? '';
	const accessToken = authorization.startsWith('Bearer ') ? authorization.slice('Bearer '.length) : '';
	if (!accessToken) error(401, 'sign in first');

	const caller = asMember({ projectURL, publishableKey }, accessToken);
	const { data: account } = await caller.auth.getUser();
	if (!account.user) error(401, 'sign in first');

	const member = await caller
		.from('member')
		.select('id, email')
		.eq('user_id', account.user.id)
		.maybeSingle<{ id: string; email: string | null }>();
	if (member.error) error(500, member.error.message);
	if (!member.data) error(403, 'refused');

	return {
		caller,
		record: controlPlane({ projectURL, serviceRoleKey }),
		memberID: member.data.id,
		email: member.data.email ?? account.user.email ?? ''
	};
}
