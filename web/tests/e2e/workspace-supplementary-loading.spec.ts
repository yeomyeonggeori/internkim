import { expect, test, type Page, type Route } from '@playwright/test';
import { mkdir, writeFile } from 'node:fs/promises';
import { WorkspaceLoadingFixture, prepareWorkspaceLoading } from './workspace-loading-fixture';
import { createDevTasksMockResponse } from '../../dev-tasks-mock-plugin';

const phase = process.env.LOADING_CAPTURE_PHASE ?? 'after';
const output = process.env.LOADING_CAPTURE_DIR;
const companyID = '20000000-0000-4000-8000-000000000001';
const categories = ['A', 'B', 'C'].map((code, index) => ({ code, slug: `folder-${index}`, name: ['Company overview', 'Products', 'Operations'][index], nameKO: ['회사 소개', '제품', '운영'][index], parent: null, description: '' }));
const documents = Array.from({ length: 3 }, (_, index) => ({ documentID: `document-${index}`, documentNumber: null, kind: 'document', documentType: 'memo', title: `Fixture 자료 ${index + 1}`, counterpart: null, language: 'ko', filePath: null, summary: '검토를 위해 공유한 예시 자료입니다.', requesterID: null, issuedAt: '2026-10-06T03:00:00Z', categoryCode: 'A', date: '2026-10-06', period: null, status: 'published', supersedes: null, sha256: null, tags: [], storagePath: null, published: null }));

class SupplementaryFixture extends WorkspaceLoadingFixture {
	readonly statuses = new Map<string, number>();
	isAdmin = true;
	tool(name: string, input: Record<string, unknown>): unknown {
		if (name === 'host_version_get') return { installedVersion: 'v2026.10.06.000001', channel: 'stable', updateMethod: 'apt', isUpdateAvailable: false };
		if (name === 'dataroom_get') return { categories, shares: [], canManage: false };
		if (name === 'company_document_list') return { count: documents.length, documents };
		return super.tool(name, input);
	}
	host(capability: string, body: Record<string, unknown>): { status: number; body: unknown } {
		const status = this.statuses.get(capability);
		if (status) return { status, body: { error: 'Fixture read refused' } };
		if (capability === 'person.files.download') return { status: 200, body: { transfer: { state: 'ready', address: 'http://127.0.0.1:56801/storage/v1/object/asset/loading/sample.md', sizeBytes: 64 } } };
		if (capability === 'person.persona.user') return { status: 200, body: { schemaVersion: 1, callMe: '샘플 님' } };
		const paths: Record<string, string> = { 'person.runs.inbound': '/runs/api/inbound', 'person.runs.llm_call': '/runs/api/llm-call', 'person.runs.turn_input': '/runs/api/turn-input' };
		if (paths[capability]) return createDevTasksMockResponse(this.runs, { method: 'GET', pathname: paths[capability], searchParams: new URLSearchParams(Object.entries(body).map(([key, value]) => [key, String(value)])), body: '' }) ?? { status: 404, body: {} };
		return super.host(capability, body);
	}
}

