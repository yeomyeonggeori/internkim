import { createClient, type SupabaseClient } from '@supabase/supabase-js';

const projectURL = import.meta.env.VITE_SUPABASE_URL;
const publishableKey = import.meta.env.VITE_SUPABASE_PUBLISHABLE_KEY;

export const isSupabaseConfigured = Boolean(projectURL && publishableKey);

let client: SupabaseClient | undefined;

export function supabase(): SupabaseClient {
	if (!projectURL || !publishableKey) {
		throw new Error('set VITE_SUPABASE_URL and VITE_SUPABASE_PUBLISHABLE_KEY to reach Supabase');
	}
	client ??= createClient(projectURL, publishableKey);
	return client;
}
