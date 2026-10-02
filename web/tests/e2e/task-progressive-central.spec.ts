import { chromium, expect, test } from '@playwright/test';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { centralPlaneAdminClient, exampleCompanyID, member1ID } from './central-test-utils';
import { seedTasks, removeTasks, taskCard, weekStartInstant, openTaskCard, taskSheet, taskParticipantIDsOf } from './task-central-test-utils';
import { signInToTheCentralPlane } from './central-plane-sign-in';
import { taskWeekOfDate } from '../../src/lib/task/task-week-code';

test.use({ locale: 'ko-KR' });

test('task startup creates report content only after the report tab is opened', async ({ page }) => {
	await signInToTheCentralPlane(page, '/example-co/task');
	await expect(page.locator('[data-task-ready="true"]')).toBeVisible();
	const report = page.getByText('이번 주간 업무 일별 종류 거리 분포', { exact: true });
	await expect(report).toHaveCount(0);
	await page.getByRole('tab', { name: '보고', exact: true }).click();
	await expect(report).toBeVisible();
	await page.getByRole('tab', { name: '업무', exact: true }).click();
	await expect(report).toHaveCount(1);
	await expect(report).toBeHidden();
	await page.getByRole('tab', { name: '보고', exact: true }).click();
	await expect(report).toBeVisible();
});

test('task avatars paint when their cards enter the visible scroll area', async ({ page }) => {
	const ids = await seedTasks(Array.from({ length: 32 }, (_, index) => ({ title: `가시성 검증 ${String(index).padStart(2, '0')}`, status: 'planned', participantIDs: [member1ID], startsAtISO: weekStartInstant(), endsAtISO: weekStartInstant() })));
	try {
		await signInToTheCentralPlane(page, '/example-co/task');
		await expect(page.locator('[data-task-ready="true"]')).toBeVisible();
		const cards = page.locator('[data-task-board-card]').filter({ hasText: '가시성 검증 ' });
		const first = cards.first();
		const last = cards.last();
		await first.scrollIntoViewIfNeeded();
		await expect(first.locator('canvas').first()).toHaveCSS('background-image', 'none');
		await expect(last).not.toBeInViewport();
		await expect(last.locator('canvas').first()).toHaveCSS('background-image', /linear-gradient/);
		await last.scrollIntoViewIfNeeded();
		await expect(last.locator('canvas').first()).toHaveCSS('background-image', 'none');
	} finally { await removeTasks(ids); }
});

test('a closed browser restores a scoped snapshot and preserves stale age on a week change', async () => {
	const [id] = await seedTasks([{ title: '재시작 후 유지하는 실제 업무', status: 'planned', participantIDs: [member1ID] }]);
	const path = await mkdtemp(join(tmpdir(), 'internkim-cache-e2e-'));
	let context = await chromium.launchPersistentContext(path, { baseURL: process.env.PLAYWRIGHT_BASE_URL, locale: 'ko-KR', timezoneId: 'Asia/Seoul' });
	let release!: () => void;
	const gate = new Promise<void>(resolve => { release = resolve; });
	const key = 'internkim:task-snapshot:v1';
	try {
		let page = context.pages()[0];
		await signInToTheCentralPlane(page, '/example-co/task');
		await page.getByRole('tab', { name: '보고', exact: true }).click();
		await expect.poll(() => page.evaluate(key => localStorage.getItem(key), key)).not.toBeNull();
		const before = await page.evaluate(key => JSON.parse(localStorage.getItem(key)!).savedAt, key);
		await context.close();
		context = await chromium.launchPersistentContext(path, { baseURL: process.env.PLAYWRIGHT_BASE_URL, locale: 'ko-KR', timezoneId: 'Asia/Seoul' });
		page = context.pages()[0];
		await page.route(/\/(task_list|task_board_get)\/invoke$/, async route => { await gate; await route.fulfill({ status: 503, json: { error: 'Temporarily unavailable' } }); });
		await page.goto('/example-co/task');
		await expect(page.locator('[data-task-progress="snapshot"]')).toBeVisible();
		await expect(taskCard(page, id)).toBeVisible();
		await expect(page.locator('[data-task-ready="false"]')).toBeVisible();
		await expect(page.locator('[data-task-cache-status]')).toBeVisible();
		await expect(page.getByRole('button', { name: '이전 주', exact: true })).toBeDisabled();
		const previousWeek = taskWeekOfDate(new Date(Date.now() - 7 * 86400000)).code;
		await page.goto(`/example-co/task?week=${previousWeek}`);
		await expect(page.locator('[data-task-progress="snapshot"]')).toBeVisible();
		expect(await page.evaluate(key => JSON.parse(localStorage.getItem(key)!).savedAt, key)).toBe(before);
		release();
		await expect(page.locator('main')).toContainText('Temporarily unavailable');
		await expect(page.locator('[data-task-cache-status]')).toBeVisible();
		await expect(page.locator('[data-task-progress="snapshot"]')).toBeVisible();
	} finally { release(); await context.close(); await rm(path, { recursive: true, force: true }); await removeTasks([id]); }
});

