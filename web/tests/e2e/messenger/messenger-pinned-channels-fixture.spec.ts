import { expect, test, type Page } from '@playwright/test';

const memberID = '10000000-0000-4000-8000-000000000001';
const companyID = '20000000-0000-4000-8000-000000000001';
const email = 'member@example.com';
const readerExternalID = 'a'.repeat(64);

type ChannelOrder = { pinnedChannels: string[]; channelOrder: string[] };

type ToolCall = { name: string; input: Record<string, unknown> };

const companyChannels = [
	{ id: 'channel-notice', name: '공지', kind: 'group', isPrivate: false, participantExternalIDs: [readerExternalID] },
	{ id: 'channel-design', name: '디자인', kind: 'group', isPrivate: false, participantExternalIDs: [readerExternalID] },
	{ id: 'channel-dev', name: '개발', kind: 'group', isPrivate: false, participantExternalIDs: [readerExternalID] }
];

function textList(value: unknown): string[] {
	return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : [];
}

function orderAfter(held: ChannelOrder, call: ToolCall): ChannelOrder {
	const channelID = typeof call.input.channelID === 'string' ? call.input.channelID : '';
	if (call.name === 'channel_pin' && !held.pinnedChannels.includes(channelID)) {
		return { ...held, pinnedChannels: [...held.pinnedChannels, channelID] };
	}
	if (call.name === 'channel_unpin') {
		return { ...held, pinnedChannels: held.pinnedChannels.filter((pinnedID) => pinnedID !== channelID) };
	}
	if (call.name === 'channel_order_set') {
		return {
			pinnedChannels: 'pinnedChannels' in call.input ? textList(call.input.pinnedChannels) : held.pinnedChannels,
			channelOrder: 'channelOrder' in call.input ? textList(call.input.channelOrder) : held.channelOrder
		};
	}
	return held;
}

