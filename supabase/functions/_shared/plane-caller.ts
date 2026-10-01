import { refuse } from './http.ts';
import { isTheSameSecret } from './same-secret.ts';
import { serviceClient, type SupabaseClient } from './service-client.ts';

export async function callingPlane(request: Request): Promise<SupabaseClient> {
	const authorization = request.headers.get('authorization') ?? '';
	const presented = authorization.startsWith('Bearer ') ? authorization.slice('Bearer '.length) : '';
	const held = Deno.env.get('SUPABASE_SERVICE_ROLE_KEY') ?? '';
	if (!presented || !held) refuse(401, 'no service key');
	if (!(await isTheSameSecret(presented, held))) refuse(403, 'refused');
	return serviceClient();
}