async function prepare(page: Page, fixture: SupplementaryFixture) {
	await prepareWorkspaceLoading(page, fixture);
	async function reply(route: Route, name: string, json: unknown) {
		await fixture.waitFor(name);
		const status = fixture.statuses.get(name) ?? 200;
		await route.fulfill({ status, json: status >= 400 ? { error: 'Fixture read refused', message: 'Fixture read refused' } : json });
	}
	await page.route('**/api/v1/tokens', route => reply(route, 'tokens', { tokens: [{ name: 'fixture-review-token', permission: 'read', expiresAt: '2027-01-01T00:00:00Z', lastUsedAt: null }] }));
	await page.route('**/api/company/host-setup', route => reply(route, 'host-setup', { company: { id: companyID, name: '예시 회사', slug: 'example-co' }, hasConfiguration: true, lastSeenAt: '2026-10-06T03:00:00Z' }));
	await page.route('**/api/company/box', route => route.fulfill({ json: { connected: null, empty: [] } }));
	await page.route('**/api/member/push-device', route => route.fulfill({ json: { serverKey: '', isServerKeyVaulted: false, hasClaimedDevice: false } }));
	await page.route('**/admin/api/diagnostics/service-logs?*', route => reply(route, 'service-logs', { service: 'blueclaw', lines: ['2026-10-06T03:00:00Z task accepted', '2026-10-06T03:00:01Z model request started', '2026-10-06T03:00:04Z task completed'], count: 3 }));
	await page.route(`**/api/v1/data-room/${companyID}`, route => reply(route, 'public-data-room', { categories: categories.map(({ nameKO, ...category }) => ({ ...category, name_ko: nameKO })), documents: documents.map(document => ({ id: document.documentID, title: document.title, summary: document.summary, category_code: document.categoryCode, document_date: document.date, status: document.status, extension: null })) }));
	await page.route('http://127.0.0.1:56801/**', async route => {
		const path = new URL(route.request().url()).pathname;
		if (path === '/rest/v1/member' && !fixture.isAdmin) return route.fulfill({ json: { id: '10000000-0000-4000-8000-000000000001', company_id: companyID, is_admin: false, name: '이샘플', company: { slug: 'example-co', locale: 'ko' } } });
		if (path === '/auth/v1/passkeys') return reply(route, 'passkeys', [{ id: 'fixture-passkey', friendly_name: 'Sample computer', created_at: '2026-01-01T00:00:00Z', last_used_at: null }]);
		if (path === '/auth/v1/user/oauth/grants') return reply(route, 'grants', []);
		if (path === '/auth/v1/oauth/authorizations/fixture-authorization') return reply(route, 'oauth', { authorization_id: 'fixture-authorization', client: { id: 'fixture-client', name: 'Sample reporting app', uri: 'http://localhost:8080', logo_uri: null }, user: { id: 'sample', email: 'member@example.com' }, redirect_uri: 'http://localhost:8080/callback', scope: 'openid', expires_at: '2026-10-06T04:00:00Z' });
		if (path === '/storage/v1/object/sign/asset/loading/sample.md') {
			if (route.request().method() === 'POST') return route.fulfill({ json: { signedURL: '/object/sign/asset/loading/sample.md?token=fixture' } });
			return route.fulfill({ contentType: 'text/plain', body: '# Fixture preview\n\n검토용 예시 문서입니다.\n\n- 준비한 사항\n- 다음에 확인할 사항' });
		}
		return route.fallback();
	});
}

type Scene = { name: string; path: string; gates: string[]; ready: string; action?: (page: Page) => Promise<void>; focus?: (page: Page) => Promise<void> };
const scenes: Scene[] = [
	{ name: 'settings-security', path: '/example-co/settings', gates: ['tokens'], ready: 'text=fixture-review-token', focus: async page => { await page.getByText('개인 액세스 토큰', { exact: true }).scrollIntoViewIfNeeded(); } },
	{ name: 'settings-setup', path: '/example-co/settings/setup', gates: ['host-setup'], ready: 'text=예시 회사' },
	{ name: 'settings-members', path: '/example-co/settings/setup', gates: ['person_list'], ready: 'text=member1@example.com', focus: async page => { await page.getByRole('button', { name: '구성원 초대', exact: true }).scrollIntoViewIfNeeded(); } },
	{ name: 'data-room', path: '/example-co/files/data-room', gates: ['dataroom_get', 'company_document_list'], ready: 'button:has-text("회사 소개")' },
	{ name: 'public-data-room', path: `/share/${companyID}`, gates: ['public-data-room'], ready: 'tbody tr' },
	{ name: 'file-preview', path: '/example-co/files', gates: ['person.files.download'], ready: 'pre:has-text("Fixture preview")', action: async page => { await page.getByRole('button', { name: /주간-회고-1.md/ }).click(); } },
	{ name: 'run-approvals', path: '/example-co/runs/approvals', gates: ['person.runs.list'], ready: 'button:has-text("이번만 승인")' },
	{ name: 'run-logs', path: '/example-co/runs/dev-task-run-001', gates: ['service-logs'], ready: 'pre:has-text("task accepted")', action: async page => { await page.getByRole('tab', { name: '로그', exact: true }).click(); } },
	{ name: 'run-inbound', path: '/example-co/runs', gates: ['person.runs.inbound'], ready: 'text=다음 주 출시 확정됐어요!', action: async page => { await page.getByRole('tab', { name: '받은 메시지', exact: true }).click(); } },
	{ name: 'run-exchange', path: '/example-co/runs/dev-task-run-001', gates: ['person.runs.llm_call'], ready: 'main pre', action: async page => { await page.getByRole('button', { name: /요청 검토/ }).click(); } },
	{ name: 'run-turn-input', path: '/example-co/runs/dev-task-run-001', gates: ['person.runs.turn_input'], ready: 'main pre', action: async page => { await page.getByRole('tab', { name: '전체 기록', exact: true }).click(); await page.getByRole('button', { name: /task.turn_input/ }).first().click(); } },
	{ name: 'oauth', path: '/oauth/consent?authorization_id=fixture-authorization', gates: ['oauth'], ready: 'button:has-text("허용")' }
];

