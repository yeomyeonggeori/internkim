import { createDevAdminMockResponse, parseJSONRecord, shouldHandleDevAdminMockRequest, type DevMockRequest, type DevMockResponse } from './dev-admin-mock';
import type { DevMailMockState } from './dev-mail-mock-state';
import type { MailAccount, MailMessage } from './src/routes/mail/mail-types';

export function createDevMailMockResponse(state: DevMailMockState, request: DevMockRequest): DevMockResponse | undefined {
	if (request.method === 'GET' && request.pathname === '/mail/api/bootstrap') {
		return {
			status: 200,
			body: {
				account: state.account,
				mailboxes: state.mailboxes,
				messages: filteredDevMailMessages(state, request),
				nextCursor: '',
				hasCachedMailboxes: true,
				hasCachedMessages: true
			}
		};
	}
	if (request.method === 'GET' && request.pathname === '/mail/api/account') {
		return { status: 200, body: state.account };
	}
	if (request.method === 'PUT' && request.pathname === '/mail/api/account') {
		state.account = { ...state.account, ...mailAccountUpdateFromBody(request.body), isConfigured: true };
		return { status: 200, body: state.account };
	}
	if (request.method === 'POST' && request.pathname === '/mail/api/account/test') {
		return { status: 200, body: { ok: true } };
	}
	if (request.method === 'GET' && request.pathname === '/mail/api/mailboxes') {
		return { status: 200, body: { mailboxes: state.mailboxes } };
	}
	if (request.method === 'GET' && request.pathname === '/mail/api/messages') {
		return { status: 200, body: { messages: filteredDevMailMessages(state, request), nextCursor: '' } };
	}
	if (request.method === 'GET' && request.pathname.startsWith('/mail/api/messages/')) {
		return readDevMailMessageResponse(state, request.pathname);
	}
	if (request.method === 'POST' && request.pathname === '/mail/api/messages/send') {
		return { status: 200, body: { sent: true, appendedTo: state.account.sentMailbox } };
	}
	if (request.method === 'POST' && request.pathname.endsWith('/move')) {
		const movedMessage = devMailPathMessage(state, request.pathname, '/move');
		if (movedMessage) state.messages = state.messages.filter((message) => message.uid !== movedMessage.uid || message.mailbox !== movedMessage.mailbox);
		return { status: 200, body: { moved: true } };
	}
	if (request.method === 'POST' && request.pathname.endsWith('/flags')) {
		const message = devMailPathMessage(state, request.pathname, '/flags');
		const seen = parseJSONRecord(request.body).seen;
		if (message && typeof seen === 'boolean') {
			state.messages = state.messages.map((candidate) =>
				candidate.uid === message.uid && candidate.mailbox === message.mailbox ? { ...candidate, isRead: seen } : candidate
			);
		}
		return { status: 200, body: { marked: true } };
	}
	return createDevAdminMockResponse(state, request);
}

export function shouldHandleDevMailMockRequest(method: string, pathname: string): boolean {
	if (pathname === '/mail/api/bootstrap') return method === 'GET';
	if (pathname === '/mail/api/account') return method === 'GET' || method === 'PUT';
	if (pathname === '/mail/api/account/test') return method === 'POST';
	if (pathname === '/mail/api/mailboxes') return method === 'GET';
	if (pathname === '/mail/api/messages') return method === 'GET';
	if (pathname === '/mail/api/messages/send') return method === 'POST';
	if (pathname.startsWith('/mail/api/messages/')) return method === 'GET' || method === 'POST';
	return shouldHandleDevAdminMockRequest(method, pathname);
}

function filteredDevMailMessages(state: DevMailMockState, request: DevMockRequest): MailMessage[] {
	const mailbox = request.searchParams.get('mailbox') || state.account.defaultMailbox || 'INBOX';
	const query = (request.searchParams.get('query') || '').trim().toLowerCase();
	return state.messages.filter((message) => {
		if (message.mailbox !== mailbox) return false;
		if (!query) return true;
		return [message.subject, message.from, message.preview].some((value) => value.toLowerCase().includes(query));
	});
}

function readDevMailMessageResponse(state: DevMailMockState, pathname: string): DevMockResponse {
	const message = devMailPathMessage(state, pathname, '');
	if (!message) return { status: 404, body: { error: 'message not found' } };
	return { status: 200, body: message };
}

function devMailPathMessage(state: DevMailMockState, pathname: string, suffix: string): MailMessage | undefined {
	const trimmedPath = pathname.slice('/mail/api/messages/'.length).replace(new RegExp(`${suffix}$`), '');
	const parts = trimmedPath.split('/').filter(Boolean);
	if (parts.length !== 2) return undefined;
	const mailbox = decodeURIComponent(parts[0] ?? '');
	const uid = Number.parseInt(parts[1] ?? '', 10);
	if (!mailbox || !Number.isFinite(uid)) return undefined;
	return state.messages.find((message) => message.mailbox === mailbox && message.uid === uid);
}

function mailAccountUpdateFromBody(body: string | undefined): Partial<MailAccount> {
	const parsed = parseJSONRecord(body);
	return Object.fromEntries(
		Object.entries(parsed).filter(([, value]) => typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean')
	) as Partial<MailAccount>;
}
