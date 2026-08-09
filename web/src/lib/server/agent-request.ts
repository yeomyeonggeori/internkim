import { env } from '$env/dynamic/private';
import { error } from '@sveltejs/kit';
import type { SupabaseClient } from '@supabase/supabase-js';
import { agentOfKey, controlPlane } from './control-plane';

export type Environment = Record<string, string | undefined>;

export type CallingAgent = {
	client: SupabaseClient;
	companyID: string;
};

export function environmentOf(platform: App.Platform | undefined): Environment {
	return { ...env, ...((platform?.env ?? {}) as Environment) };
}

export async function callingAgent(request: Request, environment: Environment): Promise<CallingAgent> {
	const projectURL = environment.SUPABASE_URL ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? '';
	if (!projectURL || !serviceRoleKey) error(500, 'the control plane is not configured');

	const authorization = request.headers.get('authorization') ?? '';
	const apiKey = authorization.startsWith('Bearer ') ? authorization.slice('Bearer '.length) : '';
	if (!apiKey) error(401, 'no agent key');

	const client = controlPlane({ projectURL, serviceRoleKey });
	const agent = await agentOfKey(client, apiKey);
	if (!agent) error(403, 'refused');

	return { client, companyID: agent.companyID };
}
