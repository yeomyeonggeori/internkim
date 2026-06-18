import { describe, expect, test } from 'bun:test';
import { createDevMailMockResponse, createDevMailMockState } from '../../../dev-mail-mock-plugin';

describe('dev mail mock plugin', () => {
	test('returns an authenticated development session', () => {
		const state = createDevMailMockState('tester@example.com');
		const response = createDevMailMockResponse(state, {
			method: 'GET',
			pathname: '/auth/session',
			searchParams: new URLSearchParams()
		});

		expect(response).toEqual({
			status: 200,
			body: { authenticated: true, email: 'tester@example.com', isAdmin: true }
		});
	});

	test('returns cached bootstrap mail state', () => {
		const state = createDevMailMockState('tester@example.com');
		const response = createDevMailMockResponse(state, {
			method: 'GET',
			pathname: '/mail/api/bootstrap',
			searchParams: new URLSearchParams('mailbox=INBOX')
		});

		expect(response?.status).toBe(200);
		expect(response?.body).toMatchObject({
			hasCachedMailboxes: true,
			hasCachedMessages: true
		});
	});

	test('supports account connection test route', () => {
		const state = createDevMailMockState('tester@example.com');
		const response = createDevMailMockResponse(state, {
			method: 'POST',
			pathname: '/mail/api/account/test',
			searchParams: new URLSearchParams(),
			body: JSON.stringify({ email: 'tester@example.com' })
		});

		expect(response).toEqual({ status: 200, body: { ok: true } });
	});

	test('updates message read state', () => {
		const state = createDevMailMockState('tester@example.com');
		const response = createDevMailMockResponse(state, {
			method: 'POST',
			pathname: '/mail/api/messages/INBOX/103/flags',
			searchParams: new URLSearchParams(),
			body: JSON.stringify({ seen: true })
		});

		expect(response).toEqual({ status: 200, body: { marked: true } });
		expect(state.messages.find((message) => message.uid === 103)?.isRead).toBe(true);
	});
});