test.use({ locale: 'ko-KR', colorScheme: 'light', contextOptions: { reducedMotion: 'reduce' } });
async function capture(page: Page, scene: string, width: number, state: string) {
	if (!output) return;
	const securityDesktop = scene === 'settings-security' && width === 1280;
	const stableScroll = scene === 'settings-members' || securityDesktop;
	if (stableScroll) await page.evaluate(async () => {
		await document.fonts.ready;
		if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
		await Promise.all(document.getAnimations().filter(animation => Number.isFinite(animation.effect?.getComputedTiming().endTime)).map(animation => animation.finished.catch(() => {})));
		for (const element of document.querySelectorAll('main,[data-app-shell-scroll]')) element.scrollTo(0, 0);
		window.scrollTo(0, 0);
	});
	let targetGeometry: { x: number; y: number; width: number; height: number } | null = null;
	if (securityDesktop) {
		await expect(page.getByText('Sample computer', { exact: true })).toBeVisible();
		const tokenCard = page.locator('[data-slot="card"]').filter({ has: page.locator('#personal-access-token-name') });
		await tokenCard.evaluate(card => {
			const main = card.closest('main');
			if (!main) throw new Error('Token list has no settings scroll container');
			main.scrollTop += card.getBoundingClientRect().top - 120;
		});
		await expect(tokenCard).toBeInViewport({ ratio: 1 });
		if (state === 'loading' && phase === 'after') await expect(tokenCard.getByRole('status', { name: '개인 액세스 토큰', exact: true })).toBeInViewport({ ratio: 1 });
		if (state === 'ready') await expect(tokenCard.getByText('fixture-review-token', { exact: true })).toBeInViewport({ ratio: 1 });
		targetGeometry = await tokenCard.boundingBox();
		expect(targetGeometry?.y).toBe(120);
	}
	await mkdir(`${output}/${phase}`, { recursive: true });
	const motion = await page.evaluate(() => matchMedia('(prefers-reduced-motion: reduce)').matches);
	expect(motion).toBe(true);
	const scroll = await page.evaluate(() => ({ windowY: window.scrollY, regions: Array.from(document.querySelectorAll('main,[data-app-shell-scroll],[role="tabpanel"]')).map(element => ({ tag: element.tagName, role: element.getAttribute('role'), top: element.scrollTop, left: element.scrollLeft })) }));
	await page.screenshot({ path: `${output}/${phase}/${scene}-${width}-${state}.png`, animations: stableScroll ? 'allow' : 'disabled', caret: 'initial' });
	const afterScroll = await page.evaluate(() => ({ windowY: window.scrollY, regions: Array.from(document.querySelectorAll('main,[data-app-shell-scroll],[role="tabpanel"]')).map(element => ({ tag: element.tagName, role: element.getAttribute('role'), top: element.scrollTop, left: element.scrollLeft })) }));
	if (stableScroll) expect(afterScroll).toEqual(scroll);
	await writeFile(`${output}/${phase}/${scene}-${width}-${state}.json`, JSON.stringify({ route: new URL(page.url()).pathname, viewport: page.viewportSize(), reducedMotionVerified: motion, animations: stableScroll ? 'allow after finite animations settled' : 'disabled', scroll, afterScroll, postScreenshotScrollVerified: stableScroll, targetGeometry, tokenCardFullyInViewport: securityDesktop }, null, 2));
}

