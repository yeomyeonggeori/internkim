import { isSupabaseConfigured, supabase } from '$lib/supabase';
import { fetchWebSessionEmail } from '$lib/web-session';

export async function signedInEmail(): Promise<string> {
	if (!isSupabaseConfigured()) return fetchWebSessionEmail();
	const { data } = await supabase().auth.getSession();
	return (data.session?.user.email ?? '').trim().toLowerCase();
}
