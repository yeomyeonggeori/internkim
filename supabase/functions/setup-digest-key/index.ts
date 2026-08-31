import { json, refuse, serveRefusals } from '../_shared/http.ts';
import { callerClient, serviceClient } from '../_shared/service-client.ts';

Deno.serve(
	serveRefusals(async (request) => {
		if (request.method !== 'POST') refuse(405, 'POST only');

		const caller = callerClient(request);
		const admin = await caller.rpc('is_company_admin');
		if (admin.error || admin.data !== true) refuse(403, 'admins only');

		const kept = await caller.rpc('digest_agent_key_keep');
		if (kept.error) refuse(kept.error.code === '42501' ? 403 : 500, kept.error.message);

		const target = await serviceClient().rpc('digest_target_keep', {
			new_project_url: Deno.env.get('SUPABASE_URL') ?? ''
		});
		if (target.error) refuse(500, target.error.message);

		return json({ stored: kept.data === true });
	})
);
