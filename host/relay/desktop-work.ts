import { reasonOf } from './failure';

export const desktopWorkPath = '/desktop-work';
export const noGatewayStatus = 501;
export const gatewayUnreachedStatus = 502;

export type DesktopWorkRoute = {
	gatewayURL: string | undefined;
	companyID: string;
	hostAccessToken: () => Promise<string>;
};

export function desktopWorkForwarder(route: DesktopWorkRoute): (request: Request) => Promise<Response> {
	return async (request) => {
		if (!route.gatewayURL) {
			return Response.json({ error: 'this relay is given no GATEWAY_URL, so it has no gateway to hand desktop work to' }, { status: noGatewayStatus });
		}
		const address = desktopWorkAddressOf(route.gatewayURL, route.companyID);
		try {
			const answered = await fetch(address, {
				method: 'POST',
				headers: { Authorization: `Bearer ${await route.hostAccessToken()}`, 'Content-Type': 'application/json' },
				body: await request.text(),
				signal: request.signal
			});
			return new Response(answered.body, { status: answered.status, headers: { 'Content-Type': answered.headers.get('Content-Type') ?? 'application/json' } });
		} catch (failure) {
			return Response.json({ error: `desktop work did not reach the gateway at ${address}: ${reasonOf(failure)}` }, { status: gatewayUnreachedStatus });
		}
	};
}

function desktopWorkAddressOf(gatewayURL: string, companyID: string): string {
	return `${gatewayURL.replace(/^ws/, 'http').replace(/\/+$/, '')}/company/${encodeURIComponent(companyID)}${desktopWorkPath}`;
}
