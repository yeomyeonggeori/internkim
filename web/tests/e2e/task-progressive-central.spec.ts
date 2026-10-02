import { chromium, expect, test } from '@playwright/test';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { member1ID } from './central-test-utils';
import { seedTasks, removeTasks, taskCard, weekStartInstant } from './task-central-test-utils';
import { signInToTheCentralPlane } from './central-plane-sign-in';

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
	const path = await mkdtemp(join(tmpdir(), 'internkim-cache-e2e-'));
	let context = await chromium.launchPersistentContext(path, { baseURL: process.env.PLAYWRIGHT_BASE_URL, locale: 'ko-KR', timezoneId: 'Asia/Seoul' });
	let release!: () => void;
	const gate = new Promise<void>(resolve => { release = resolve; });
	const key = 'internkim:task-snapshot:v1';
	try {
		let page = context.pages()[0];
		await signInToTheCentralPlane(page, '/example-co/task');
		await expect.poll(() => page.evaluate(key => localStorage.getItem(key), key)).not.toBeNull();
		const before = await page.evaluate(key => JSON.parse(localStorage.getItem(key)!).savedAt, key);
		await context.close();
		context = await chromium.launchPersistentContext(path, { baseURL: process.env.PLAYWRIGHT_BASE_URL, locale: 'ko-KR', timezoneId: 'Asia/Seoul' });
		page = context.pages()[0];
		await page.route('**/task_list/invoke', async route => { await gate; await route.fulfill({ status: 503, json: { error: 'Temporarily unavailable' } }); });
		await page.goto('/example-co/task');
		await expect(page.locator('[data-task-progress="snapshot"]')).toBeVisible();
		await expect(page.locator('[data-task-cache-status]')).toBeVisible();
		await page.getByRole('button', { name: '이전 주', exact: true }).click();
		expect(await page.evaluate(key => JSON.parse(localStorage.getItem(key)!).savedAt, key)).toBe(before);
		release();
		await expect(page.locator('main')).toContainText('Temporarily unavailable');
		await expect(page.locator('[data-task-cache-status]')).toBeVisible();
		await expect(page.locator('[data-task-progress="snapshot"]')).toBeVisible();
	} finally { release(); await context.close(); await rm(path, { recursive: true, force: true }); }
});

test('first task visit shows structure then real cards before directory completion', async ({ page }) => {
	const [id] = await seedTasks([{ title: '단계별 실제 업무', status: 'planned', participantIDs: [member1ID], startsAtISO: weekStartInstant(), endsAtISO: weekStartInstant() }]);
	let releaseTasks!: () => void;
	let releasePeople!: () => void;
	const tasks = new Promise<void>(resolve => { releaseTasks = resolve; });
	const people = new Promise<void>(resolve => { releasePeople = resolve; });
	try {
		await signInToTheCentralPlane(page, '/example-co/organization');
		await expect(page.getByTestId('organization-board')).toBeVisible();
		await page.evaluate(() => localStorage.removeItem('internkim:task-snapshot:v1'));
		await page.route('**/task_list/invoke', async route => { await tasks; await route.continue(); });
		await page.route('**/person_list/invoke', async route => { await people; await route.continue(); });
		await page.goto('/example-co/task');
		await expect(page.locator('[data-task-progress="skeleton"]')).toBeVisible();
		await expect(page.locator('[data-task-skeleton]')).toBeVisible();
		await page.screenshot({ path: 'tests/performance/task-skeleton-desktop.png' });
		releaseTasks();
		await expect(taskCard(page, id)).toBeVisible();
		await expect(page.locator('[data-task-progress="tasks"]')).toBeVisible();
		await page.screenshot({ path: 'tests/performance/task-progressive-desktop.png' });
		expect(await page.evaluate(() => localStorage.getItem('internkim:task-snapshot:v1'))).toBeNull();
		releasePeople();
		await expect(page.locator('[data-task-ready="true"]')).toBeVisible();
		await expect.poll(() => page.evaluate(() => localStorage.getItem('internkim:task-snapshot:v1'))).not.toBeNull();
	} finally { releaseTasks(); releasePeople(); await removeTasks([id]); }
});

test('directory denial wins over a later task preview and cannot reappear on revisit', async ({ page }) => {
	let release!: () => void;
	const gate = new Promise<void>(resolve => { release = resolve; });
	try {
		await signInToTheCentralPlane(page, '/example-co/organization');
		await expect(page.getByTestId('organization-board')).toBeVisible();
		await page.route('**/task_list/invoke', async route => { await gate; await route.continue(); });
		await page.route('**/person_list/invoke', route => route.fulfill({ status: 403, json: { error: 'Permission denied' } }));
		await page.goto('/example-co/task');
		await expect(page.locator('[data-task-ready="false"]')).toBeVisible();
		await expect(page.locator('main')).toContainText('Permission denied');
		release();
		await page.waitForTimeout(200);
		await expect(page.locator('[data-task-board-card]')).toHaveCount(0);
		expect(await page.evaluate(() => localStorage.getItem('internkim:task-snapshot:v1'))).toBeNull();
		await page.goto('/example-co/organization');
		await page.goto('/example-co/task');
		await expect(page.locator('[data-task-board-card]')).toHaveCount(0);
	} finally { release(); }
});
