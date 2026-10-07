import { expect, test, type Page } from '@playwright/test';
import { mkdir, writeFile } from 'node:fs/promises';
import { WorkspaceLoadingFixture, prepareWorkspaceLoading } from './workspace-loading-fixture';
	import { defaultLeavePolicy } from '../../src/lib/attendance/leave-policy-defaults';

const phase = process.env.EMPTY_CAPTURE_PHASE ?? 'after';
const output = process.env.EMPTY_CAPTURE_DIR;
test.use({ locale: 'ko-KR', colorScheme: 'light', contextOptions: { reducedMotion: 'reduce' } });

class EmptyWorkspaceFixture extends WorkspaceLoadingFixture {
	readonly statuses = new Map<string, number>();
	empty = true;
	tool(name: string, input: Record<string, unknown>): unknown {
		if (name === 'attendance_leave_policy_get') return defaultLeavePolicy();
		if (name === 'company_holiday_list') return { count: 0, holidays: [] };
		if (name === 'host_version_get') return { installedVersion: 'v2026.10.06.000001', channel: 'stable', updateMethod: 'apt', isUpdateAvailable: false };
		if (name === 'task_board_get' || name === 'task_list') return { scope: 'all', count: 0, tasks: [], registeredLabels: { businesses: [], types: [], sizes: ['S', 'M', 'L'], statuses: ['requested', 'planned', 'in_progress', 'completed', 'paused', 'rejected', 'stopped'] } };
		if (this.empty) {
			if (name === 'person_list') return { requesterID: '10000000-0000-4000-8000-000000000001', people: [], count: 0 };
			if (name === 'team_list') return { teams: [], count: 0 };
			if (name === 'crm_organization_list') return { organizations: [] };
			if (name === 'crm_contact_list') return { contacts: [] };
			if (name === 'crm_opportunity_list') return { opportunities: [] };
			if (name === 'crm_activity_list') return { activities: [], registeredLabels: { businesses: [], types: [] } };
			if (name === 'mail_message_list' || name === 'mail_message_search') return { messages: [], nextCursor: '' };
		}
		if (name === 'mail_message_search') return { messages: [], nextCursor: '' };
		return super.tool(name, input);
	}
	host(capability: string, body: Record<string, unknown>): { status: number; body: unknown } {
		const status = this.statuses.get(capability);
		if (status) return { status, body: { error: 'Fixture read unavailable' } };
		if (capability === 'person.skills.list') return { status: 200, body: { skills: this.empty ? [] : [{ name: 'fixture-loaded-skill', description: 'Synthetic review procedure', path: '/skills/sample', toolReferences: [] }], unavailableSkills: [] } };
		if (capability === 'person.agent_learning.skills.list') return { status: 200, body: { skills: this.empty ? [] : [{ id: 'fixture-learned-skill', version: 1, audience: 'company', description: 'Synthetic learned procedure', instruction: 'Review sample data', evidenceIDs: [], reason: '', verification: 'evidence-reviewed', status: 'active', protected: false, createdAt: '2026-10-06T03:00:00Z', updatedAt: '2026-10-06T03:00:00Z' }] } };
		if (capability === 'person.agent_learning.settings.get') return { status: 200, body: { enabled: true, activeLimit: 20 } };
		const response = super.host(capability, body);
		if (!this.empty || response.status >= 400) return response;
		if (capability === 'person.files.list') return { status: 200, body: { entries: [] } };
		if (capability === 'person.memory.facts') return { status: 200, body: { ...(response.body as object), facts: [] } };
		if (capability === 'person.memory.schedules') return { status: 200, body: { schedules: [], totalCount: 0 } };
		if (capability === 'person.runs.list') return { status: 200, body: { taskRuns: [], totalCount: 0, dailyCostSummaries: [] } };
		return response;
	}
}

async function prepareAdmin(page: Page, fixture: EmptyWorkspaceFixture) {
	await prepareWorkspaceLoading(page, fixture);
	await page.route('http://127.0.0.1:56801/auth/v1/user', route => route.fulfill({ json: { id: '10000000-0000-4000-8000-000000000001', email: 'member@example.com' } }));
	await page.route('**/api/v1/tokens', route => route.fulfill({ json: { tokens: [] } }));
}

