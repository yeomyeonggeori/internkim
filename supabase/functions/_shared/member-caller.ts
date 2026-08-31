import { createClient, type SupabaseClient } from 'npm:@supabase/supabase-js@2';
import { refuse } from './http.ts';
import { serviceClient } from './service-client.ts';

export type CallingMember = {
	caller: SupabaseClient;
	record: SupabaseClient;
	memberID: string;
};

export async function callingMember(request: Request): Promise<CallingMember> {
	const authorization = request.headers.get('authorization') ?? '';
	const accessToken = authorization.startsWith('Bearer ') ? authorization.slice('Bearer '.length) : '';
	if (!accessToken) refuse(401, 'sign in first');

	const caller = createClient(
		Deno.env.get('SUPABASE_URL') ?? '',
		Deno.env.get('SUPABASE_ANON_KEY') ?? '',
		{
			auth: { autoRefreshToken: false, persistSession: false },
			global: { headers: { Authorization: `Bearer ${accessToken}` } }
		}
	);
	const { data: account } = await caller.auth.getUser();
	if (!account.user) refuse(401, 'sign in first');

	const member = await caller
		.from('member')
		.select('id')
		.eq('user_id', account.user.id)
		.maybeSingle<{ id: string }>();
	if (member.error) refuse(500, member.error.message);
	if (!member.data) refuse(403, 'refused');

	return { caller, record: serviceClient(), memberID: member.data.id };
}
