import { json, error } from '@sveltejs/kit';
import { callingAgent, environmentOf } from '$lib/server/agent-request';
import { reconcileMessengerAccounts } from '$lib/server/member-credential';
import { messengerIdentityCredentialKinds } from '$lib/server/public-api/catalog/credential';
import type { RequestHandler } from './$types';

type ReconcileRequest = {
	platform?: unknown;
	kind?: unknown;
};

function required(value: unknown, field: string): string {
	if (typeof value !== 'string' || !value.trim()) error(400, `${field} is required`);
	return value.trim();
}

function declaredMessengerIdentityKind(value: unknown): string {
	const kind = required(value, 'kind');
	if (!messengerIdentityCredentialKinds.some((declared) => declared === kind)) {
		error(400, `no credential kind named ${kind} carries a messenger identity here`);
	}
	return kind;
}

export const POST: RequestHandler = async ({ request, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));
	const body = (await request.json().catch(() => ({}))) as ReconcileRequest;

	const reconciled = await reconcileMessengerAccounts(
		client,
		companyID,
		required(body.platform, 'platform'),
		declaredMessengerIdentityKind(body.kind)
	);

	return json({ reconciled });
};
