import { expect, test, type Page } from '@playwright/test';

const memberID = '10000000-0000-4000-8000-000000000001';
const companyID = '20000000-0000-4000-8000-000000000001';
const email = 'member@example.com';
const readerExternalID = 'a'.repeat(64);
const parkExternalID = 'b'.repeat(64);
const choiExternalID = 'c'.repeat(64);
const directory = [
	{ id: memberID, name: '이샘플', email, messenger: { buzz: readerExternalID }, is_admin: true },
	{ id: '10000000-0000-4000-8000-000000000002', name: '박예시', email: 'park@example.com', messenger: { buzz: parkExternalID }, is_admin: false },
	{ id: '10000000-0000-4000-8000-000000000003', name: '최견본', email: 'choi@example.com', messenger: { buzz: choiExternalID }, is_admin: false }
];

type Conversation = {
	id: string;
	name: string;
	kind: 'dm' | 'group';
	isHidden?: boolean;
	isPrivate: boolean;
	participantExternalIDs: string[];
};

function companyConversations(): Conversation[] {
	return [
		{ id: 'channel-notice', name: '공지', kind: 'group', isPrivate: false, participantExternalIDs: [readerExternalID] },
		{ id: 'dm-park', name: '박예시', kind: 'dm', isPrivate: true, participantExternalIDs: [readerExternalID, parkExternalID] },
		{ id: 'dm-choi', name: '최견본', kind: 'dm', isPrivate: true, participantExternalIDs: [readerExternalID, choiExternalID] }
	];
}

const inboxFromPark = [
	{ reason: 'direct', isUnread: true, message: { id: 'direct-park', conversationID: 'dm-park', authorExternalID: parkExternalID, body: '자료 보냈어요', postedAt: '2026-10-13T01:00:00.000Z' } }
];

async function signInToTheCompanyApp(page: Page, conversations: Conversation[], asked: string[]): Promise<void> {
	await page.addInitScript(({ memberID, email }) => {
		const issuedAt = Math.floor(Date.now() / 1000);
		const encode = (value: object) => btoa(JSON.stringify(value)).replace(/=/g, '').replace(/\+/g, '-').replace(/\//g, '_');
		localStorage.setItem('sb-127-auth-token', JSON.stringify({ access_token: `${encode({ alg: 'HS256', typ: 'JWT' })}.${encode({ sub: memberID, iat: issuedAt, exp: issuedAt + 3600 })}.fixture`, refresh_token: 'nonfunctional-fixture-refresh', token_type: 'bearer', expires_in: 3600, expires_at: issuedAt + 3600, user: { id: memberID, email, aud: 'authenticated', role: 'authenticated', app_metadata: {}, user_metadata: {}, created_at: new Date().toISOString() } }));
	}, { memberID, email });
	await page.route('http://127.0.0.1:56801/**', async (route) => {
		const url = new URL(route.request().url());
		if (url.pathname === '/auth/v1/user') return route.fulfill({ json: { id: memberID, email, aud: 'authenticated', role: 'authenticated', app_metadata: {}, user_metadata: {}, created_at: new Date().toISOString() } });
		if (url.pathname !== '/rest/v1/member') return route.fulfill({ json: [] });
		if (url.searchParams.has('status')) return route.fulfill({ json: directory });
		const reader = { id: memberID, company_id: companyID, is_admin: true, name: '이샘플', messenger: { buzz: readerExternalID }, company: { slug: 'example-co', locale: 'ko' } };
		const asksForOneRow = (route.request().headers()['accept'] ?? '').includes('vnd.pgrst.object');
		await route.fulfill({ json: asksForOneRow ? reader : [reader] });
	});
	await page.route('**/api/member/me', (route) => route.fulfill({ json: { member: { memberID } } }));
	await page.route('**/auth/session**', (route) => route.fulfill({ json: { authenticated: true, email, isAdmin: true } }));
	await page.route('**/admin/api/locale', (route) => route.fulfill({ json: { locale: 'ko' } }));
	await page.route('**/agent/api/**', (route) => route.fulfill({ json: {} }));
	await page.routeWebSocket('**/company/*/client', (socket) => {
		socket.send(JSON.stringify({ kind: 'presence', isServerConnected: true }));
		socket.onMessage((raw) => {
			const message: { kind: string; capability: string; requestID: string; body?: { conversationID?: string } } = JSON.parse(String(raw));
			if (message.kind !== 'call') return;
			asked.push(message.capability);
			const answer = (body: object) => socket.send(JSON.stringify({ kind: 'result', requestID: message.requestID, status: 200, body }));
			if (message.capability === 'person.conversations.list') return answer({ conversations });
			if (message.capability === 'person.identity') return answer({ externalID: readerExternalID, name: '이샘플' });
			if (message.capability === 'person.messages.list') return answer({ messages: [], hasMoreBefore: false });
			if (message.capability === 'person.inbox.list') return answer({ items: inboxFromPark });
			if (message.capability === 'person.dm.hide') {
				const hidden = conversations.find((conversation) => conversation.id === message.body?.conversationID);
				if (hidden) hidden.isHidden = true;
			}
			if (message.capability === 'person.dm.reveal') {
				const revealed = conversations.find((conversation) => conversation.id === message.body?.conversationID);
				if (revealed) revealed.isHidden = false;
			}
			answer({});
		});
	});
}

const rowNamed = (page: Page, name: string) => page.locator('[data-sidebar="menu-item"]').filter({ hasText: name });

test('a direct conversation is hidden from its menu and the list says so', async ({ page }) => {
	const asked: string[] = [];
	await signInToTheCompanyApp(page, companyConversations(), asked);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/messenger?channel=dm-park');
	await expect(rowNamed(page, '박예시')).toBeVisible();

	await rowNamed(page, '박예시').click({ button: 'right' });
	await expect(page.getByRole('menuitem', { name: '대화 숨기기' })).toBeVisible();

	await page.getByRole('menuitem', { name: '대화 숨기기' }).click();
	await expect(page.getByText('대화를 숨겼습니다.')).toBeVisible();
	await expect(rowNamed(page, '박예시')).toHaveCount(0);
	await expect(rowNamed(page, '최견본')).toBeVisible();
	await expect(page).toHaveURL(/channel=channel-notice/);
	expect(asked).toContain('person.dm.hide');

	await page.reload();
	await expect(rowNamed(page, '최견본')).toBeVisible();
	await expect(rowNamed(page, '박예시')).toHaveCount(0);
});

test('a channel row offers no hiding', async ({ page }) => {
	await signInToTheCompanyApp(page, companyConversations(), []);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/messenger?channel=channel-notice');
	await rowNamed(page, '공지').click({ button: 'right' });
	await expect(page.getByRole('menuitem', { name: '채널 나가기' })).toBeVisible();
	await expect(page.getByRole('menuitem', { name: '대화 숨기기' })).toHaveCount(0);
});

test('a hidden direct conversation opened from the inbox comes back to the list', async ({ page }) => {
	const asked: string[] = [];
	const conversations = companyConversations().map((conversation) =>
		conversation.id === 'dm-park' ? { ...conversation, isHidden: true } : conversation
	);
	await signInToTheCompanyApp(page, conversations, asked);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/messenger');
	await expect(rowNamed(page, '최견본')).toBeVisible();
	await expect(rowNamed(page, '박예시')).toHaveCount(0);

	await page.getByRole('complementary', { name: '수신함' }).locator('[data-inbox-entry="direct-park"]').click();
	await expect(page).toHaveURL(/channel=dm-park/);
	await expect(rowNamed(page, '박예시')).toBeVisible();
	expect(asked).toContain('person.dm.reveal');
});