async function capture(page: Page, scene: string, state: string) {
	if (!output) return;
	await mkdir(`${output}/${phase}`, { recursive: true });
	await page.evaluate(() => document.fonts.ready);
	const viewport = page.viewportSize();
	await page.screenshot({ path: `${output}/${phase}/${scene}-${viewport?.width}-${state}.png`, animations: 'disabled' });
	await writeFile(`${output}/${phase}/${scene}-${viewport?.width}-${state}.json`, JSON.stringify({ route: new URL(page.url()).pathname, viewport, state, reducedMotion: await page.evaluate(() => matchMedia('(prefers-reduced-motion: reduce)').matches), scroll: await page.evaluate(() => ({ windowY: window.scrollY, main: document.querySelector('main')?.scrollTop })) }, null, 2));
}

const scenes = [
	{ name: 'crm-accounts', path: '/crm', gate: 'crm_organization_list', title: /등록된 관계처가 없습니다|조건에 맞는 관계처가 없습니다/ },
	{ name: 'crm-contacts', path: '/crm', gate: 'crm_organization_list', tab: '연락처', title: /등록된 연락처가 없습니다|조건에 맞는 연락처가 없습니다/ },
	{ name: 'crm-deals', path: '/crm', gate: 'crm_organization_list', tab: '거래', title: /등록된 거래가 없습니다|이 단계에 거래가 없습니다/ },
	{ name: 'crm-activities', path: '/crm', gate: 'crm_organization_list', tab: '활동', title: /등록된 활동이 없습니다|조건에 맞는 활동이 없습니다/ },
	{ name: 'organization', path: '/organization', gate: 'person_list', title: '표시할 조직도 구성원이 없습니다.' },
	{ name: 'files', path: '/files', gate: 'person.files.list', title: '이 폴더는 비어 있습니다.' },
	{ name: 'mail', path: '/mail', gate: 'mail_message_list', title: '표시할 메일이 없습니다' },
	{ name: 'memory', path: '/memory', gate: 'person.memory.facts', title: '아직 볼 수 있는 기억이 없습니다.' },
	{ name: 'schedules', path: '/memory/schedules', gate: 'person.memory.schedules', title: /아직 예약 작업이 없습니다\.|선택한 필터에 맞는 예약 작업이 없습니다\./ },
	{ name: 'runs', path: '/runs', gate: 'person.runs.list', title: '표시할 작업이 없습니다.' },
	{ name: 'approvals', path: '/runs/approvals', gate: 'person.runs.list', title: '승인을 기다리는 작업이 없습니다.' }
];

for (const width of [1280, 390]) for (const scene of scenes) {
	test(`${scene.name} settles from pending to a real empty state at ${width}px`, async ({ page }) => {
		await page.setViewportSize({ width, height: 900 });
		const fixture = new EmptyWorkspaceFixture();
		fixture.hold(scene.gate);
		await prepareWorkspaceLoading(page, fixture);
		await page.goto(`/example-co${scene.path}`);
		await expect.poll(() => fixture.requested.has(scene.gate)).toBe(true);
		await expect(page.getByText(scene.title, { exact: true })).toHaveCount(0);
		if (phase === 'after') await expect(page.locator('[data-slot="empty"]:visible')).toHaveCount(0);
		if (!scene.tab) await capture(page, scene.name, 'loading');
		fixture.release(scene.gate);
		if (scene.tab) await page.getByRole('tab', { name: scene.tab, exact: true }).click();
		await expect(page.getByText(scene.title, { exact: true }).first()).toBeVisible();
		await expect(page.locator('[data-slot="skeleton"]:visible')).toHaveCount(0);
		if (phase === 'after') {
			await expect(page.locator('[data-slot="empty-title"]').filter({ hasText: scene.title, visible: true }).first()).toBeVisible();
			expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
		}
		await capture(page, scene.name, 'empty');
	});
}

