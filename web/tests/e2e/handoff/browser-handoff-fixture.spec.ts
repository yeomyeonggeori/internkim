import { expect, test, type Page, type WebSocketRoute } from '@playwright/test';

const memberID = '10000000-0000-4000-8000-000000000001';
const companyID = '20000000-0000-4000-8000-000000000001';
const email = 'member@example.com';
const handoffID = 'handoff-sample-1';
const onePixel = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==';

type Call = { capability: string; body: Record<string, unknown> };
type Answer = { status: number; body: unknown };

async function signIn(page: Page): Promise<void> {
	await page.addInitScript(({ memberID, email }) => {
		const issuedAt = Math.floor(Date.now() / 1000);
		const encode = (value: object) => btoa(JSON.stringify(value)).replace(/=/g, '').replace(/\+/g, '-').replace(/\//g, '_');
		localStorage.setItem('sb-127-auth-token', JSON.stringify({ access_token: `${encode({ alg: 'HS256', typ: 'JWT' })}.${encode({ sub: memberID, iat: issuedAt, exp: issuedAt + 3600 })}.fixture`, refresh_token: 'nonfunctional-fixture-refresh', token_type: 'bearer', expires_in: 3600, expires_at: issuedAt + 3600, user: { id: memberID, email, aud: 'authenticated', role: 'authenticated', app_metadata: {}, user_metadata: {}, created_at: new Date().toISOString() } }));
	}, { memberID, email });
	await page.route('http://127.0.0.1:56801/**', async (route) => {
		const url = new URL(route.request().url());
		if (url.pathname === '/auth/v1/user') return route.fulfill({ json: { id: memberID, email, aud: 'authenticated', role: 'authenticated', app_metadata: {}, user_metadata: {}, created_at: new Date().toISOString() } });
		if (url.pathname !== '/rest/v1/member') return route.fulfill({ json: [] });
		const reader = { id: memberID, company_id: companyID, is_admin: false, name: '이샘플', company: { slug: 'example-co', locale: 'ko' } };
		const asksForOneRow = (route.request().headers()['accept'] ?? '').includes('vnd.pgrst.object');
		await route.fulfill({ json: asksForOneRow ? reader : [reader] });
	});
	await page.route('**/api/member/me', (route) => route.fulfill({ json: { member: { memberID } } }));
	await page.route('**/auth/session**', (route) => route.fulfill({ json: { authenticated: true, email, isAdmin: false } }));
	await page.route('**/admin/api/locale', (route) => route.fulfill({ json: { locale: 'ko' } }));
}

async function answerTheCompanyComputer(page: Page, calls: Call[], answer: (call: Call, socket: WebSocketRoute) => Answer): Promise<void> {
	await page.routeWebSocket('**/company/*/client', (socket) => {
		socket.send(JSON.stringify({ kind: 'presence', isServerConnected: true }));
		socket.onMessage((raw) => {
			const message: { kind: string; capability: string; requestID: string; body?: Record<string, unknown> } = JSON.parse(String(raw));
			if (message.kind !== 'call') return;
			const call = { capability: message.capability, body: message.body ?? {} };
			calls.push(call);
			const answered = answer(call, socket);
			socket.send(JSON.stringify({ kind: 'result', requestID: message.requestID, status: answered.status, body: answered.body }));
		});
	});
}

function frameEvent(): string {
	return JSON.stringify({
		kind: 'deliver',
		event: { kind: 'browser.handoff.frame', handoffID, image: onePixel, width: 1280, height: 800, url: 'https://portal.example.com/login', fields: [[20, 20, 300, 30]] }
	});
}

function inputsSent(calls: Call[]): Record<string, unknown>[] {
	return calls
		.filter((call) => call.capability === 'person.browser.handoff.input')
		.flatMap((call) => (Array.isArray(call.body.inputs) ? call.body.inputs : []));
}

test('the requester works in the handed-over browser and hands it back', async ({ page }) => {
	const calls: Call[] = [];
	await signIn(page);
	await answerTheCompanyComputer(page, calls, (call, socket) => {
		if (call.capability === 'person.browser.handoff.watch') {
			setTimeout(() => socket.send(frameEvent()), 50);
			const expiresAt = new Date(Date.now() + 15 * 60_000).toISOString();
			return { status: 200, body: { handoffID, signInOrigin: 'https://portal.example.com', expiresAt, viewport: { width: 1280, height: 800 } } };
		}
		if (call.capability === 'person.browser.handoff.input') return { status: 200, body: { accepted: 1 } };
		if (call.capability === 'person.browser.handoff.finish') return { status: 200, body: { status: 'done' } };
		return { status: 200, body: {} };
	});

	await page.goto(`/example-co/handoff/${handoffID}`);
	const screen = page.getByRole('application', { name: '브라우저를 넘겨받았어요' });
	await expect(screen.locator('img')).toBeVisible();
	await expect(page.getByText('portal.example.com/login')).toBeVisible();

	await screen.click({ position: { x: 40, y: 40 } });
	await page.keyboard.type('sample');
	await expect.poll(() => inputsSent(calls).some((input) => input.type === 'mouse' && input.action === 'down')).toBe(true);
	await expect.poll(() => inputsSent(calls).filter((input) => input.type === 'key' || input.type === 'text').length).toBeGreaterThan(0);

	await page.getByRole('button', { name: '완료' }).click();
	await expect(page.getByText('완료했어요')).toBeVisible();
	expect(calls.find((call) => call.capability === 'person.browser.handoff.finish')?.body).toEqual({ handoffID, outcome: 'done' });
});

test('someone other than the requester is told the browser is not theirs', async ({ page }) => {
	const calls: Call[] = [];
	await signIn(page);
	await answerTheCompanyComputer(page, calls, () => ({ status: 403, body: { error: 'this browser was handed to someone else' } }));

	await page.goto(`/example-co/handoff/${handoffID}`);

	await expect(page.getByText('요청한 사람만 열 수 있어요')).toBeVisible();
	expect(calls.map((call) => call.capability)).not.toContain('person.browser.handoff.input');
});
