import { error, json } from '@sveltejs/kit';
import { hostSetupRequestSchema } from '$lib/company/host-setup';
import { environmentOfPlatform } from '$lib/server/agent-request';
import { callingHostAdministrator, companyHostSetupStatus, createHostConfiguration } from '$lib/server/company-host-setup';
import type { RequestHandler } from './$types';

const privateHeaders = { 'Cache-Control': 'no-store', 'Pragma': 'no-cache' };

export const GET: RequestHandler = async ({ request, platform }) => {
	const member = await callingHostAdministrator(request, environmentOfPlatform(platform?.env));
	return json(await companyHostSetupStatus(member), { headers: privateHeaders });
};

export const POST: RequestHandler = async ({ request, platform, url }) => {
	const environment = environmentOfPlatform(platform?.env);
	const member = await callingHostAdministrator(request, environment);
	const parsed = hostSetupRequestSchema.safeParse(await request.json().catch(() => null));
	if (!parsed.success) error(400, 'choose whether to replace the existing computer connection');
	const configuration = await createHostConfiguration(member, environment, url.origin, parsed.data.replaceExisting);
	return json(configuration, {
		headers: { ...privateHeaders, 'Content-Disposition': 'attachment; filename="internkim-host.json"' }
	});
};
