import { error, json } from '@sveltejs/kit';
import { CompanySignupError, companySignupRequest, requestCompanySignupEmail } from '$lib/server/company-signup';
import { environmentOfPlatform } from '$lib/server/agent-request';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform, url }) => {
	const environment = environmentOfPlatform(platform?.env);
	const parsed = companySignupRequest.safeParse(await request.json().catch(() => null));
	if (!parsed.success) error(400, 'a valid email address is required');
	try {
		await requestCompanySignupEmail(environment, parsed.data.email, url.origin);
	} catch (caughtError) {
		if (caughtError instanceof CompanySignupError) error(caughtError.status, caughtError.message);
		throw caughtError;
	}
	return json({ sent: true });
};
