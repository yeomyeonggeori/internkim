import { expect, test } from '@playwright/test';
import { member1ID } from './central-test-utils';
import {
	openTaskCard,
	removeTasks,
	seedTasks,
	signInToTheTaskBoard,
	taskSheet
} from './task-central-test-utils';

test.describe.configure({ mode: 'serial', timeout: 90_000 });
test.use({ locale: 'ko-KR' });

let seededTaskIDs: string[] = [];
let openableTaskID = '';

test.beforeAll(async () => {
	seededTaskIDs = await seedTasks([
		{
			title: 'E2E 빠른 추가 옆의 업무',
			status: 'planned',
			participantIDs: [member1ID],
			business: '사업하나',
			type: '기능',
			size: 'M'
		}
	]);
	[openableTaskID] = seededTaskIDs;
});

test.afterAll(async () => {
	await removeTasks(seededTaskIDs);
	seededTaskIDs = [];
});

test('the launcher opens the quick add panel and Escape gives the launcher its focus back', async ({ page }) => {
	await signInToTheTaskBoard(page);

	const launcher = page.locator('[data-task-quick-add-launcher]');
	await expect(launcher).toHaveAttribute('aria-expanded', 'false');
	await launcher.click();

	const panel = page.getByRole('dialog', { name: 'AI로 업무 추가' });
	await expect(panel).toBeVisible();
	await expect(launcher).toHaveAttribute('aria-expanded', 'true');
	await expect(panel.getByPlaceholder('예: 10분 회의')).toBeFocused();

	await page.keyboard.press('Escape');
	await expect(panel).not.toBeVisible();
	await expect(launcher).toHaveAttribute('aria-expanded', 'false');
	await expect(launcher).toBeFocused();
});

test('the submit button waits for something to ask for', async ({ page }) => {
	await signInToTheTaskBoard(page);
	await page.locator('[data-task-quick-add-launcher]').click();

	const panel = page.getByRole('dialog', { name: 'AI로 업무 추가' });
	const submit = panel.getByRole('button', { name: 'AI로 업무 추가' });
	await expect(submit).toBeDisabled();

	await panel.getByPlaceholder('예: 10분 회의').fill('E2E 요청 문장');
	await expect(submit).toBeEnabled();
});

test('the launcher steps aside while a task sheet is open', async ({ page }) => {
	await signInToTheTaskBoard(page);
	await expect(page.locator('[data-task-quick-add-launcher]')).toBeVisible();

	await openTaskCard(page, openableTaskID);
	await expect(page.locator('[data-task-quick-add-launcher]')).toHaveCount(0);

	await page.keyboard.press('Escape');
	await expect(taskSheet(page)).not.toBeVisible();
	await expect(page.locator('[data-task-quick-add-launcher]')).toBeVisible();
});

test('the quick add panel stays inside a narrow viewport', async ({ page }) => {
	await page.setViewportSize({ width: 430, height: 932 });
	await signInToTheTaskBoard(page);
	await page.locator('[data-task-quick-add-launcher]').click();

	const panel = page.getByRole('dialog', { name: 'AI로 업무 추가' });
	await expect(panel).toBeVisible();
	const bounds = await panel.boundingBox();
	expect(bounds).not.toBeNull();
	if (!bounds) return;
	expect(bounds.x).toBeGreaterThanOrEqual(0);
	expect(bounds.x + bounds.width).toBeLessThanOrEqual(430);
});
