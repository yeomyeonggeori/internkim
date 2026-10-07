import { expect, test, type Page, type Route } from '@playwright/test';
import { mkdir } from 'node:fs/promises';
import { taskWeekOfDate } from '../../src/lib/task/task-week-code';

const memberID = '10000000-0000-4000-8000-000000000001';
const companyID = '20000000-0000-4000-8000-000000000001';
const fixedDate = '2026-10-06T03:00:00Z';
const taskTitle = '확인된 샘플 업무';
const isBaseline = process.env.LOADING_EVIDENCE_PHASE === 'before';

function gate() {
	let release = () => {};
	const promise = new Promise<void>(resolve => { release = resolve; });
	return { promise, release };
}

class LoadingFixture {
	boardGate = Promise.resolve();
	historyGate = Promise.resolve();
	peopleGate = Promise.resolve();
	boardStarted = gate();
	historyStarted = gate();
	boardFailure = false;
	historyFailure = false;
	peopleFailure = false;

	async answer(route: Route) {
		const tool = new URL(route.request().url()).pathname.split('/').at(-2);
		if (tool === 'person_list') {
			await this.peopleGate;
			if (this.peopleFailure) { await route.fulfill({ status: 503, json: { error: 'Directory read unavailable' } }); return; }
			await route.fulfill({ json: { result: { requesterID: memberID, count: 1, people: [{ personID: memberID, name: '이샘플', email: 'member@example.com', isAdmin: true, hireDate: '2026-01-01' }] } } });
			return;
		}
		if (tool === 'task_board_get' || tool === 'task_list') {
			const history = tool === 'task_list';
			(history ? this.historyStarted : this.boardStarted).release();
			await (history ? this.historyGate : this.boardGate);
			if (history ? this.historyFailure : this.boardFailure) { await route.fulfill({ status: 503, json: { error: 'Task read unavailable' } }); return; }
			await route.fulfill({ json: { result: {
				scope: 'all', count: 1,
				tasks: [{ taskID: '30000000-0000-4000-8000-000000000001', content: taskTitle, status: 'planned', size: 'M', ownerID: memberID, ownerName: '이샘플', participantIDs: [memberID], participantNames: ['이샘플'], createdAt: fixedDate, weekCode: taskWeekOfDate(new Date(fixedDate)).code }],
				registeredLabels: { businesses: [], types: [], sizes: ['S', 'M', 'L'], statuses: ['requested', 'planned', 'in_progress', 'completed', 'paused', 'rejected', 'stopped'] }
			} } });
			return;
		}
		await route.fulfill({ status: 503, json: { error: 'Supplementary fixture has no answer for this tool' } });
	}
}

