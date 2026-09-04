import { environmentOf } from '$lib/server/agent-request';
import { callingMember } from '$lib/server/member-request';
import { answerMCP } from '$lib/server/public-api/mcp';
import type { RequestHandler } from './$types';

export const fallback: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOf(platform);
	const member = await callingMember(request, environment);
	return answerMCP(request, environment, member);
};
