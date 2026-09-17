import { environmentOf } from '$lib/server/agent-request';
import { callingMember } from '$lib/server/member-request';
import { answerMCP } from '$lib/server/public-api/mcp';
import { challengingTheUnauthenticated } from '$lib/server/public-api/protected-resource';
import type { RequestHandler } from './$types';

// Only POST. The server keeps no session, so it has nothing to send a client
// between calls and nothing to terminate; a caller that asks for the standalone
// stream is answered 405, which the transport specification allows for exactly
// this case.
export const POST: RequestHandler = async ({ request, url, platform }) => {
	const environment = environmentOf(platform);
	const member = await callingMember(request, environment).catch(challengingTheUnauthenticated(url));
	if (member instanceof Response) return member;
	return answerMCP(request, environment, member);
};
