import { isSupabaseConfigured } from '$lib/supabase';

export function settingsSource(adminBaseURL: string): string {
	return isSupabaseConfigured() ? 'company' : adminBaseURL;
}