for (const width of [1280, 390]) for (const scene of scenes) {
	test(`${scene.name} supplementary pending and ready at ${width}px`, async ({ page }) => {
		await page.setViewportSize({ width, height: 900 });
		const fixture = new SupplementaryFixture();
		if (scene.name === 'settings-security') fixture.isAdmin = false;
		for (const gate of scene.gates) fixture.hold(gate);
		await prepare(page, fixture);
		await page.goto(scene.path);
		expect(await page.evaluate(() => matchMedia('(prefers-reduced-motion: reduce)').matches)).toBe(true);
		await scene.action?.(page);
		for (const gate of scene.gates) await expect.poll(() => fixture.requested.has(gate)).toBe(true);
		await scene.focus?.(page);
		if (phase === 'after') expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
		await capture(page, scene.name, width, 'loading');
		for (const gate of scene.gates) fixture.release(gate);
		await expect(page.locator(scene.ready).filter({ visible: true }).first()).toBeVisible();
		await scene.focus?.(page);
		await capture(page, scene.name, width, 'ready');
	});
}

for (const status of [401, 403]) for (const target of ['memory', 'files', 'mail', 'runs', 'run-detail']) {
	test(`${target} removes retained records after current ${status} read denial`, async ({ page }) => {
		test.skip(phase === 'before', 'The original screen did not safely distinguish authorization denial');
		const fixture = new SupplementaryFixture();
		if (target === 'run-detail') fixture.runs.taskRuns[0].status = 'running';
		await prepare(page, fixture);
		await page.goto(target === 'run-detail' ? '/example-co/runs/dev-task-run-001' : `/example-co/${target}`);
		if (target === 'memory') {
			const fact = page.locator('main button[aria-pressed]').filter({ has: page.locator('p') }).first();
			await expect(fact).toBeVisible();
			await fact.click();
			fixture.statuses.set('person.memory.facts', status);
			await page.getByRole('main').getByRole('button', { name: '새로고침', exact: true }).click();
			await expect(page.getByRole('alert')).toBeVisible();
			await expect(fact).toHaveCount(0);
			await expect(page.locator('#memory-layers-title')).toHaveCount(0);
		} else if (target === 'files') {
			const entry = page.getByRole('button', { name: /주간-회고-1.md/ });
			await expect(entry).toBeVisible();
			await entry.click();
			await expect(page.locator('pre:has-text("Fixture preview")')).toBeVisible();
			fixture.statuses.set('person.files.list', status);
			await page.getByRole('tabpanel', { name: '워크스페이스' }).getByRole('button', { name: '새로고침', exact: true }).click();
			await expect(page.getByRole('alert')).toBeVisible();
			await expect(entry).toHaveCount(0);
			await expect(page.locator('pre:has-text("Fixture preview")')).toHaveCount(0);
		} else if (target === 'mail') {
			const message = page.getByRole('button', { name: /메일 캐시 동작 확인/ }).first();
			await expect(message).toBeVisible();
			await page.route('**/api/v1/tools/mail_message_list/invoke', route => route.fulfill({ status, json: { error: 'Fixture access denied' } }));
			await page.getByRole('button', { name: '새로고침', exact: true }).click();
			await expect(page.getByRole('alert')).toBeVisible();
			await expect(message).toHaveCount(0);
		} else if (target === 'runs') {
			await expect(page.locator('main tbody').first()).toBeVisible();
			fixture.statuses.set('person.runs.list', status);
			await page.getByRole('button', { name: '새로고침', exact: true }).click();
			await expect(page.getByText('작업 목록을 불러오지 못했습니다.', { exact: true })).toBeVisible();
			await expect(page.locator('main tbody')).toHaveCount(0);
		} else {
			await expect(page.locator('main h1')).toBeVisible();
			fixture.statuses.set('person.runs.detail', status);
			await expect(page.locator('main h1')).toHaveCount(0);
			await expect(page.getByRole('tab', { name: '로그', exact: true })).toHaveCount(0);
		}
	});
}

test('security and public data-room first-read failures do not masquerade as empty', async ({ page }) => {
	await page.setViewportSize({ width: 1280, height: 900 });
	const fixture = new SupplementaryFixture();
	fixture.isAdmin = false;
	fixture.statuses.set('tokens', 503);
	fixture.statuses.set('public-data-room', 503);
	await prepare(page, fixture);
	await page.goto('/example-co/settings');
	await expect(page.getByText('토큰 목록을 불러오지 못했습니다.', { exact: true }).first()).toBeVisible();
	if (phase === 'after') await expect(page.getByText('아직 만든 토큰이 없습니다.', { exact: true })).toHaveCount(0);
	await page.getByText('개인 액세스 토큰', { exact: true }).scrollIntoViewIfNeeded();
	await capture(page, 'settings-security', 1280, 'error');
	await page.goto(`/share/${companyID}`);
	await expect(page.getByRole('alert')).toBeVisible();
	if (phase === 'after') {
		await expect(page.getByText(/^(No documents yet\.|보관된 자료가 없습니다\.)$/)).toHaveCount(0);
		await expect(page.getByText('Loading…', { exact: true })).toHaveCount(0);
	}
	await capture(page, 'public-data-room', 1280, 'error');
});

