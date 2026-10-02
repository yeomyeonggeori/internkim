import { error, json } from '@sveltejs/kit';
import { bearerTokenOf, environmentOfPlatform } from '$lib/server/agent-request';
import { boxKeyOfAssertion, boxOfPublicKey } from '$lib/server/box';
import { pendingWifiChangeOf, recordNearbyNetworks, wifiChangeFetchSchema } from '$lib/server/box-wifi';
import { planeCredentialsOf, controlPlane } from '$lib/server/control-plane';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOfPlatform(platform?.env);
	const plane = planeCredentialsOf(environment);
	if (!plane) error(500, 'the central plane is not configured');

	const publicKey = await boxKeyOfAssertion(bearerTokenOf(request));
	if (!publicKey) error(401, 'a Wi-Fi change is asked for with an assertion signed by the box key');

	const box = await boxOfPublicKey(controlPlane(plane), publicKey);
	if (!box) error(404, 'this box belongs to no company yet');

	const body = await request.text();
	const report = wifiChangeFetchSchema.safeParse(body.trim() ? parsedJson(body) : {});
	if (!report.success) error(400, 'a Wi-Fi fetch carries at most 50 nearby networks, each with an ssid, a signal percent and whether it is secured');
	if (report.data.nearbyNetworks) await recordNearbyNetworks(controlPlane(plane), box, report.data.nearbyNetworks);

	return json({ companyID: box.companyID, change: pendingWifiChangeOf(box) }, { headers: { 'Cache-Control': 'no-store' } });
};

function parsedJson(body: string): unknown {
	try {
		return JSON.parse(body);
	} catch {
		return null;
	}
}
