import { encodeBase64URL } from '../_shared/base64url.ts';
import { askedObject, json, refuse, serveRefusals } from '../_shared/http.ts';
import { callerClient, serviceClient } from '../_shared/service-client.ts';

Deno.serve(
	serveRefusals(async (request) => {
		if (request.method !== 'POST') refuse(405, 'POST only');

		const caller = callerClient(request);
		const admin = await caller.rpc('is_company_admin');
		if (admin.error || admin.data !== true) refuse(403, 'admins only');

		const asked = await askedObject(request);
		const subject = typeof asked.subject === 'string' ? asked.subject : '';
		if (!subject.startsWith('mailto:') && !subject.startsWith('https://')) {
			refuse(400, 'pass subject as a mailto: address or an https:// url');
		}

		const pair = await crypto.subtle.generateKey({ name: 'ECDSA', namedCurve: 'P-256' }, true, [
			'sign',
			'verify'
		]);
		const publicPoint = await crypto.subtle.exportKey('raw', pair.publicKey);
		const privateJWK = await crypto.subtle.exportKey('jwk', pair.privateKey);
		if (!privateJWK.d) refuse(500, 'the generated key carries no private scalar');

		const kept = await serviceClient().rpc('vapid_keys_keep', {
			new_public_key: encodeBase64URL(publicPoint),
			new_private_key: privateJWK.d,
			new_subject: subject,
			replace_existing: false
		});
		if (kept.error) refuse(kept.error.code === 'P0001' ? 409 : 500, kept.error.message);

		return json({ stored: kept.data === true });
	})
);