async function installFixture(page: Page, fixture: LoadingFixture) {
	expect(await page.evaluate(() => matchMedia('(prefers-reduced-motion: reduce)').matches)).toBe(true);
	await page.clock.setFixedTime(new Date(fixedDate));
	await page.addInitScript(({ memberID }) => {
		const issuedAt = Math.floor(Date.now() / 1000);
		const encode = (value: object) => btoa(JSON.stringify(value)).replace(/=/g, '').replace(/\+/g, '-').replace(/\//g, '_');
		localStorage.setItem('sb-127-auth-token', JSON.stringify({
			access_token: `${encode({ alg: 'HS256', typ: 'JWT' })}.${encode({ sub: memberID, iat: issuedAt, exp: issuedAt + 3600 })}.fixture`,
			refresh_token: 'nonfunctional-fixture-refresh', token_type: 'bearer', expires_in: 3600, expires_at: issuedAt + 3600,
			user: { id: memberID, email: 'member@example.com', aud: 'authenticated', role: 'authenticated', app_metadata: {}, user_metadata: {}, created_at: new Date().toISOString() }
		}));
	}, { memberID });
	await page.route('http://127.0.0.1:56801/**', async route => {
		const pathname = new URL(route.request().url()).pathname;
		if (pathname === '/rest/v1/member') { await route.fulfill({ json: { id: memberID, company_id: companyID, is_admin: true, name: '이샘플', company: { slug: 'example-co', locale: 'ko' } } }); return; }
		if (pathname === '/auth/v1/user') { await route.fulfill({ json: { id: memberID, email: 'member@example.com' } }); return; }
		await route.fulfill({ json: [] });
	});
	await page.route('**/api/member/me', route => route.fulfill({ json: { member: { memberID } } }));
	await page.route('**/agent/api/**', route => route.fulfill({ json: {} }));
	await page.route('**/admin/api/locale', route => route.fulfill({ json: { locale: 'ko' } }));
	await page.route('**/api/v1/tools/*/invoke', route => fixture.answer(route));
}

async function capture(page: Page, scene: string) {
	const directory = process.env.LOADING_EVIDENCE_DIRECTORY;
	if (!directory) return;
	await page.evaluate(async () => {
		await document.fonts.ready;
		if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
		const animations = document.getAnimations().filter(animation => animation.playState === 'running' && animation.effect?.getComputedTiming().iterations !== Infinity);
		await Promise.all(animations.map(animation => animation.finished.catch(() => undefined)));
	});
	await page.locator('main[data-task-progress]').evaluate(element => {
		let current: Element | null = element;
		while (current) {
			current.scrollTo({ top: 0, left: current.scrollLeft, behavior: 'instant' });
			current = current.parentElement;
		}
		window.scrollTo({ top: 0, left: window.scrollX, behavior: 'instant' });
	});
	await page.evaluate(() => new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
	for (const name of ['업무', '보고', '정의', '구성원']) await expect(page.getByRole('tab', { name, exact: true })).toBeInViewport({ ratio: 1 });
	if (scene.startsWith('task-board-') || scene.startsWith('task-list-')) {
		await expect(page.getByRole('tab', { name: '표', exact: true })).toBeInViewport({ ratio: 1 });
		const isPhone = (page.viewportSize()?.width ?? 1280) < 640;
		const weekControl = isPhone ? page.getByLabel('날짜로 주차 이동', { exact: true }) : page.getByRole('button', { name: '이전 주', exact: true });
		await expect(weekControl).toBeInViewport({ ratio: 1 });
	}
	if (!isBaseline) {
		const animatedSkeletons = await page.locator('[data-slot="skeleton"]:visible').evaluateAll(elements => elements.filter(element => getComputedStyle(element).animationName !== 'none').length);
		expect(animatedSkeletons).toBe(0);
	}
	await mkdir(directory, { recursive: true });
	const chrome = await page.locator('[data-task-active-tab]').evaluate(element => ({ top: element.getBoundingClientRect().top, height: element.getBoundingClientRect().height, scrollTop: document.querySelector('[data-app-shell-scroll]')?.scrollTop ?? 0 }));
	await import('node:fs/promises').then(files => files.writeFile(`${directory}/${scene}-${isBaseline ? 'before' : 'after'}.geometry.json`, JSON.stringify(chrome)));
	await page.screenshot({ path: `${directory}/${scene}-${isBaseline ? 'before' : 'after'}.png`, animations: 'allow', caret: 'initial' });
	const capturedChrome = await page.locator('[data-task-active-tab]').evaluate(element => ({ top: element.getBoundingClientRect().top, height: element.getBoundingClientRect().height, scrollTop: document.querySelector('[data-app-shell-scroll]')?.scrollTop ?? 0 }));
	await import('node:fs/promises').then(files => files.writeFile(`${directory}/${scene}-${isBaseline ? 'before' : 'after'}.captured-geometry.json`, JSON.stringify(capturedChrome)));
	expect(capturedChrome).toEqual(chrome);
}

test.use({ locale: 'ko-KR', colorScheme: 'light', contextOptions: { reducedMotion: 'reduce' } });

for (const width of [1280, 390, 320]) {
	test(`cold board and history list reserve content at ${width}px`, async ({ page }) => {
		await page.setViewportSize({ width, height: 900 });
		const fixture = new LoadingFixture();
		const board = gate();
		const history = gate();
		fixture.boardGate = board.promise;
		fixture.historyGate = history.promise;
		await installFixture(page, fixture);
		await page.goto('/example-co/task');
		await fixture.boardStarted.promise;
		await expect(page.locator('[data-task-skeleton]')).toBeVisible();
		await capture(page, `task-board-${width}`);
		board.release();
		await expect(page.locator('[data-task-ready="true"]')).toBeVisible();
		await page.getByRole('tab', { name: '표', exact: true }).click();
		await fixture.historyStarted.promise;
		if (!isBaseline) await expect(page.locator('[data-task-content-skeleton="list"]')).toBeVisible();
		else await expect(page.getByRole('status').filter({ hasText: '전체 업무 내역을 불러오고 있습니다' })).toBeVisible();
		await capture(page, `task-list-${width}`);
		history.release();
		await expect(page.getByText(taskTitle, { exact: true }).last()).toBeVisible();
		await expect(page.locator('[data-task-content-skeleton="list"]')).toHaveCount(0);
		await capture(page, `task-list-loaded-${width}`);
	});
}

for (const width of [1280, 390]) {
	for (const pane of [{ name: '보고', variant: 'report' }, { name: '구성원', variant: 'members' }]) {
		test(`${pane.variant} pending geometry at ${width}px`, async ({ page }) => {
			await page.setViewportSize({ width, height: 900 });
			const fixture = new LoadingFixture();
			const history = gate();
			fixture.historyGate = history.promise;
			await installFixture(page, fixture);
			await page.goto('/example-co/task');
			await expect(page.locator('[data-task-ready="true"]')).toBeVisible();
			await page.getByRole('tab', { name: pane.name, exact: true }).click();
			await fixture.historyStarted.promise;
			if (!isBaseline) await expect(page.locator(`[data-task-content-skeleton="${pane.variant}"]`)).toBeVisible();
			else await expect(page.getByRole('status').filter({ hasText: '전체 업무 내역을 불러오고 있습니다' })).toBeVisible();
			await capture(page, `task-${pane.variant}-${width}`);
			history.release();
			await expect(page.getByRole('status').filter({ hasText: '전체 업무 내역을 불러오고 있습니다' })).toHaveCount(0);
			await capture(page, `task-${pane.variant}-loaded-${width}`);
		});
	}
}

test('initial task failure ends the pending skeleton', async ({ page }) => {
	const fixture = new LoadingFixture();
	fixture.boardFailure = true;
	await installFixture(page, fixture);
	await page.goto('/example-co/task');
	await expect(page.locator('main').filter({ hasText: 'Task read unavailable' }).last()).toBeVisible();
	if (!isBaseline) await expect(page.locator('[data-task-skeleton]')).toHaveCount(0);
	else await expect(page.locator('[data-task-skeleton]')).toBeVisible();
});

test('history failure shows retry rather than a perpetual content skeleton', async ({ page }) => {
	const fixture = new LoadingFixture();
	fixture.historyFailure = true;
	await installFixture(page, fixture);
	await page.goto('/example-co/task');
	await expect(page.locator('[data-task-ready="true"]')).toBeVisible();
	await page.getByRole('tab', { name: '표', exact: true }).click();
	await expect(page.getByRole('button', { name: '다시 불러오기', exact: true })).toBeVisible();
	await expect(page.locator('[data-task-content-skeleton="list"]')).toHaveCount(0);
	fixture.historyFailure = false;
	await page.getByRole('button', { name: '다시 불러오기', exact: true }).click();
	await expect(page.getByText(taskTitle, { exact: true }).last()).toBeVisible();
});

test('changing the board week hides old-week tasks until that week loads', async ({ page }) => {
	test.skip(isBaseline, 'The old-week guard is asserted against implementation');
	const fixture = new LoadingFixture();
	await installFixture(page, fixture);
	await page.goto('/example-co/task');
	await expect(page.locator('[data-task-ready="true"]')).toBeVisible();
	const pending = gate();
	fixture.boardGate = pending.promise;
	fixture.boardStarted = gate();
	await page.getByRole('button', { name: '이전 주', exact: true }).click();
	await fixture.boardStarted.promise;
	await expect(page.locator('[data-task-skeleton]')).toBeVisible();
	await expect(page.locator('[data-task-board-card]').filter({ hasText: taskTitle })).toHaveCount(0);
	pending.release();
	await expect(page.locator('[data-task-skeleton]')).toHaveCount(0);
});

test('same-week refresh retains the settled task list', async ({ page }) => {
	test.skip(isBaseline, 'Settled-list refresh retention is asserted against implementation');
	const fixture = new LoadingFixture();
	await installFixture(page, fixture);
	await page.goto('/example-co/task');
	await expect(page.locator('[data-task-ready="true"]')).toBeVisible();
	await page.getByRole('tab', { name: '표', exact: true }).click();
	await expect(page.getByText(taskTitle, { exact: true }).last()).toBeVisible();
	const pending = gate();
	fixture.historyGate = pending.promise;
	fixture.historyStarted = gate();
	await page.evaluate(async () => { const moduleURL = '/src/lib/components/app-page-actions.svelte.ts'; const actions = await import(moduleURL); void actions.pageActions.refresh(); });
	await fixture.historyStarted.promise;
	await expect(page.getByText(taskTitle, { exact: true }).last()).toBeVisible();
	await expect(page.locator('[data-task-content-skeleton="list"]')).toHaveCount(0);
	pending.release();
	await expect(page.locator('[data-task-ready="true"]')).toBeVisible();
});

test('partial board stops its directory loading status after directory failure', async ({ page }) => {
	test.skip(isBaseline, 'Terminal partial-load behavior is asserted against implementation');
	const fixture = new LoadingFixture();
	const pending = gate();
	fixture.peopleGate = pending.promise;
	fixture.peopleFailure = true;
	await installFixture(page, fixture);
	await page.goto('/example-co/task');
	await expect(page.locator('[data-task-board-card]').filter({ hasText: taskTitle })).toBeVisible();
	await expect(page.getByRole('status').filter({ hasText: '구성원 정보를 준비' })).toBeVisible();
	pending.release();
	await expect(page.locator('main').filter({ hasText: 'Directory read unavailable' }).last()).toBeVisible();
	await expect(page.getByRole('status').filter({ hasText: '구성원 정보를 준비' })).toHaveCount(0);
	await expect(page.locator('[data-task-board-card]').filter({ hasText: taskTitle })).toBeVisible();
});
