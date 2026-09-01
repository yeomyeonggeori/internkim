import { refuse } from './http.ts';
import { serviceClient, type SupabaseClient } from './service-client.ts';

export async function callingPlane(request: Request): Promise<SupabaseClient> {
	const authorization = request.headers.get('authorization') ?? '';
	const presented = authorization.startsWith('Bearer ') ? authorization.slice('Bearer '.length) : '';
	const held = Deno.env.get('SUPABASE_SERVICE_ROLE_KEY') ?? '';
	if (!presented || !held) refuse(401, 'no service key');
	if (!(await isTheSameSecret(presented, held))) refuse(403, 'refused');
	return serviceClient();
}

async function isTheSameSecret(presented: string, held: string): Promise<boolean> {
	const [a, b] = await Promise.all([digestOf(presented), digestOf(held)]);
	return a.length === b.length && a.every((byte, at) => byte === b[at]);
}

async function digestOf(secret: string): Promise<Uint8Array> {
	return new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(secret)));
}