test('first task visit shows structure then real cards before directory completion', async ({ page }) => {
	const [id] = await seedTasks([{ title: '단계별 실제 업무', status: 'planned', participantIDs: [member1ID], startsAtISO: weekStartInstant(), endsAtISO: weekStartInstant() }]);
	const admin = centralPlaneAdminClient();
	const noted = await admin.from('task').update({ note: '보드에 포함하지 않는 상세 메모' }).eq('id', id);
	if (noted.error) throw new Error(noted.error.message);
	let releaseTasks!: () => void;
	let releasePeople!: () => void;
	const tasks = new Promise<void>(resolve => { releaseTasks = resolve; });
	const people = new Promise<void>(resolve => { releasePeople = resolve; });
	try {
		await signInToTheCentralPlane(page, '/example-co/organization');
		await expect(page.getByTestId('organization-board')).toBeVisible();
		await page.evaluate(() => { localStorage.removeItem('internkim:task-snapshot:v1'); localStorage.removeItem('internkim:task-board-snapshots:v1'); });
		await page.route('**/task_board_get/invoke', async route => { await tasks; await route.continue(); });
		await page.route('**/person_list/invoke', async route => { await people; await route.continue(); });
		await page.goto('/example-co/task');
		await expect(page.locator('[data-task-progress="skeleton"]')).toBeVisible();
		await expect(page.locator('[data-task-skeleton]')).toBeVisible();
		await page.screenshot({ path: 'tests/performance/task-skeleton-desktop.png' });
		releaseTasks();
		await expect(taskCard(page, id)).toBeVisible();
		await expect(page.locator('[data-task-ready="true"]')).toBeVisible();
		await expect(page.getByRole('status').filter({ hasText: '구성원 정보를 준비' })).toBeVisible();
		await page.screenshot({ path: 'tests/performance/task-progressive-desktop.png' });
		expect(await page.evaluate(() => localStorage.getItem('internkim:task-board-snapshots:v1'))).toBeNull();
		await openTaskCard(page, id);
		const sheet = taskSheet(page);
		await sheet.getByRole('button', { name: '업무 수정' }).click();
		await sheet.getByPlaceholder('업무 내용').fill('구성원 도착 전 수정한 업무');
		const saving = page.waitForResponse(response => response.url().endsWith('/task_update/invoke'));
		await sheet.getByRole('button', { name: '업무 저장' }).click();
		expect((await saving).ok()).toBe(true);
		releasePeople();
		await expect(sheet).not.toBeVisible();
		await expect(page.locator('[data-task-ready="true"]')).toBeVisible();
		await expect(taskCard(page, id)).toContainText('구성원 도착 전 수정한 업무');
		const saved = await admin.from('task').select('note').eq('id', id).single();
		expect(saved.error).toBeNull();
		expect(saved.data?.note).toBe('보드에 포함하지 않는 상세 메모');
		expect(await taskParticipantIDsOf(id)).toEqual([member1ID]);
		await expect.poll(() => page.evaluate(() => localStorage.getItem('internkim:task-snapshot:v1'))).not.toBeNull();
		expect(await page.evaluate(() => JSON.parse(localStorage.getItem('internkim:task-snapshot:v1')!).state.completeness)).toBe('full');
	} finally { releaseTasks(); releasePeople(); await removeTasks([id]); }
});

