import { expect, test, type Page, type WebSocketRoute } from '@playwright/test';
import { waitForHydration } from '../support/hydration';

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
	isPrivate: boolean;
	isWithTheAgent?: boolean;
	participantExternalIDs: string[];
	lastMessageAt?: string;
	hasNoMessages?: boolean;
};

const agentExternalID = 'd'.repeat(64);

function companyConversations(): Conversation[] {
	return [
		{ id: 'channel-notice', name: '공지', kind: 'group', isPrivate: false, participantExternalIDs: [readerExternalID] },
		{ id: 'dm-choi', name: '최견본', kind: 'dm', isPrivate: true, participantExternalIDs: [readerExternalID, choiExternalID], lastMessageAt: '2026-11-15T09:00:00.000Z' },
		{ id: 'dm-agent', name: '김인턴', kind: 'dm', isPrivate: true, isWithTheAgent: true, participantExternalIDs: [readerExternalID, agentExternalID], lastMessageAt: '2026-11-01T09:00:00.000Z' },
		{ id: 'dm-park', name: '박예시', kind: 'dm', isPrivate: true, participantExternalIDs: [readerExternalID, parkExternalID], lastMessageAt: '2026-11-16T09:00:00.000Z' }
	];
}

async function signInToTheCompanyApp(page: Page, conversations: Conversation[], sockets: WebSocketRoute[], asked: string[] = []): Promise<void> {
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
			asked.push(message.capability);
			const answer = (body: object) => socket.send(JSON.stringify({ kind: 'result', requestID: message.requestID, status: 200, body }));
			if (message.capability === 'person.conversations.list') return answer({ conversations });
			if (message.capability === 'person.identity') return answer({ externalID: readerExternalID, name: '이샘플' });
			if (message.capability === 'person.messages.list') return answer({ messages: [], hasMoreBefore: false });
			if (message.capability === 'person.inbox.list') return answer({ items: [] });
			answer({});
		});
	});
}

const directMessageNames = (page: Page) =>
	page.locator('[data-reorder-key]').filter({ has: page.locator('[data-sidebar="menu-button"]') }).allInnerTexts();

test('direct conversations run newest message first under the conversation with the agent, and one that hears a message rises', async ({ page }) => {
	const conversations = companyConversations();
	const sockets: WebSocketRoute[] = [];
	await signInToTheCompanyApp(page, conversations, sockets);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/messenger?channel=channel-notice');
	await waitForHydration(page);
	await expect.poll(() => directMessageNames(page)).toEqual(['김인턴', '박예시', '최견본']);

	await page.evaluate(() => {
		const animate = Element.prototype.animate;
		Element.prototype.animate = function (this: Element, ...options: Parameters<Element['animate']>) {
			const key = this.getAttribute('data-reorder-key');
			if (key) document.body.dataset.slidRows = `${document.body.dataset.slidRows ?? ''} ${key}`.trim();
			return animate.apply(this, options);
		};
	});
	const choi = conversations.find((conversation) => conversation.id === 'dm-choi');
	if (choi) choi.lastMessageAt = '2026-11-16T10:00:00.000Z';
	for (const socket of sockets) {
		socket.send(JSON.stringify({ kind: 'deliver', event: { kind: 'message.arrived', conversationID: 'dm-choi', messageID: 'arrived-choi', authorExternalID: choiExternalID } }));
	}

	await expect.poll(() => directMessageNames(page)).toEqual(['김인턴', '최견본', '박예시']);
	const slidRows = (await page.locator('body').getAttribute('data-slid-rows')) ?? '';
	expect(slidRows.split(' ').toSorted()).toEqual(['dm-choi', 'dm-park']);
});

test('while the pointer rests on the direct conversations they hold still, and take the new order once it leaves', async ({ page }) => {
	const conversations = companyConversations();
	const sockets: WebSocketRoute[] = [];
	const asked: string[] = [];
	await signInToTheCompanyApp(page, conversations, sockets, asked);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/messenger?channel=channel-notice');
	await waitForHydration(page);
	await expect.poll(() => directMessageNames(page)).toEqual(['김인턴', '박예시', '최견본']);
	const listReads = () => asked.filter((capability) => capability === 'person.conversations.list').length;
	const listReadsBefore = listReads();

	await page.locator('[data-reorder-key="dm-park"]').hover();
	const choi = conversations.find((conversation) => conversation.id === 'dm-choi');
	if (choi) choi.lastMessageAt = '2026-11-16T10:00:00.000Z';
	for (const socket of sockets) {
		socket.send(JSON.stringify({ kind: 'deliver', event: { kind: 'message.arrived', conversationID: 'dm-choi', messageID: 'arrived-choi', authorExternalID: choiExternalID } }));
	}
	await expect.poll(listReads).toBeGreaterThan(listReadsBefore);
	await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
	expect(await directMessageNames(page)).toEqual(['김인턴', '박예시', '최견본']);

	const newcomerExternalID = 'e'.repeat(64);
	const listReadsBeforeNewcomer = listReads();
	conversations.push({ id: 'dm-newcomer', name: '새 대화', kind: 'dm', isPrivate: true, participantExternalIDs: [readerExternalID, newcomerExternalID], lastMessageAt: '2026-11-16T11:00:00.000Z' });
	for (const socket of sockets) {
		socket.send(JSON.stringify({ kind: 'deliver', event: { kind: 'message.arrived', conversationID: 'dm-newcomer', messageID: 'arrived-newcomer', authorExternalID: newcomerExternalID } }));
	}
	await expect.poll(listReads).toBeGreaterThan(listReadsBeforeNewcomer);
	await expect.poll(() => directMessageNames(page)).toHaveLength(4);
	expect(await directMessageNames(page)).toEqual(['김인턴', '박예시', '최견본', '새 대화']);

	await page.mouse.move(800, 400);
	await expect.poll(() => directMessageNames(page)).toEqual(['김인턴', '새 대화', '최견본', '박예시']);
});

test('a direct conversation nobody has spoken in shows only while it is open, and stays once a message arrives', async ({ page }) => {
	const conversations: Conversation[] = [
		...companyConversations(),
		{ id: 'dm-quiet', name: '조용한 대화', kind: 'dm', isPrivate: true, participantExternalIDs: [readerExternalID, 'f'.repeat(64)], hasNoMessages: true }
	];
	const sockets: WebSocketRoute[] = [];
	await signInToTheCompanyApp(page, conversations, sockets);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/messenger?channel=channel-notice');
	await waitForHydration(page);
	await expect.poll(() => directMessageNames(page)).toEqual(['김인턴', '박예시', '최견본']);

	await page.goto('/messenger?channel=dm-quiet');
	await waitForHydration(page);
	await expect.poll(() => directMessageNames(page)).toContain('조용한 대화');

	await page.locator('[data-sidebar="menu-item"]').filter({ hasText: '공지' }).click();
	await expect.poll(() => directMessageNames(page)).toEqual(['김인턴', '박예시', '최견본']);

	const quiet = conversations.find((conversation) => conversation.id === 'dm-quiet');
	if (quiet) Object.assign(quiet, { hasNoMessages: false, lastMessageAt: '2026-11-16T12:00:00.000Z' });
	for (const socket of sockets) {
		socket.send(JSON.stringify({ kind: 'deliver', event: { kind: 'message.arrived', conversationID: 'dm-quiet', messageID: 'arrived-quiet', authorExternalID: 'f'.repeat(64) } }));
	}
	await expect.poll(() => directMessageNames(page)).toEqual(['김인턴', '조용한 대화', '박예시', '최견본']);
});
