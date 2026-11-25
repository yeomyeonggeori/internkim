import { createClient } from 'npm:@supabase/supabase-js@2';
import { decodeBase64URL } from './base64url.ts';
import { refuse } from './http.ts';
import { serviceClient, type SupabaseClient } from './service-client.ts';

export type CallingComputer = { boxKey: string };

export type CallingAgent = {
	client: SupabaseClient;
	companyID: string;
	computer: CallingComputer;
};

export async function callingAgent(request: Request): Promise<CallingAgent> {
	const authorization = request.headers.get('authorization') ?? '';
	const presented = authorization.startsWith('Bearer ') ? authorization.slice('Bearer '.length) : '';
	if (!presented) refuse(401, 'no company computer credential');

	const companyID = await companyOfHostSession(presented);
	if (!companyID) refuse(403, 'refused');
	return { client: serviceClient(), companyID, computer: { boxKey: boxNamedBy(presented) } };
}

function boxNamedBy(verifiedHostSession: string): string {
	const [, payload] = verifiedHostSession.split('.');
	const claims: unknown = JSON.parse(new TextDecoder().decode(decodeBase64URL(payload)));
	if (typeof claims !== 'object' || claims === null || !('app_metadata' in claims)) refuse(403, 'refused');
	const metadata = claims.app_metadata;
	if (typeof metadata !== 'object' || metadata === null) refuse(403, 'refused');
	const boxKey: unknown = Reflect.get(metadata, 'fleet_id');
	if (typeof boxKey !== 'string') refuse(403, 'refused');
	return boxKey;
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