test('mail search and unread empties explain filters and can restore the loaded list', async ({ page }) => {
	test.skip(phase === 'before', 'Behavioral checks for the correction');
	const fixture = new EmptyWorkspaceFixture();
	fixture.empty = false;
	await prepareWorkspaceLoading(page, fixture);
	await page.goto('/example-co/mail');
	await expect(page.getByRole('button', { name: /메일 캐시 동작 확인/ }).first()).toBeVisible();
	await page.getByRole('tab', { name: '읽지 않음', exact: true }).click();
	await expect(page.locator('[data-slot="empty-title"]')).toHaveText('읽지 않은 메일이 없습니다');
	await capture(page, 'mail', 'filtered-empty');
	await page.getByRole('button', { name: '필터 초기화', exact: true }).click();
	await expect(page.getByRole('button', { name: /메일 캐시 동작 확인/ }).first()).toBeVisible();
	const search = page.getByRole('textbox', { name: '메일 검색', exact: true });
	await search.fill('no-match-fixture');
	await search.press('Enter');
	await expect(page.locator('[data-slot="empty-title"]')).toHaveText('검색 조건에 맞는 메일이 없습니다');
	await capture(page, 'mail', 'search-empty');
	await page.getByRole('button', { name: '필터 초기화', exact: true }).click();
	await expect(search).toHaveValue('');
	await expect(page.getByRole('button', { name: /메일 캐시 동작 확인/ }).first()).toBeVisible();
});

test('CRM filtered zero keeps the table and can reset to loaded accounts', async ({ page }) => {
	test.skip(phase === 'before', 'Behavioral checks for the correction');
	const fixture = new EmptyWorkspaceFixture();
	fixture.empty = false;
	await prepareWorkspaceLoading(page, fixture);
	await page.goto('/example-co/crm');
	const accounts = page.getByRole('tabpanel', { name: '관계처', exact: true });
	await expect(accounts.getByText('예시 파트너 1', { exact: true })).toBeVisible();
	await page.getByRole('combobox', { name: '상태', exact: true }).click();
	await page.getByRole('option', { name: '비활성', exact: true }).click();
	await expect(accounts.locator('[data-slot="empty-title"]')).toHaveText('조건에 맞는 관계처가 없습니다.');
	await capture(page, 'crm-accounts', 'filtered-empty');
	await accounts.getByRole('button', { name: '초기화', exact: true }).click();
	await expect(accounts.getByText('예시 파트너 1', { exact: true })).toBeVisible();
	await expect(accounts.locator('[data-slot="empty"]')).toHaveCount(0);
});

test('memory search zero can restore all loaded memories', async ({ page }) => {
	test.skip(phase === 'before', 'Behavioral checks for the correction');
	const fixture = new EmptyWorkspaceFixture();
	fixture.empty = false;
	await prepareWorkspaceLoading(page, fixture);
	await page.goto('/example-co/memory');
	await expect(page.locator('#memory-layers-title')).toBeVisible();
	await page.locator('#memory-search').fill('no-match-fixture');
	await expect(page.locator('[data-slot="empty-title"]')).toContainText('검색');
	await capture(page, 'memory', 'search-empty');
	await page.getByRole('button', { name: '모든 기억 보기', exact: true }).click();
	await expect(page.locator('#memory-search')).toHaveValue('');
	await expect(page.locator('main button[aria-pressed]').filter({ has: page.locator('p') }).first()).toBeVisible();
});

for (const status of [503, 403]) for (const scene of ['files', 'memory', 'runs']) {
	test(`${scene} ${status} is an error and never a loaded empty`, async ({ page }) => {
		const fixture = new EmptyWorkspaceFixture();
		const capability = { files: 'person.files.list', memory: 'person.memory.facts', runs: 'person.runs.list' }[scene]!;
		fixture.statuses.set(capability, status);
		await prepareWorkspaceLoading(page, fixture);
		await page.goto(`/example-co/${scene}`);
		if (scene === 'runs') await expect(page.getByText('작업 목록을 불러오지 못했습니다.', { exact: true })).toBeVisible();
		else await expect(page.getByRole('alert').first()).toBeVisible();
		await expect(page.locator('[data-slot="empty"]:visible')).toHaveCount(0);
		await capture(page, scene, status === 403 ? 'denied' : 'error');
	});
}

