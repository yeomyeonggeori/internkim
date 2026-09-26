import { error } from '@sveltejs/kit';
import type { SupabaseClient } from '@supabase/supabase-js';
import { bearerTokenOf, type Environment } from './agent-request';
import { controlPlane } from './control-plane';

export async function callingScheduledJob(request: Request, environment: Environment): Promise<SupabaseClient> {
	const credentials = {
		projectURL: environment.SUPABASE_URL ?? '',
		serviceRoleKey: environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? '',
	};
	if (!credentials.projectURL || !credentials.serviceRoleKey) error(500, 'the control plane is not configured');

	const presented = bearerTokenOf(request);
	if (!presented) error(401, 'no scheduled job secret');

	const client = controlPlane(credentials);
	const { data, error: refusal } = await client.rpc('is_a_scheduled_job', { presented });
	if (refusal) error(500, `scheduled job secret: ${refusal.message}`);
	if (data !== true) error(403, 'refused');
	return client;
}
