import { supabase } from '$lib/supabase';
import { callCompanyApp } from '$lib/host-bridge';
import { normalizeMailAccountResponse } from './mail-api-normalizers';
import type { MailAccountWritePayload } from './mail-types';

async function askTheRecord(method: string, payload?: MailAccountWritePayload): Promise<unknown> {
	const { data } = await supabase().auth.getSession();
	const accessToken = data.session?.access_token ?? '';
	if (!accessToken) throw new Error('sign in first');

	const response = await fetch('/api/member/mail-account', {
		method,
		headers: {
			Authorization: `Bearer ${accessToken}`,
			...(payload ? { 'Content-Type': 'application/json' } : {})
		},
		body: payload ? JSON.stringify(payload) : undefined
	});
	if (!response.ok) throw new Error((await refusalOf(response)) || `the mail account answered ${response.status}`);
	const answered = (await response.json()) as { account: unknown };
	return answered.account;
}

async function refusalOf(response: Response): Promise<string> {
	const body: unknown = await response.json().catch(() => null);
	if (typeof body !== 'object' || body === null || !('message' in body)) return '';
	return typeof body.message === 'string' ? body.message : '';
}

export async function recordMailAccount() {
	return normalizeMailAccountResponse(await askTheRecord('GET'));
}

export async function keepRecordMailAccount(payload: MailAccountWritePayload) {
	return normalizeMailAccountResponse(await askTheRecord('PUT', payload));
}

export async function testRecordMailAccount(): Promise<void> {
	const answer = await callCompanyApp({ capability: 'person.mail.test' });
	if (answer.status === 200) return;
	const said = (answer.body as { error?: string } | null)?.error;
	throw new Error(said || `the mail account answered ${answer.status}`);
}
