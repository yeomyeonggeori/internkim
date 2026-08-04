import { isSupabaseConfigured } from '$lib/supabase-session';
import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const load: PageLoad = () => {
	if (isSupabaseConfigured) redirect(307, '/flow/');
};
