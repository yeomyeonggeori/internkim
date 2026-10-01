import { env } from '$env/dynamic/private';
import { error } from '@sveltejs/kit';
import type { SupabaseClient } from '@supabase/supabase-js';
import { companyOfHostCredential, controlPlane, planeCredentialsOf } from './control-plane';
export { fleetDirectory } from './fleet-directory';

export type Environment = Record<string, string | undefined>;

export type CallingAgent = {
	client: SupabaseClient;
	companyID: string;
};

export function environmentOf(platform: App.Platform | undefined): Environment {
	return { ...env, ...((platform?.env ?? {}) as Environment) };
}

export async function callingAgent(request: Request, environment: Environment): Promise<CallingAgent> {
	const credentials = planeCredentialsOf(environment);
	if (!credentials) error(500, 'the control plane is not configured');

	const presented = bearerTokenOf(request);
	if (!presented) error(401, 'no company computer credential');

	const companyID = await companyOfHostCredential(credentials, presented);
	if (!companyID) error(403, 'refused');

	return { client: controlPlane(credentials), companyID };
}

export function bearerTokenOf(request: Request): string {
	const authorization = request.headers.get('authorization') ?? '';
	return authorization.startsWith('Bearer ') ? authorization.slice('Bearer '.length) : '';
}

export function environmentOfPlatform(platformEnvironment: unknown): Environment {
	const held: Environment = {};
	for (const [name, value] of Object.entries((platformEnvironment ?? {}) as Record<string, unknown>)) {
		if (typeof value === 'string') held[name] = value;
	}
	return { ...env, ...held };
}