for (const changed of ['company', 'role', 'account']) {
	test(`organization ${changed} change hides old cached members and ignores a late previous directory`, async ({ page }) => {
		test.skip(phase === 'before', 'The original last-seen organization cache was not authority scoped');
		const fixture = new SupplementaryFixture();
		await prepare(page, fixture);
		const identity = { memberID: '10000000-0000-4000-8000-000000000001', companyID, email: 'member@example.com', isAdmin: true };
		let directoryPhase = 0;
		let oldRefreshReturned = false;
		let reads = 0;
		await page.route('http://127.0.0.1:56801/rest/v1/member*', route => route.fulfill({ json: { id: identity.memberID, company_id: identity.companyID, is_admin: identity.isAdmin, name: '이샘플', company: { slug: 'example-co', locale: 'ko' } } }));
		await page.route('**/api/v1/tools/person_list/invoke', async route => {
			const requestedPhase = directoryPhase;
			const requestNumber = ++reads;
			const name = requestedPhase === 0 ? '이샘플' : '박예시';
			const person = { personID: identity.memberID, name, email: identity.email, isAdmin: identity.isAdmin, teamID: 'team-product', jobTitle: '프로덕트 디자이너' };
			await fixture.waitFor(`directory-${requestedPhase}`);
			await route.fulfill({ json: { result: { people: [person], count: 1 } } });
			if (requestedPhase === 0 && requestNumber > 1) oldRefreshReturned = true;
		});
		await page.goto('/example-co/organization');
		await expect(page.locator('main').getByText('이샘플', { exact: true }).first()).toBeVisible();
		await page.locator('a[href="/example-co/runs"]').first().click();
		await expect(page.locator('main tbody').first()).toBeVisible();
		fixture.hold('directory-0');
		await page.locator('a[href="/example-co/organization"]').first().click();
		await expect.poll(() => reads).toBeGreaterThan(1);
		await expect(page.locator('main').getByText('이샘플', { exact: true }).first()).toBeVisible();
		directoryPhase = 1;
		fixture.hold('directory-1');
		if (changed === 'company') identity.companyID = '20000000-0000-4000-8000-000000000002';
		if (changed === 'role') identity.isAdmin = false;
		if (changed === 'account') {
			identity.memberID = '10000000-0000-4000-8000-000000000002';
			identity.email = 'other@example.com';
			await page.evaluate(({ id, email }) => {
				const saved = JSON.parse(localStorage.getItem('sb-127-auth-token') || '{}');
				saved.user = { ...saved.user, id, email };
				const payload = { sub: id, iat: Math.floor(Date.now() / 1000), exp: Math.floor(Date.now() / 1000) + 36000 };
				const encoded = btoa(JSON.stringify(payload)).replace(/=/g, '').replace(/\+/g, '-').replace(/\//g, '_');
				saved.access_token = `${saved.access_token.split('.')[0]}.${encoded}.fixture`;
				localStorage.setItem('sb-127-auth-token', JSON.stringify(saved));
			}, { id: identity.memberID, email: identity.email });
		}
		await page.evaluate(() => { window.dispatchEvent(new Event('blur')); window.dispatchEvent(new Event('focus')); });
		await expect.poll(() => fixture.requested.has('directory-1')).toBe(true);
		await expect(page.locator('main').getByText('이샘플', { exact: true })).toHaveCount(0);
		await expect(page.getByTestId('organization-skeleton')).toBeVisible();
		fixture.release('directory-0');
		await expect.poll(() => oldRefreshReturned).toBe(true);
		await expect(page.locator('main').getByText('이샘플', { exact: true })).toHaveCount(0);
		await expect(page.getByTestId('organization-skeleton')).toBeVisible();
		fixture.release('directory-1');
		await expect(page.locator('main').getByText('박예시', { exact: true }).first()).toBeVisible();
		await expect(page.locator('main').getByText('이샘플', { exact: true })).toHaveCount(0);
	});
}
