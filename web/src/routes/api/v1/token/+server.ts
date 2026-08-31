import { environmentOf } from '$lib/server/agent-request';
import { callingMember } from '$lib/server/member-request';
import {
	forgetPersonalAccessToken,
	issuePersonalAccessToken,
	personalAccessTokens,
} from '$lib/server/control-plane';
import { issueRefusal, nextUnusedName, revocationRefusal } from '$lib/server/public-api/tokens';
import { publicAPIPermissionOf } from '$lib/public-api-permission';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

// The token is answered once. Only its hash is kept, so nothing can read it back
// to whoever lost it; they make another by that name and it replaces this one.
export const POST: RequestHandler = async ({ request, platform }) => {
	const member = await callingMember(request, environmentOf(platform));

	const asked = (await request.json().catch(() => null)) as { name?: unknown; permission?: unknown } | null;
	if (!asked || typeof asked !== 'object') error(400, 'this call carried a body that is not a json object');

	const namedByTheCaller = typeof asked.name === 'string' ? asked.name.trim() : '';
	const name =
		namedByTheCaller || nextUnusedName(await personalAccessTokens(member.record, member.memberID));

	const permission =
		asked.permission === undefined ? member.permission : publicAPIPermissionOf(asked.permission);
	if (!permission) error(400, 'a token reads, writes or deletes');

	const refused = issueRefusal(member, name, permission);
	if (refused) error(refused.status, refused.message);

	return json({
		name,
		permission,
		token: await issuePersonalAccessToken(member.record, member.memberID, name, permission),
	});
};

export const DELETE: RequestHandler = async ({ request, url, platform }) => {
	const member = await callingMember(request, environmentOf(platform));

	const name = (url.searchParams.get('name') ?? '').trim();
	const refused = revocationRefusal(member, name);
	if (refused) error(refused.status, refused.message);

	if (!(await forgetPersonalAccessToken(member.record, member.memberID, name))) {
		error(404, 'no token of yours goes by that');
	}
	return json({ forgotten: name });
};