test('files keep loaded records through a failed refresh instead of switching to empty', async ({ page }) => {
	const fixture = new EmptyWorkspaceFixture();
	fixture.empty = false;
	await prepareWorkspaceLoading(page, fixture);
	await page.goto('/example-co/files');
	const file = page.getByRole('button', { name: /주간-회고-1.md/ });
	await expect(file).toBeVisible();
	fixture.hold('person.files.list');
	await page.getByRole('tabpanel', { name: '워크스페이스' }).getByRole('button', { name: '새로고침', exact: true }).click();
	await expect.poll(() => fixture.requested.has('person.files.list')).toBe(true);
	await expect(file).toBeVisible();
	await expect(page.locator('[data-slot="empty"]:visible')).toHaveCount(0);
	fixture.statuses.set('person.files.list', 503);
	fixture.release('person.files.list');
	await expect(page.getByRole('alert')).toBeVisible();
	await expect(file).toBeVisible();
	await expect(page.locator('[data-slot="empty"]:visible')).toHaveCount(0);
	await capture(page, 'files', 'retained-error');
});

for (const status of [503, 403]) for (const kind of ['skills', 'learned-skills']) {
	test(`admin ${kind} retains transient failures and clears denied data on ${status}`, async ({ page }) => {
		test.skip(phase === 'before', 'Behavioral checks for denial-safe retained data');
		const fixture = new EmptyWorkspaceFixture();
		fixture.empty = false;
		await prepareAdmin(page, fixture);
		await page.goto('/example-co/settings');
		await page.getByRole('tab', { name: '관리자', exact: true }).click();
		const title = kind === 'skills' ? '스킬' : '배운 절차';
		const name = kind === 'skills' ? 'fixture-loaded-skill' : 'fixture-learned-skill';
		const card = page.locator('[data-slot="card"]').filter({ has: page.locator('[data-slot="card-title"]').filter({ hasText: new RegExp(`^${title}$`) }) });
		await expect(card.getByText(name, { exact: true })).toBeVisible();
		await card.scrollIntoViewIfNeeded();
		fixture.statuses.set(kind === 'skills' ? 'person.skills.list' : 'person.agent_learning.skills.list', status);
		await card.getByRole('button', { name: '새로고침', exact: true }).click();
		await expect(card.getByRole('alert')).toBeVisible();
		if (status === 403) await expect(card.getByText(name, { exact: true })).toHaveCount(0);
		else await expect(card.getByText(name, { exact: true })).toBeVisible();
		await expect(card.locator('[data-slot="empty"]')).toHaveCount(0);
		await capture(page, kind, status === 403 ? 'denied' : 'retained-error');
	});
}

for (const width of [1280, 390, 320]) {
	test(`task list preserves pending then shows a compact settled zero at ${width}px`, async ({ page }) => {
		await page.setViewportSize({ width, height: 900 });
		const fixture = new EmptyWorkspaceFixture();
		fixture.empty = false;
		fixture.hold('task_list');
		await prepareWorkspaceLoading(page, fixture);
		await page.goto('/example-co/task');
		await expect(page.locator('[data-task-ready="true"]')).toBeVisible();
		await page.getByRole('tab', { name: '표', exact: true }).click();
		await expect.poll(() => fixture.requested.has('task_list')).toBe(true);
		await expect(page.locator('[data-task-content-skeleton="list"]')).toBeVisible();
		await expect(page.locator('[data-task-active-tab] [data-slot="empty"]')).toHaveCount(0);
		await capture(page, 'task-list', 'loading');
		fixture.release('task_list');
		await expect(page.getByText(phase === 'before' ? '조건에 맞는 업무가 없습니다.' : '아직 업무가 없습니다.', { exact: true })).toBeVisible();
		await expect(page.locator('[data-task-content-skeleton="list"]')).toHaveCount(0);
		if (phase !== 'before') {
			await expect(page.locator('[data-slot="empty-title"]').filter({ hasText: '아직 업무가 없습니다.' })).toBeVisible();
			expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
		}
		await capture(page, 'task-list', 'empty');
	});
}

