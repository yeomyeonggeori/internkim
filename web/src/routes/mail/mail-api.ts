import { invokeTool } from '$lib/public-api-call';
import { isSupabaseConfigured } from '$lib/supabase';
import { keepRecordMailAccount, recordMailAccount, testRecordMailAccount } from './mail-account-api';
import {
	normalizeMailBootstrapResponse,
	normalizeMailAccountResponse,
	normalizeMailboxesResponse,
	normalizeMailMessageDetailResponse,
	normalizeMailMessagesResponse
} from './mail-api-normalizers';
import type { ComposePayload, MailAccountWritePayload, MailMessage } from './mail-types';

export type MailErrorMessages = {
	fallback: string;
	serviceUnavailable: string;
};

export async function fetchMailBootstrap(query: URLSearchParams, errors: MailErrorMessages) {
	if (isSupabaseConfigured()) return companyMailBootstrap(query, errors);
	const response = await fetchMailResponse(`/mail/api/bootstrap?${query}`, {
		method: 'GET',
		credentials: 'include'
	}, errors);
	await assertMailResponse(response, errors);
	return normalizeMailBootstrapResponse(await response.json());
}

async function companyMailBootstrap(query: URLSearchParams, errors: MailErrorMessages) {
	const account = await fetchMailAccount(errors);
	if (!account.isConfigured) return normalizeMailBootstrapResponse({ account });
	const [mailboxes, messages] = await Promise.all([
		fetchMailboxes(errors),
		fetchMailMessages(query, errors)
	]);
	return normalizeMailBootstrapResponse({
		account,
		mailboxes,
		messages: messages.messages,
		nextCursor: messages.nextCursor
	});
}

export async function fetchMailAccount(errors: MailErrorMessages) {
	if (isSupabaseConfigured()) return recordMailAccount();
	const response = await fetchMailResponse('/mail/api/account', {
		credentials: 'include'
	}, errors);
	await assertMailResponse(response, errors);
	return normalizeMailAccountResponse(await response.json());
}

export async function fetchMailboxes(errors: MailErrorMessages) {
	if (isSupabaseConfigured()) {
		return normalizeMailboxesResponse(await askTheCompany('mail_mailbox_list', {}, errors));
	}
	const response = await fetchMailResponse('/mail/api/mailboxes', {
		credentials: 'include'
	}, errors);
	await assertMailResponse(response, errors);
	return normalizeMailboxesResponse(await response.json());
}

export async function fetchMailMessages(query: URLSearchParams, errors: MailErrorMessages) {
	if (isSupabaseConfigured()) {
		const searchText = query.get('query') ?? '';
		const input = messageListToolInput(query, searchText);
		const toolName = searchText ? 'mail_message_search' : 'mail_message_list';
		return normalizeMailMessagesResponse(await askTheCompany(toolName, input, errors));
	}
	const response = await fetchMailResponse(`/mail/api/messages?${query}`, {
		credentials: 'include'
	}, errors);
	await assertMailResponse(response, errors);
	return normalizeMailMessagesResponse(await response.json());
}

export async function fetchMailMessage(message: MailMessage, errors: MailErrorMessages) {
	if (isSupabaseConfigured()) {
		return normalizeMailMessageDetailResponse(
			await askTheCompany('mail_message_read', messageTarget(message), errors)
		);
	}
	const response = await fetchMailResponse(`/mail/api/messages/${encodeURIComponent(message.mailbox)}/${message.uid}`, {
		credentials: 'include'
	}, errors);
	await assertMailResponse(response, errors);
	return normalizeMailMessageDetailResponse(await response.json());
}

export async function saveMailAccount(payload: MailAccountWritePayload, errors: MailErrorMessages) {
	if (isSupabaseConfigured()) return keepRecordMailAccount(payload);
	const response = await fetchMailResponse('/mail/api/account', {
		method: 'PUT',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(payload)
	}, errors);
	await assertMailResponse(response, errors);
	return normalizeMailAccountResponse(await response.json());
}

export async function testMailAccount(payload: MailAccountWritePayload, errors: MailErrorMessages) {
	if (isSupabaseConfigured()) return testRecordMailAccount();
	const response = await fetchMailResponse('/mail/api/account/test', {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(payload)
	}, errors);
	await assertMailResponse(response, errors);
}

export async function sendMailMessage(payload: ComposePayload, errors: MailErrorMessages) {
	if (isSupabaseConfigured()) {
		await askTheCompany('mail_message_send', payload, errors);
		return;
	}
	const response = await fetchMailResponse('/mail/api/messages/send', {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(payload)
	}, errors);
	await assertMailResponse(response, errors);
}

export async function moveMailMessage(message: MailMessage, targetMailbox: string, errors: MailErrorMessages) {
	if (isSupabaseConfigured()) {
		await askTheCompany('mail_message_move', { ...messageTarget(message), targetMailbox }, errors);
		return;
	}
	const response = await fetchMailResponse(`/mail/api/messages/${encodeURIComponent(message.mailbox)}/${message.uid}/move`, {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ targetMailbox })
	}, errors);
	await assertMailResponse(response, errors);
}

export async function updateMailMessageFlags(message: MailMessage, seen: boolean, errors: MailErrorMessages) {
	if (isSupabaseConfigured()) {
		await askTheCompany('mail_message_mark', { ...messageTarget(message), seen }, errors);
		return;
	}
	const response = await fetchMailResponse(`/mail/api/messages/${encodeURIComponent(message.mailbox)}/${message.uid}/flags`, {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ seen })
	}, errors);
	await assertMailResponse(response, errors);
}

async function askTheCompany(name: string, input: Record<string, unknown>, errors: MailErrorMessages) {
	try {
		return await invokeTool<unknown>(name, input);
	} catch (error) {
		const said = error instanceof Error ? error.message.trim() : '';
		throw new Error(said || `${errors.fallback} ${errors.serviceUnavailable}`);
	}
}

function messageTarget(message: MailMessage) {
	return { mailbox: message.mailbox, uid: String(message.uid) };
}

function messageListToolInput(query: URLSearchParams, searchText: string) {
	const input: Record<string, unknown> = {};
	const mailbox = query.get('mailbox');
	if (mailbox) input.mailbox = mailbox;
	const limit = Number(query.get('limit'));
	if (Number.isInteger(limit) && limit > 0) input.limit = limit;
	const cursor = query.get('cursor');
	if (cursor) input.cursor = cursor;
	if (searchText) input.query = searchText;
	return input;
}

async function fetchMailResponse(input: RequestInfo | URL, init: RequestInit, errors: MailErrorMessages) {
	try {
		return await fetch(input, init);
	} catch (error) {
		throw new Error(`${errors.fallback} ${errors.serviceUnavailable}`);
	}
}

async function assertMailResponse(response: Response, errors: MailErrorMessages) {
	if (response.ok) return;
	throw new Error(await responseErrorMessage(response, errors));
}

async function responseErrorMessage(response: Response, errors: MailErrorMessages) {
	const message = (await response.text()).trim();
	if (!message || isHTMLResponse(response, message)) return unavailableMessage(response, errors);
	return message;
}

function isHTMLResponse(response: Response, message: string) {
	const contentType = response.headers.get('content-type')?.toLowerCase() ?? '';
	const normalizedMessage = message.toLowerCase();
	return contentType.includes('text/html') || normalizedMessage.startsWith('<!doctype html') || normalizedMessage.startsWith('<html');
}

function unavailableMessage(response: Response, errors: MailErrorMessages) {
	if (response.status >= 500) return `${errors.fallback} ${errors.serviceUnavailable}`;
	return errors.fallback;
}
