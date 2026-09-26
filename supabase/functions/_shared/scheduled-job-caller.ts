import { refuse } from './http.ts';
import { serviceClient, type SupabaseClient } from './service-client.ts';

export async function callingScheduledJob(request: Request): Promise<SupabaseClient> {
	const authorization = request.headers.get('authorization') ?? '';
	const presented = authorization.startsWith('Bearer ') ? authorization.slice('Bearer '.length) : '';
	if (!presented) refuse(401, 'no scheduled job secret');

	const client = serviceClient();
	const { data, error } = await client.rpc('is_a_scheduled_job', { presented });
	if (error) refuse(500, `scheduled job secret: ${error.message}`);
	if (data !== true) refuse(403, 'refused');
	return client;
}