for (const deniedBy of ['newer-refresh', 'history']) {
	test(`learned skills cannot restore an older successful refresh after ${deniedBy} denial`, async ({ page }) => {
		test.skip(phase === 'before', 'Regression for denial-safe retained data');
		const fixture = new EmptyWorkspaceFixture();
		fixture.empty = false;
		const capability = 'person.agent_learning.skills.list';
		let reads = 0;
		let actions = 0;
		let releaseOlder = () => {};
		const olderResponse = new Promise<void>(resolve => { releaseOlder = resolve; });
		const wait = fixture.waitFor.bind(fixture);
		fixture.waitFor = async name => {
			if (name === 'person.agent_learning.skills.action') actions += 1;
			if (name === capability && ++reads === 2) { await olderResponse; return; }
			await wait(name);
		};
		await prepareAdmin(page, fixture);
		await page.goto('/example-co/settings');
		await page.getByRole('tab', { name: '관리자', exact: true }).click();
		const card = page.locator('[data-slot="card"]').filter({ has: page.getByText('배운 절차', { exact: true }) });
		const skill = card.getByText('fixture-learned-skill', { exact: true });
		await expect(skill).toBeVisible();
		await card.getByRole('button', { name: '새로고침', exact: true }).click();
		await expect.poll(() => reads).toBe(2);
		if (deniedBy === 'history') {
			fixture.statuses.set('person.agent_learning.skills.get', 403);
			await card.getByRole('button', { name: '변경 이력', exact: true }).click();
		} else {
			fixture.statuses.set(capability, 403);
			await card.getByRole('button', { name: '보호', exact: true }).click();
			await expect.poll(() => reads).toBe(3);
		}
		await expect(card.getByRole('alert')).toBeVisible();
		await expect(skill).toHaveCount(0);
		await expect(card.getByRole('button', { name: '새로고침', exact: true })).toBeEnabled();
		fixture.statuses.clear();
		releaseOlder();
		await page.evaluate(() => new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
		await expect(skill).toHaveCount(0);
		await expect(card.getByRole('alert')).toBeVisible();
		await card.getByRole('button', { name: '새로고침', exact: true }).click();
		await expect(skill).toBeVisible();
		const previousActions = actions;
		await card.getByRole('button', { name: '보호', exact: true }).click();
		await expect.poll(() => actions).toBe(previousActions + 1);
	});
}

test('learned skills denial ends loading while a sibling hangs and allows a new retry', async ({ page }) => {
	test.skip(phase === 'before', 'Regression for terminal denial with a pending sibling');
	const fixture = new EmptyWorkspaceFixture();
	fixture.empty = false;
	let settingsReads = 0;
	let releaseSettings = () => {};
	const oldSettings = new Promise<void>(resolve => { releaseSettings = resolve; });
	const wait = fixture.waitFor.bind(fixture);
	fixture.waitFor = async name => {
		if (name === 'person.agent_learning.settings.get' && ++settingsReads === 2) { await oldSettings; return; }
		await wait(name);
	};
	await prepareAdmin(page, fixture);
	await page.goto('/example-co/settings');
	await page.getByRole('tab', { name: '관리자', exact: true }).click();
	const card = page.locator('[data-slot="card"]').filter({ has: page.getByText('배운 절차', { exact: true }) });
	const skill = card.getByText('fixture-learned-skill', { exact: true });
	await expect(skill).toBeVisible();
	fixture.statuses.set('person.agent_learning.skills.list', 403);
	await card.getByRole('button', { name: '새로고침', exact: true }).click();
	await expect.poll(() => settingsReads).toBe(2);
	await expect(card.getByRole('alert')).toBeVisible();
	await expect(skill).toHaveCount(0);
	await expect(card.getByRole('button', { name: '새로고침', exact: true })).toBeEnabled();
	fixture.statuses.clear();
	await card.getByRole('button', { name: '새로고침', exact: true }).click();
	await expect(skill).toBeVisible();
	releaseSettings();
	await page.evaluate(() => new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
	await expect(skill).toBeVisible();
	await expect(card.getByRole('alert')).toHaveCount(0);
});
