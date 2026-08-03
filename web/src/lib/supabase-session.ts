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

export async function signInWithSupabase(email: string, password: string): Promise<void> {
	const { error } = await supabase().auth.signInWithPassword({ email: email.trim().toLowerCase(), password });
	if (error) throw new Error(error.message);
}
