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

export async function supabaseMemberRole(): Promise<'admin' | 'member'> {
	const { data } = await supabase().auth.getSession();
	const accountID = data.session?.user.id;
	if (!accountID) return 'member';
	const member = await supabase().from('member').select('is_admin').eq('user_id', accountID).maybeSingle();
	return member.data?.is_admin ? 'admin' : 'member';
}

// Signing out globally needs the server to accept the token, and a stale one
// leaves the browser still holding a session. Ending it locally always works.
export async function signOutOfSupabase(): Promise<void> {
	await supabase().auth.signOut({ scope: 'local' });
}

export async function signInWithSupabase(email: string, password: string): Promise<void> {
	const { error } = await supabase().auth.signInWithPassword({ email: email.trim().toLowerCase(), password });
	if (error) throw new Error(error.message);
}
