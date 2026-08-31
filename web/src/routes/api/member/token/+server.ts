import { forgetPersonalAccessToken, issuePersonalAccessToken } from '$lib/server/control-plane';
import { memberSignedIn, tokenStore } from '$lib/server/member-token-request';
import { fullPublicAPIPermission, publicAPIPermissionOf } from '$lib/public-api-permission';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

// The token is answered once. Only its hash is kept, so nothing can read it back
// to whoever lost it; they make another by that name and it replaces this one.
export const POST: RequestHandler = async ({ request, platform }) => {
	const memberID = await memberSignedIn(request, platform);

	const asked = (await request.json().catch(() => ({}))) as { name?: unknown; permission?: unknown };
	const name = typeof asked.name === 'string' ? asked.name.trim() : '';
	if (!name) error(400, 'a token needs a name');
	if (name.length > 64) error(400, 'that name is too long for a token');

	const permission = asked.permission === undefined ? fullPublicAPIPermission : publicAPIPermissionOf(asked.permission);
	if (!permission) error(400, 'a token reads, writes or deletes');

	return json({
		name,
		permission,
		token: await issuePersonalAccessToken(tokenStore(platform), memberID, name, permission),
	});
};

export const DELETE: RequestHandler = async ({ request, url, platform }) => {
	const memberID = await memberSignedIn(request, platform);

	const name = (url.searchParams.get('name') ?? '').trim();
	if (!name) error(400, 'a revocation names the token');

	if (!(await forgetPersonalAccessToken(tokenStore(platform), memberID, name))) {
		error(404, 'no token of yours goes by that');
	}
	return json({ forgotten: name });
};
