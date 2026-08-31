import { createClient } from 'npm:@supabase/supabase-js@2';

function json(body: Record<string, unknown>, status = 200): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}

Deno.serve(async (request) => {
	if (request.method !== 'POST') return json({ error: 'POST only' }, 405);

	const projectURL = Deno.env.get('SUPABASE_URL') ?? '';
	const caller = createClient(projectURL, Deno.env.get('SUPABASE_ANON_KEY') ?? '', {
		global: { headers: { Authorization: request.headers.get('Authorization') ?? '' } }
	});
	const admin = await caller.rpc('is_company_admin');
	if (admin.error || admin.data !== true) return json({ error: 'admins only' }, 403);

	const kept = await caller.rpc('digest_agent_key_keep');
	if (kept.error) {
		return json({ error: kept.error.message }, kept.error.code === '42501' ? 403 : 500);
	}

	const service = createClient(projectURL, Deno.env.get('SUPABASE_SERVICE_ROLE_KEY') ?? '');
	const target = await service.rpc('digest_target_keep', { new_project_url: projectURL });
	if (target.error) return json({ error: target.error.message }, 500);

	return json({ stored: kept.data === true });
});
