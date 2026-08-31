import { refuse } from './http.ts';
import { serviceClient, type SupabaseClient } from './service-client.ts';

export type CallingAgent = {
	client: SupabaseClient;
	companyID: string;
};

export async function callingAgent(request: Request): Promise<CallingAgent> {
	const authorization = request.headers.get('authorization') ?? '';
	const apiKey = authorization.startsWith('Bearer ') ? authorization.slice('Bearer '.length) : '';
	if (!apiKey) refuse(401, 'no agent key');

	const client = serviceClient();
	const { data, error } = await client
		.from('agent')
		.select('id, company_id, revoked_at')
		.eq('api_key_hash', await hashOf(apiKey))
		.maybeSingle();
	if (error) refuse(500, `agent key: ${error.message}`);
	if (!data || data.revoked_at) refuse(403, 'refused');

	await client.from('agent').update({ last_seen_at: new Date().toISOString() }).eq('id', data.id);
	return { client, companyID: data.company_id };
}

async function hashOf(secret: string): Promise<string> {
	const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(secret));
	return [...new Uint8Array(digest)].map((byte) => byte.toString(16).padStart(2, '0')).join('');
}
