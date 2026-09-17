import { environmentOfPlatform } from '$lib/server/agent-request';
import { defaultZone } from '$lib/server/fleet-domain';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = ({ platform }) => ({
	addressZone: environmentOfPlatform(platform?.env).CLOUDFLARE_DOMAIN || defaultZone
});
