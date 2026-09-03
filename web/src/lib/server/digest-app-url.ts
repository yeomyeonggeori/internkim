import type { SupabaseClient } from '@supabase/supabase-js';
import type { Environment } from './agent-request';
import { theOneAddressOf } from './company-host-redirect';
import { defaultZone } from './fleet-domain';

function theAppAddressOf(environment: Environment): string {
	return `https://${theOneAddressOf(environment.CLOUDFLARE_DOMAIN || defaultZone)}`;
}

export async function keepTheAppAddress(plane: SupabaseClient, environment: Environment): Promise<boolean> {
	const kept = await plane.rpc('digest_app_url_keep', { new_app_url: theAppAddressOf(environment) });
	if (kept.error) throw new Error(kept.error.message);
	return kept.data === true;
}
