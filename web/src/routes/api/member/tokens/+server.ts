import { personalAccessTokens } from '$lib/server/control-plane';
import { memberSignedIn, tokenStore } from '$lib/server/member-token-request';
import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ request, platform }) => {
	const memberID = await memberSignedIn(request, platform);
	return json({ tokens: await personalAccessTokens(tokenStore(platform), memberID) });
};