test('1501 historical rows keep the real board and its primary editor complete', async ({ page }) => {
	const [id] = await seedTasks([{ title: '대량 이력 중 표시하는 업무', status: 'planned', participantIDs: [member1ID] }]);
	const admin = centralPlaneAdminClient();
	const historicalIDs = Array.from({ length: 1501 }, () => crypto.randomUUID());
	const readBoard = () => page.waitForResponse(response => response.url().endsWith('/task_board_get/invoke') && response.ok());
	try {
		const initial = readBoard();
		await signInToTheCentralPlane(page, '/example-co/task');
		const before = (await (await initial).json()).result;
		await expect(taskCard(page, id)).toBeVisible();
		for (let offset = 0; offset < historicalIDs.length; offset += 500) {
			const inserted = await admin.from('task').insert(historicalIDs.slice(offset, offset + 500).map(id => ({
				id, company_id: exampleCompanyID, title: '보드 밖의 오래된 완료 업무', status: 'completed',
				starts_at: '2020-01-01T00:00:00Z', ends_at: '2020-01-02T00:00:00Z'
			})));
			if (inserted.error) throw new Error(inserted.error.message);
		}
		const next = readBoard();
		await page.reload();
		const after = (await (await next).json()).result;
		expect(after.tasks.map((task: { taskID: string }) => task.taskID).sort()).toEqual(before.tasks.map((task: { taskID: string }) => task.taskID).sort());
		await expect(page.locator('[data-task-ready="true"]')).toBeVisible();
		await openTaskCard(page, id);
		await expect(taskSheet(page).getByRole('button', { name: '업무 수정' })).toBeEnabled();
		await taskSheet(page).getByRole('button', { name: '업무 수정' }).click();
		await expect(taskSheet(page).getByPlaceholder('업무 내용')).toHaveValue('대량 이력 중 표시하는 업무');
	} finally {
		for (let offset = 0; offset < historicalIDs.length; offset += 100) await removeTasks(historicalIDs.slice(offset, offset + 100));
		await removeTasks([id]);
	}
});

test('directory denial wins over a later task preview and cannot reappear on revisit', async ({ page }) => {
	let release!: () => void;
	const gate = new Promise<void>(resolve => { release = resolve; });
	try {
		await signInToTheCentralPlane(page, '/example-co/organization');
		await expect(page.getByTestId('organization-board')).toBeVisible();
		await page.route('**/task_board_get/invoke', async route => { await gate; await route.continue(); });
		await page.route('**/person_list/invoke', route => route.fulfill({ status: 403, json: { error: 'Permission denied' } }));
		await page.goto('/example-co/task');
		await expect(page.locator('[data-task-ready="false"]')).toBeVisible();
		await expect(page.locator('main')).toContainText('Permission denied');
		release();
		await page.waitForTimeout(200);
		await expect(page.locator('[data-task-board-card]')).toHaveCount(0);
		expect(await page.evaluate(() => localStorage.getItem('internkim:task-snapshot:v1'))).toBeNull();
		expect(await page.evaluate(() => localStorage.getItem('internkim:task-board-snapshots:v1'))).toBeNull();
		await page.goto('/example-co/organization');
		await page.goto('/example-co/task');
		await expect(page.locator('[data-task-board-card]')).toHaveCount(0);
	} finally { release(); }
});
