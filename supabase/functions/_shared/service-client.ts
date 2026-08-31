import { createClient, type SupabaseClient } from 'npm:@supabase/supabase-js@2';

export type { SupabaseClient };

export function serviceClient(): SupabaseClient {
	return createClient(
		Deno.env.get('SUPABASE_URL') ?? '',
		Deno.env.get('SUPABASE_SERVICE_ROLE_KEY') ?? '',
		{ auth: { autoRefreshToken: false, persistSession: false } }
	);
}
