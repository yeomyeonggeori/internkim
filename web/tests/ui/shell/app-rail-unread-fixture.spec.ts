import { expect, test, type Locator, type Page, type WebSocketRoute } from '@playwright/test';
import { waitForHydration } from '../support/hydration';

const memberID = '10000000-0000-4000-8000-000000000001';
const companyID = '20000000-0000-4000-8000-000000000001';
const email = 'member@example.com';
const readerExternalID = 'a'.repeat(64);
const parkExternalID = 'b'.repeat(64);
const directory = [
	{ id: memberID, name: '이샘플', email, messenger: { buzz: readerExternalID }, is_admin: true },
	{ id: '10000000-0000-4000-8000-000000000002', name: '박예시', email: 'park@example.com', messenger: { buzz: parkExternalID }, is_admin: false }
];

type Conversation = {
	id: string;
	name: string;
	kind: 'dm' | 'group';
	isPrivate: boolean;
	participantExternalIDs: string[];
	unread: { status: 'exact'; value: number };
};

function companyConversations(): Conversation[] {
	return [
		{ id: 'channel-notice', name: '공지', kind: 'group', isPrivate: false, participantExternalIDs: [readerExternalID], unread: { status: 'exact', value: 0 } },
		{ id: 'dm-park', name: '박예시', kind: 'dm', isPrivate: true, participantExternalIDs: [readerExternalID, parkExternalID], unread: { status: 'exact', value: 0 } }
	];
}

async function signInToTheCompanyApp(page: Page, conversations: Conversation[], sockets: WebSocketRoute[]): Promise<void> {
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
		sockets.push(socket);
		socket.send(JSON.stringify({ kind: 'presence', isServerConnected: true }));
		socket.onMessage((raw) => {
			const message: { kind: string; capability: string; requestID: string } = JSON.parse(String(raw));
			if (message.kind !== 'call') return;
			const answer = (body: object) => socket.send(JSON.stringify({ kind: 'result', requestID: message.requestID, status: 200, body }));
			if (message.capability === 'person.conversations.list') return answer({ conversations });
			if (message.capability === 'person.identity') return answer({ externalID: readerExternalID, name: '이샘플' });
			if (message.capability === 'person.messages.list') return answer({ messages: [], hasMoreBefore: false });
			if (message.capability === 'person.inbox.list') return answer({ items: [] });
			answer({});
		});
	});
}

function messengerRailItem(page: Page): Locator {
	return page.locator('[data-app-rail] [data-sidebar="menu-item"]').filter({ has: page.getByRole('link', { name: '메신저', exact: true }) });
}

async function recordRailMotion(page: Page): Promise<void> {
	await page.evaluate(() => {
		document.addEventListener('animationstart', (event) => {
			if (!(event.target instanceof Element) || !event.target.closest('[data-app-rail]')) return;
			const moved = event.target.tagName === 'svg' ? 'icon' : 'mark';
			document.body.dataset.railMotion = `${document.body.dataset.railMotion ?? ''} ${moved}`.trim();
		});
	});
}

function railMotion(page: Page): Promise<string[]> {
	return page.evaluate(() => (document.body.dataset.railMotion ?? '').split(' ').filter(Boolean).toSorted());
}

function hearMessageFromPark(conversations: Conversation[], sockets: WebSocketRoute[], unread: number, messageID: string): void {
	const park = conversations.find((conversation) => conversation.id === 'dm-park');
	if (park) park.unread = { status: 'exact', value: unread };
	for (const socket of sockets) {
		socket.send(JSON.stringify({ kind: 'deliver', event: { kind: 'message.arrived', conversationID: 'dm-park', messageID, authorExternalID: parkExternalID } }));
	}
}

test('a direct message that arrives while another page is open is counted on the folded rail, and the icon and the count move once', async ({ page }) => {
	const conversations = companyConversations();
	const sockets: WebSocketRoute[] = [];
	await signInToTheCompanyApp(page, conversations, sockets);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/example-co/memory');
	await waitForHydration(page);
	const badge = messengerRailItem(page).getByTestId('app-rail-nav-badge');
	await expect(messengerRailItem(page)).toBeVisible();
	await expect.poll(() => sockets.length).toBeGreaterThan(0);
	await expect(badge).toHaveCount(0);
	await recordRailMotion(page);

	hearMessageFromPark(conversations, sockets, 2, 'arrived-park-1');

	await expect(badge).toHaveText(/^읽지 않은 메시지\s*2$/);
	await expect.poll(() => railMotion(page)).toEqual(['icon', 'mark']);

	hearMessageFromPark(conversations, sockets, 0, 'arrived-park-2');
	await expect(badge).toHaveCount(0);
	await page.evaluate(() => delete document.body.dataset.railMotion);
	await page.goto('/example-co/messenger?channel=channel-notice');
	await waitForHydration(page);
	await recordRailMotion(page);
	hearMessageFromPark(conversations, sockets, 1, 'arrived-park-3');

	await expect(badge).toHaveText(/^읽지 않은 메시지\s*1$/);
	expect(await railMotion(page)).toEqual([]);
});

test('on an expanded rail only the count moves when a direct message arrives', async ({ page }) => {
	const conversations = companyConversations();
	const sockets: WebSocketRoute[] = [];
	await page.addInitScript(() => localStorage.setItem('internkim.app-rail-mode', 'expanded'));
	await signInToTheCompanyApp(page, conversations, sockets);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/example-co/memory');
	await waitForHydration(page);
	await expect(messengerRailItem(page)).toBeVisible();
	await expect.poll(() => sockets.length).toBeGreaterThan(0);
	await recordRailMotion(page);

	hearMessageFromPark(conversations, sockets, 1, 'arrived-park-1');

	await expect(messengerRailItem(page).getByTestId('app-rail-nav-badge')).toHaveText(/^읽지 않은 메시지\s*1$/);
	await expect.poll(() => railMotion(page)).toEqual(['mark']);
});
