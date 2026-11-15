import { createClient } from 'npm:@supabase/supabase-js@2';
import { decodeBase64URL } from './base64url.ts';
import { refuse } from './http.ts';
import { serviceClient, type SupabaseClient } from './service-client.ts';

export type CallingComputer = { boxKey: string | null; agentKeyHash: string | null };

export type CallingAgent = {
	client: SupabaseClient;
	companyID: string;
	computer: CallingComputer;
};

export async function callingAgent(request: Request): Promise<CallingAgent> {
	const authorization = request.headers.get('authorization') ?? '';
	const presented = authorization.startsWith('Bearer ') ? authorization.slice('Bearer '.length) : '';
	if (!presented) refuse(401, 'no company computer credential');

	const client = serviceClient();
	const isHostSession = presented.split('.').length === 3;
	const companyID = isHostSession ? await companyOfHostSession(presented) : await companyOfAgentKey(client, presented);
	if (!companyID) refuse(403, 'refused');
	const computer = isHostSession ? computerNamedBy(presented) : { boxKey: null, agentKeyHash: await hashOf(presented) };
	return { client, companyID, computer };
}

function computerNamedBy(verifiedHostSession: string): CallingComputer {
	const [, payload] = verifiedHostSession.split('.');
	const claims: unknown = JSON.parse(new TextDecoder().decode(decodeBase64URL(payload)));
	return { boxKey: computerClaim(claims, 'fleet_id'), agentKeyHash: computerClaim(claims, 'agent_key_hash') };
}

function computerClaim(claims: unknown, name: string): string | null {
	if (typeof claims !== 'object' || claims === null || !('app_metadata' in claims)) return null;
	const metadata = claims.app_metadata;
	if (typeof metadata !== 'object' || metadata === null) return null;
	const value: unknown = Reflect.get(metadata, name);
	return typeof value === 'string' ? value : null;
}

async function companyOfHostSession(accessToken: string): Promise<string | null> {
	const caller = createClient(Deno.env.get('SUPABASE_URL') ?? '', Deno.env.get('SUPABASE_ANON_KEY') ?? '', {
		auth: { autoRefreshToken: false, persistSession: false },
		global: { headers: { Authorization: `Bearer ${accessToken}` } }
	});
	const { data, error, status } = await caller.rpc('my_app_company');
	if (error && status !== 401) refuse(502, `the record could not say whose computer this is: ${error.message}`);
	return typeof data === 'string' ? data : null;
}

async function companyOfAgentKey(client: SupabaseClient, apiKey: string): Promise<string | null> {
	const { data, error } = await client
		.from('agent')
		.select('id, company_id, revoked_at')
		.eq('api_key_hash', await hashOf(apiKey))
		.maybeSingle();
	if (error) refuse(500, `agent key: ${error.message}`);
	if (!data || data.revoked_at) return null;

	await client.from('agent').update({ last_seen_at: new Date().toISOString() }).eq('id', data.id);
	return data.company_id;
}

async function hashOf(secret: string): Promise<string> {
	const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(secret));
	return [...new Uint8Array(digest)].map((byte) => byte.toString(16).padStart(2, '0')).join('');
}
