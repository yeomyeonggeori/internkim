import { createClient } from 'npm:@supabase/supabase-js@2';

function json(body: Record<string, unknown>, status = 200): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}

async function hashOf(secret: string): Promise<string> {
	const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(secret));
	return [...new Uint8Array(digest)].map((byte) => byte.toString(16).padStart(2, '0')).join('');
}

Deno.serve(async (request) => {
	if (request.method !== 'POST') return json({ error: 'POST only' }, 405);

	const caller = createClient(
		Deno.env.get('SUPABASE_URL') ?? '',
		Deno.env.get('SUPABASE_ANON_KEY') ?? '',
		{ global: { headers: { Authorization: request.headers.get('Authorization') ?? '' } } }
	);

	const apiKey = [...crypto.getRandomValues(new Uint8Array(32))]
		.map((byte) => byte.toString(16).padStart(2, '0'))
		.join('');

	const kept = await caller.rpc('digest_agent_key_keep', {
		new_key_hash: await hashOf(apiKey),
		new_key: apiKey
	});
	if (kept.error) {
		return json({ error: kept.error.message }, kept.error.code === '42501' ? 403 : 500);
	}

	return json({ stored: kept.data === true });
});
