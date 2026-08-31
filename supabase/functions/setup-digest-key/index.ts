import { createClient } from 'npm:@supabase/supabase-js@2';

function json(body: Record<string, unknown>, status = 200): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}

Deno.serve(async (request) => {
	if (request.method !== 'POST') return json({ error: 'POST only' }, 405);

	const caller = createClient(
		Deno.env.get('SUPABASE_URL') ?? '',
		Deno.env.get('SUPABASE_ANON_KEY') ?? '',
		{ global: { headers: { Authorization: request.headers.get('Authorization') ?? '' } } }
	);

	const kept = await caller.rpc('digest_agent_key_keep');
	if (kept.error) {
		return json({ error: kept.error.message }, kept.error.code === '42501' ? 403 : 500);
	}

	return json({ stored: kept.data === true });
});
