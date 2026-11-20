import { expect, test, type Page } from '@playwright/test';
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
const conversations = [
	{ id: 'channel-notice', name: '공지', kind: 'group', isPrivate: false, participantExternalIDs: [readerExternalID, parkExternalID] },
	{ id: 'dm-park', name: '박예시', kind: 'dm', isPrivate: true, participantExternalIDs: [readerExternalID, parkExternalID] }
];

type CompanyComputer = { isConnected: boolean };

async function signInToTheCompanyApp(page: Page, companyComputer: CompanyComputer): Promise<void> {
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
		socket.send(JSON.stringify({ kind: 'presence', isServerConnected: companyComputer.isConnected }));
		socket.onMessage((raw) => {
			const message: { kind: string; capability: string; requestID: string } = JSON.parse(String(raw));
			if (message.kind !== 'call') return;
			const answer = (status: number, body: object) => socket.send(JSON.stringify({ kind: 'result', requestID: message.requestID, status, body }));
			if (!companyComputer.isConnected) return answer(503, { error: 'server_offline' });
			if (message.capability === 'person.conversations.list') return answer(200, { conversations });
			if (message.capability === 'person.identity') return answer(200, { externalID: readerExternalID, name: '이샘플' });
			if (message.capability === 'person.messages.list') return answer(200, { messages: [], hasMoreBefore: false });
			if (message.capability === 'person.inbox.list') return answer(200, { items: [] });
			if (message.capability === 'person.people.list') return answer(200, { people: [{ externalID: parkExternalID, name: '박예시' }] });
			answer(200, {});
		});
	});
}

const rowNamed = (page: Page, name: string) => page.locator('[data-sidebar="menu-item"]').filter({ hasText: name });
const offlineBanner = (page: Page) => page.getByRole('status').filter({ hasText: '회사 컴퓨터가 응답하지 않아요.' });

test('an offline company computer is told once, in words, above the conversations it sent last', async ({ page }) => {
	const companyComputer = { isConnected: true };
	await signInToTheCompanyApp(page, companyComputer);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/messenger?channel=channel-notice');
	await waitForHydration(page);
	await expect(rowNamed(page, '공지')).toBeVisible();

	companyComputer.isConnected = false;
	await page.reload();
	await waitForHydration(page);

	await expect(offlineBanner(page)).toContainText('마지막으로 받은 대화를 보여 드리고 있어요.');
	await expect(rowNamed(page, '공지')).toBeVisible();
	await expect(page.getByText('server_offline')).toHaveCount(0);
	await expect(page.getByRole('alert')).toHaveCount(0);

	companyComputer.isConnected = true;
	await offlineBanner(page).getByRole('button', { name: '다시 연결' }).click();
	await expect(offlineBanner(page)).toHaveCount(0);
	await expect(rowNamed(page, '공지')).toBeVisible();
});

test('with nothing kept from before, the banner says the conversations need the company computer', async ({ page }) => {
	await signInToTheCompanyApp(page, { isConnected: false });
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/messenger');
	await waitForHydration(page);

	await expect(offlineBanner(page)).toContainText('연결되어 있어야 대화를 불러올 수 있어요.');
	await expect(page.getByText('server_offline')).toHaveCount(0);
	await expect(page.getByRole('alert')).toHaveCount(0);
});