async function signInToTheCompanyApp(page: Page, held: ChannelOrder, calls: ToolCall[]): Promise<void> {
	await page.addInitScript(({ memberID, email }) => {
		const issuedAt = Math.floor(Date.now() / 1000);
		const encode = (value: object) => btoa(JSON.stringify(value)).replace(/=/g, '').replace(/\+/g, '-').replace(/\//g, '_');
		localStorage.setItem('sb-127-auth-token', JSON.stringify({ access_token: `${encode({ alg: 'HS256', typ: 'JWT' })}.${encode({ sub: memberID, iat: issuedAt, exp: issuedAt + 3600 })}.fixture`, refresh_token: 'nonfunctional-fixture-refresh', token_type: 'bearer', expires_in: 3600, expires_at: issuedAt + 3600, user: { id: memberID, email, aud: 'authenticated', role: 'authenticated', app_metadata: {}, user_metadata: {}, created_at: new Date().toISOString() } }));
	}, { memberID, email });
	await page.route('http://127.0.0.1:56801/**', async (route) => {
		const url = new URL(route.request().url());
		if (url.pathname === '/auth/v1/user') return route.fulfill({ json: { id: memberID, email, aud: 'authenticated', role: 'authenticated', app_metadata: {}, user_metadata: {}, created_at: new Date().toISOString() } });
		if (url.pathname !== '/rest/v1/member') return route.fulfill({ json: [] });
		const reader = { id: memberID, company_id: companyID, is_admin: true, name: '이샘플', messenger: { buzz: readerExternalID }, company: { slug: 'example-co', locale: 'ko' } };
		if (url.searchParams.has('status')) return route.fulfill({ json: [reader] });
		const asksForOneRow = (route.request().headers()['accept'] ?? '').includes('vnd.pgrst.object');
		await route.fulfill({ json: asksForOneRow ? reader : [reader] });
	});
	await page.route('**/api/member/me', (route) => route.fulfill({ json: { member: { memberID } } }));
	await page.route('**/auth/session**', (route) => route.fulfill({ json: { authenticated: true, email, isAdmin: true } }));
	await page.route('**/admin/api/locale', (route) => route.fulfill({ json: { locale: 'ko' } }));
	await page.route('**/agent/api/**', (route) => route.fulfill({ json: {} }));
	await page.route('**/api/v1/tools/*/invoke', async (route) => {
		const name = new URL(route.request().url()).pathname.split('/').at(-2) ?? '';
		const asked: unknown = route.request().postDataJSON();
		const input = typeof asked === 'object' && asked !== null ? Reflect.get(asked, 'input') : {};
		const call = { name, input: typeof input === 'object' && input !== null ? { ...input } : {} };
		if (name.startsWith('channel_')) calls.push(call);
		Object.assign(held, orderAfter(held, call));
		if (name.startsWith('channel_')) return route.fulfill({ json: { tool: name, result: { ...held } } });
		if (name === 'notification_settings_get') return route.fulfill({ json: { tool: name, result: { categories: [], mutedConversationIDs: [] } } });
		await route.fulfill({ json: { tool: name, result: {} } });
	});
	await page.routeWebSocket('**/company/*/client', (socket) => {
		socket.send(JSON.stringify({ kind: 'presence', isServerConnected: true }));
		socket.onMessage((raw) => {
			const message: { kind: string; capability: string; requestID: string } = JSON.parse(String(raw));
			if (message.kind !== 'call') return;
			const answer = (body: object) => socket.send(JSON.stringify({ kind: 'result', requestID: message.requestID, status: 200, body }));
			if (message.capability === 'person.conversations.list') return answer({ conversations: companyChannels });
			if (message.capability === 'person.identity') return answer({ externalID: readerExternalID, name: '이샘플' });
			if (message.capability === 'person.messages.list') return answer({ messages: [], hasMoreBefore: false });
			if (message.capability === 'person.inbox.list') return answer({ items: [] });
			answer({});
		});
	});
}

const rowNamed = (page: Page, name: string) => page.locator('[data-sidebar="menu-item"]').filter({ hasText: name });

const sectionTitled = (page: Page, title: string) =>
	page.locator('[data-sidebar="group"]').filter({ has: page.getByRole('button', { name: title, exact: true }) });

const rowsIn = (page: Page, title: string) => sectionTitled(page, title).locator('[data-sidebar="menu-item"]');

async function centerOf(page: Page, name: string): Promise<{ x: number; y: number }> {
	const box = await rowNamed(page, name).boundingBox();
	if (!box) throw new Error(`the channel row ${name} has no box to touch`);
	return { x: box.x + box.width / 2, y: box.y + box.height / 2 };
}

test.describe('on a pointer that can hover', () => {
	test.use({ viewport: { width: 1280, height: 800 } });

	test('a channel pinned from its menu moves to the pinned section and goes back to its place when unpinned', async ({ page }) => {
		const held: ChannelOrder = { pinnedChannels: [], channelOrder: [] };
		const calls: ToolCall[] = [];
		await signInToTheCompanyApp(page, held, calls);
		await page.goto('/messenger?channel=channel-notice');
		await expect(rowsIn(page, '채널')).toHaveText(['공지', '디자인', '개발']);
		await expect(sectionTitled(page, '고정됨')).toHaveCount(0);

		await rowNamed(page, '디자인').click({ button: 'right' });
		await page.getByRole('menuitem', { name: '고정하기' }).click();
		await expect(rowsIn(page, '고정됨')).toHaveText(['디자인']);
		await expect(rowsIn(page, '채널')).toHaveText(['공지', '개발']);
		expect(calls).toContainEqual({ name: 'channel_pin', input: { channelID: 'channel-design' } });

		await rowNamed(page, '디자인').click({ button: 'right' });
		await page.getByRole('menuitem', { name: '고정 해제' }).click();
		await expect(sectionTitled(page, '고정됨')).toHaveCount(0);
		await expect(rowsIn(page, '채널')).toHaveText(['공지', '디자인', '개발']);
		expect(calls).toContainEqual({ name: 'channel_unpin', input: { channelID: 'channel-design' } });
	});

	test('a channel pinned on another device is pinned here when the messenger opens', async ({ page }) => {
		const held: ChannelOrder = { pinnedChannels: ['channel-dev'], channelOrder: [] };
		await signInToTheCompanyApp(page, held, []);
		await page.goto('/messenger?channel=channel-notice');

		await expect(rowsIn(page, '고정됨')).toHaveText(['개발']);
		await expect(rowsIn(page, '채널')).toHaveText(['공지', '디자인']);
	});

	test('dragging a pinned channel onto another reorders the pins', async ({ page }) => {
		const held: ChannelOrder = { pinnedChannels: ['channel-notice', 'channel-design'], channelOrder: [] };
		const calls: ToolCall[] = [];
		await signInToTheCompanyApp(page, held, calls);
		await page.goto('/messenger?channel=channel-notice');
		await expect(rowsIn(page, '고정됨')).toHaveText(['공지', '디자인']);

		await rowNamed(page, '디자인').dragTo(rowNamed(page, '공지'));

		await expect(rowsIn(page, '고정됨')).toHaveText(['디자인', '공지']);
		expect(calls).toContainEqual({ name: 'channel_order_set', input: { pinnedChannels: ['channel-design', 'channel-notice'] } });
	});
});

test.describe('on a touch screen', () => {
	test.use({ hasTouch: true, isMobile: true, viewport: { width: 700, height: 900 } });

	test('holding a channel and moving the finger reorders the list instead of leaving the menu open', async ({ page }) => {
		const held: ChannelOrder = { pinnedChannels: [], channelOrder: [] };
		const calls: ToolCall[] = [];
		await signInToTheCompanyApp(page, held, calls);
		await page.goto('/messenger?channel=channel-notice');
		await expect(rowsIn(page, '채널')).toHaveText(['공지', '디자인', '개발']);

		const from = await centerOf(page, '개발');
		const to = await centerOf(page, '공지');
		const session = await page.context().newCDPSession(page);
		await session.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [from] });
		await page.waitForTimeout(900);
		await expect(page.getByRole('menuitem', { name: '고정하기' })).toBeVisible();
		for (let step = 1; step <= 6; step += 1) {
			const point = { x: from.x, y: from.y + ((to.y - from.y) * step) / 6 };
			await session.send('Input.dispatchTouchEvent', { type: 'touchMove', touchPoints: [point] });
			await page.waitForTimeout(60);
		}
		await session.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });

		await expect(page.getByRole('menuitem', { name: '고정하기' })).toHaveCount(0);
		await expect(rowsIn(page, '채널')).toHaveText(['개발', '공지', '디자인']);
		expect(calls).toContainEqual({ name: 'channel_order_set', input: { channelOrder: ['channel-dev', 'channel-notice', 'channel-design'] } });
		await expect(page).toHaveURL(/channel=channel-notice/);
	});
});
