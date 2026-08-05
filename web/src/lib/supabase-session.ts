import { isSupabaseConfigured, supabase } from '$lib/supabase';
import { signedOutSession, type WebAuthSession } from '$lib/web-auth-session';

export { isSupabaseConfigured };

export async function supabaseWebAuthSession(returnPath: string): Promise<WebAuthSession> {
	const { data } = await supabase().auth.getSession();
	const email = data.session?.user.email;
	if (!email) return signedOutSession(returnPath, false);
	return {
		authenticated: true,
		email,
		image: '',
		canViewTasks: true,
		isPocSuperAdmin: false,
		mattermostLoginURL: '',
		cloudflareLoginURL: '',
		isUnavailable: false
	};
}

export type SignedInMember = { memberID: string; role: 'admin' | 'member' };

export async function supabaseMember(): Promise<SignedInMember> {
	const { data } = await supabase().auth.getSession();
	const accountID = data.session?.user.id;
	if (!accountID) return { memberID: '', role: 'member' };
	const member = await supabase()
		.from('member')
		.select('id, is_admin')
		.eq('user_id', accountID)
		.maybeSingle<{ id: string; is_admin: boolean }>();
	return { memberID: member.data?.id ?? '', role: member.data?.is_admin ? 'admin' : 'member' };
}

export async function supabaseMemberRole(): Promise<'admin' | 'member'> {
	return (await supabaseMember()).role;
}

export async function signOutOfSupabase(): Promise<void> {
	await supabase().auth.signOut({ scope: 'local' });
}

export type ClaimMailOutcome = 'sent' | 'tooManyLately' | 'failed';

export async function sendClaimLink(email: string): Promise<ClaimMailOutcome> {
	const response = await fetch('/api/auth/claim', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ email: email.trim().toLowerCase() })
	});
	if (response.ok) return 'sent';
	return response.status === 429 ? 'tooManyLately' : 'failed';
}

export async function verifyClaimCode(email: string, code: string): Promise<void> {
	const { error } = await supabase().auth.verifyOtp({
		email: email.trim().toLowerCase(),
		token: code.trim(),
		type: 'email'
	});
	if (error) throw new Error(error.message);
}

export async function setSupabasePassword(password: string): Promise<void> {
	const { error } = await supabase().auth.updateUser({ password });
	if (error) throw new Error(error.message);
}

export async function signInWithSupabase(email: string, password: string): Promise<void> {
	const { error } = await supabase().auth.signInWithPassword({ email: email.trim().toLowerCase(), password });
	if (error) throw new Error(error.message);
}
