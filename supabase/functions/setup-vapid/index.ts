import { createClient } from 'npm:@supabase/supabase-js@2';
import { encodeBase64URL } from '../_shared/base64url.ts';

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

	const body = await request.json().catch(() => ({}));
	const subject = typeof body.subject === 'string' ? body.subject : '';
	const rotate = body.rotate === true;
	if (!subject.startsWith('mailto:') && !subject.startsWith('https://')) {
		return json({ error: 'pass subject as a mailto: address or an https:// url' }, 400);
	}

	const pair = await crypto.subtle.generateKey({ name: 'ECDSA', namedCurve: 'P-256' }, true, [
		'sign',
		'verify'
	]);
	const publicPoint = await crypto.subtle.exportKey('raw', pair.publicKey);
	const privateJWK = await crypto.subtle.exportKey('jwk', pair.privateKey);
	if (!privateJWK.d) return json({ error: 'the generated key carries no private scalar' }, 500);

	const service = createClient(projectURL, Deno.env.get('SUPABASE_SERVICE_ROLE_KEY') ?? '');
	const kept = await service.rpc('vapid_keys_keep', {
		new_public_key: encodeBase64URL(publicPoint),
		new_private_key: privateJWK.d,
		new_subject: subject,
		replace_existing: rotate
	});
	if (kept.error) return json({ error: kept.error.message }, 500);

	return json({ stored: kept.data === true });
});
