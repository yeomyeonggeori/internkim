import { error, json } from '@sveltejs/kit';
import { bearerTokenOf, environmentOfPlatform } from '$lib/server/agent-request';
import { boxKeyOfAssertion, boxOfPublicKey, BoxRefused } from '$lib/server/box';
import { adminPasswordOutcomeReportSchema, reportAdminPasswordOutcome } from '$lib/server/box-admin-password';
import { planeCredentialsOf, controlPlane } from '$lib/server/control-plane';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOfPlatform(platform?.env);
	const plane = planeCredentialsOf(environment);
	if (!plane) error(500, 'the central plane is not configured');

	const publicKey = await boxKeyOfAssertion(bearerTokenOf(request));
	if (!publicKey) error(401, 'an admin password outcome is reported with an assertion signed by the box key');

	const client = controlPlane(plane);
	const box = await boxOfPublicKey(client, publicKey);
	if (!box) error(404, 'this box belongs to no company yet');

	const report = adminPasswordOutcomeReportSchema.safeParse(await request.json().catch(() => null));
	if (!report.success) error(400, 'an admin password outcome names the setting it answers and whether it was applied or failed');

	try {
		await reportAdminPasswordOutcome(client, box, report.data.settingID, report.data.result);
	} catch (refusal) {
		if (refusal instanceof BoxRefused) error(refusal.status, refusal.message);
		throw refusal;
	}
	return json({}, { headers: { 'Cache-Control': 'no-store' } });
};
