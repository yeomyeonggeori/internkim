import { isSupabaseConfigured } from '$lib/supabase';
import { mailRequestHeaders } from './mail-request-actor';
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

export async function fetchMailBootstrap(actorEmail: string, query: URLSearchParams, errors: MailErrorMessages) {
	const response = await fetchMailResponse(`/mail/api/bootstrap?${query}`, {
		method: 'GET',
		credentials: 'include',
		headers: mailRequestHeaders(actorEmail)
	}, errors);
	await assertMailResponse(response, errors);
	return normalizeMailBootstrapResponse(await response.json());
}

export async function fetchMailAccount(actorEmail: string, errors: MailErrorMessages) {
	if (isSupabaseConfigured()) return recordMailAccount();
	const response = await fetchMailResponse('/mail/api/account', {
		credentials: 'include',
		headers: mailRequestHeaders(actorEmail)
	}, errors);
	await assertMailResponse(response, errors);
	return normalizeMailAccountResponse(await response.json());
}

export async function fetchMailboxes(actorEmail: string, errors: MailErrorMessages) {
	const response = await fetchMailResponse('/mail/api/mailboxes', {
		credentials: 'include',
		headers: mailRequestHeaders(actorEmail)
	}, errors);
	await assertMailResponse(response, errors);
	return normalizeMailboxesResponse(await response.json());
}

export async function fetchMailMessages(actorEmail: string, query: URLSearchParams, errors: MailErrorMessages) {
	const response = await fetchMailResponse(`/mail/api/messages?${query}`, {
		credentials: 'include',
		headers: mailRequestHeaders(actorEmail)
	}, errors);
	await assertMailResponse(response, errors);
	return normalizeMailMessagesResponse(await response.json());
}

export async function fetchMailMessage(actorEmail: string, message: MailMessage, errors: MailErrorMessages) {
	const response = await fetchMailResponse(`/mail/api/messages/${encodeURIComponent(message.mailbox)}/${message.uid}`, {
		credentials: 'include',
		headers: mailRequestHeaders(actorEmail)
	}, errors);
	await assertMailResponse(response, errors);
	return normalizeMailMessageDetailResponse(await response.json());
}

export async function saveMailAccount(actorEmail: string, payload: MailAccountWritePayload, errors: MailErrorMessages) {
	if (isSupabaseConfigured()) return keepRecordMailAccount(payload);
	const response = await fetchMailResponse('/mail/api/account', {
		method: 'PUT',
		credentials: 'include',
		headers: mailRequestHeaders(actorEmail, { 'Content-Type': 'application/json' }),
		body: JSON.stringify(payload)
	}, errors);
	await assertMailResponse(response, errors);
	return normalizeMailAccountResponse(await response.json());
}

export async function testMailAccount(actorEmail: string, payload: MailAccountWritePayload, errors: MailErrorMessages) {
	if (isSupabaseConfigured()) return testRecordMailAccount();
	const response = await fetchMailResponse('/mail/api/account/test', {
		method: 'POST',
		credentials: 'include',
		headers: mailRequestHeaders(actorEmail, { 'Content-Type': 'application/json' }),
		body: JSON.stringify(payload)
	}, errors);
	await assertMailResponse(response, errors);
}

export async function sendMailMessage(actorEmail: string, payload: ComposePayload, errors: MailErrorMessages) {
	const response = await fetchMailResponse('/mail/api/messages/send', {
		method: 'POST',
		credentials: 'include',
		headers: mailRequestHeaders(actorEmail, { 'Content-Type': 'application/json' }),
		body: JSON.stringify(payload)
	}, errors);
	await assertMailResponse(response, errors);
}

export async function moveMailMessage(actorEmail: string, message: MailMessage, targetMailbox: string, errors: MailErrorMessages) {
	const response = await fetchMailResponse(`/mail/api/messages/${encodeURIComponent(message.mailbox)}/${message.uid}/move`, {
		method: 'POST',
		credentials: 'include',
		headers: mailRequestHeaders(actorEmail, { 'Content-Type': 'application/json' }),
		body: JSON.stringify({ targetMailbox })
	}, errors);
	await assertMailResponse(response, errors);
}

export async function updateMailMessageFlags(actorEmail: string, message: MailMessage, seen: boolean, errors: MailErrorMessages) {
	const response = await fetchMailResponse(`/mail/api/messages/${encodeURIComponent(message.mailbox)}/${message.uid}/flags`, {
		method: 'POST',
		credentials: 'include',
		headers: mailRequestHeaders(actorEmail, { 'Content-Type': 'application/json' }),
		body: JSON.stringify({ seen })
	}, errors);
	await assertMailResponse(response, errors);
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
