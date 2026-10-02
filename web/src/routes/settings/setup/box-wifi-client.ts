import { z } from 'zod';
import { wifiChangeStatusSchema, type ConnectedBox, type WifiChangeStatus, type WifiNetwork } from '$lib/company/box';
import { sealWifiNetwork } from '$lib/company/seal-to-box';
import { askCompanyRoute } from './host-setup-client';

const statusAnswerSchema = z.object({ status: wifiChangeStatusSchema }).strict();

export async function changeBoxWifiNetwork(
	box: ConnectedBox,
	network: WifiNetwork
): Promise<{ requestID: string; status: WifiChangeStatus }> {
	const requestID = crypto.randomUUID();
	const sealed = await sealWifiNetwork(network, box, requestID);
	const answer = statusAnswerSchema.parse(await askCompanyRoute('/api/company/box/wifi', 'PUT', { requestID, sealed }));
	return { requestID, status: answer.status };
}

export async function fetchWifiChangeStatus(): Promise<WifiChangeStatus> {
	return statusAnswerSchema.parse(await askCompanyRoute('/api/company/box/wifi', 'GET')).status;
}
