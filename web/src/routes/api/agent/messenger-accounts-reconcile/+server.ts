import { json, error } from '@sveltejs/kit';
import { callingAgent, environmentOf } from '$lib/server/agent-request';
import { reconcileMessengerAccounts } from '$lib/server/member-credential';
import type { RequestHandler } from './$types';

type ReconcileRequest = {
	platform?: unknown;
	kind?: unknown;
};

function required(value: unknown, field: string): string {
	if (typeof value !== 'string' || !value.trim()) error(400, `${field} is required`);
	return value.trim();
}

export const POST: RequestHandler = async ({ request, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));
	const body = (await request.json().catch(() => ({}))) as ReconcileRequest;

	const reconciled = await reconcileMessengerAccounts(
		client,
		companyID,
		required(body.platform, 'platform'),
		required(body.kind, 'kind')
	);

	return json({ reconciled });
};
