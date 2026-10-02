import { error, json } from '@sveltejs/kit';
import { bearerTokenOf, environmentOfPlatform } from '$lib/server/agent-request';
import { boxKeyOfAssertion, boxOfPublicKey, BoxRefused } from '$lib/server/box';
import { reportWifiOutcome, wifiOutcomeReportSchema } from '$lib/server/box-wifi';
import { planeCredentialsOf, controlPlane } from '$lib/server/control-plane';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOfPlatform(platform?.env);
	const plane = planeCredentialsOf(environment);
	if (!plane) error(500, 'the central plane is not configured');

	const publicKey = await boxKeyOfAssertion(bearerTokenOf(request));
	if (!publicKey) error(401, 'a Wi-Fi outcome is reported with an assertion signed by the box key');

	const client = controlPlane(plane);
	const box = await boxOfPublicKey(client, publicKey);
	if (!box) error(404, 'this box belongs to no company yet');

	const report = wifiOutcomeReportSchema.safeParse(await request.json().catch(() => null));
	if (!report.success) error(400, 'a Wi-Fi outcome names the request it answers and whether it joined or failed');

	try {
		await reportWifiOutcome(client, box, report.data.requestID, report.data.result);
	} catch (refusal) {
		if (refusal instanceof BoxRefused) error(refusal.status, refusal.message);
		throw refusal;
	}
	return json({}, { headers: { 'Cache-Control': 'no-store' } });
};
