import { env } from '$env/dynamic/private';
import { error } from '@sveltejs/kit';
import type { SupabaseClient } from '@supabase/supabase-js';
import { agentOfKey, companyOfFleet, controlPlane } from './control-plane';
import type { FleetDirectory } from './fleet-user-directory';

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

export function environmentOfPlatform(platformEnvironment: unknown): Environment {
	const held: Environment = {};
	for (const [name, value] of Object.entries((platformEnvironment ?? {}) as Record<string, unknown>)) {
		if (typeof value === 'string') held[name] = value;
	}
	return { ...env, ...held };
}

export async function fleetDirectory(environment: Environment, fleetID: string): Promise<FleetDirectory | null> {
	const projectURL = environment.SUPABASE_URL ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? '';
	if (!projectURL || !serviceRoleKey) return null;

	const client = controlPlane({ projectURL, serviceRoleKey });
	const companyID = await companyOfFleet(client, fleetID);
	if (!companyID) return null;
	return { client, companyID };
}
