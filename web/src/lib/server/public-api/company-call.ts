import { error } from '@sveltejs/kit';
import type { Environment } from '$lib/server/agent-request';

export type CompanyAnswer = { status: number; body: unknown };

export type CompanyCallTransport = (
	url: string,
	options: { method: string; headers: Record<string, string>; body: string }
) => Promise<{ status: number; json: () => Promise<unknown> }>;

const callThroughTheRuntime: CompanyCallTransport = (url, options) => fetch(url, options);

export function gatewayHTTPAddress(gatewayURL: string): string {
	return gatewayURL
		.replace(/\/+$/, '')
		.replace(/^wss:\/\//, 'https://')
		.replace(/^ws:\/\//, 'http://');
}

export async function callCompany(
	environment: Environment,
	companyID: string,
	capability: string,
	body: Record<string, unknown>,
	transport: CompanyCallTransport = callThroughTheRuntime
): Promise<CompanyAnswer> {
	const gatewayURL = environment.GATEWAY_URL ?? '';
	const gatewayToken = environment.GATEWAY_ADMIN_TOKEN ?? '';
	if (!gatewayURL || !gatewayToken) error(503, 'the company gateway is not configured');

	const answered = await transport(
		`${gatewayHTTPAddress(gatewayURL)}/company/${encodeURIComponent(companyID)}/call`,
		{
			method: 'POST',
			headers: { Authorization: `Bearer ${gatewayToken}`, 'Content-Type': 'application/json' },
			body: JSON.stringify({ requestID: crypto.randomUUID(), capability, body })
		}
	);
	const answer = (await answered.json().catch(() => null)) as { status?: unknown; body?: unknown } | null;
	const status = httpStatusOf(answer?.status);
	if (!status) error(502, `the gateway answered ${answered.status}, and no status a caller can use`);
	return { status, body: answer?.body ?? null };
}

function httpStatusOf(offered: unknown): number | null {
	if (typeof offered !== 'number' || !Number.isInteger(offered)) return null;
	if (offered < 100 || offered > 599) return null;
	return offered;
}
