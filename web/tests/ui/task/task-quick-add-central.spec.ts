import { expect, test } from '@playwright/test';
import { member1ID } from '../support/central-test-utils';
import {
	openTaskCard,
	removeTasks,
	seedTasks,
	signInToTheTaskBoard,
	taskCard,
	taskRowTitled,
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

	const panel = page.getByRole('dialog', { name: '업무 추가' });
	await expect(panel).toBeVisible();
	await expect(launcher).toHaveAttribute('aria-expanded', 'true');
	await expect(panel.getByPlaceholder('예: 금요일까지 견적서 보내기')).toBeFocused();

	await page.keyboard.press('Escape');
	await expect(panel).not.toBeVisible();
	await expect(launcher).toHaveAttribute('aria-expanded', 'false');
	await expect(launcher).toBeFocused();
});

test('the submit button waits for something to ask for', async ({ page }) => {
	await signInToTheTaskBoard(page);
	await page.locator('[data-task-quick-add-launcher]').click();

	const panel = page.getByRole('dialog', { name: '업무 추가' });
	const submit = panel.getByRole('button', { name: '추가', exact: true });
	await expect(submit).toBeDisabled();

	await panel.getByPlaceholder('예: 금요일까지 견적서 보내기').fill('E2E 요청 문장');
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

	const panel = page.getByRole('dialog', { name: '업무 추가' });
	await expect(panel).toBeVisible();
	const bounds = await panel.boundingBox();
	expect(bounds).not.toBeNull();
	if (!bounds) return;
	expect(bounds.x).toBeGreaterThanOrEqual(0);
	expect(bounds.x + bounds.width).toBeLessThanOrEqual(430);
});

test('a note adds every task drafted from it through task_add, and says which one it could not add', async ({ page }) => {
	const addedTitle = 'E2E 빠른 추가로 적은 업무';
	const refusedTitle = 'E2E 빠른 추가로 맡긴 업무';
	await page.route('**/api/v1/tools/task_draft_get/invoke', (route) =>
		route.fulfill({
			json: {
				toolName: 'task_draft_get',
				outcome: 'succeeded',
				result: { tasks: [{ title: addedTitle }, { title: refusedTitle, participantPersonHints: ['아무도아닌이름'] }] }
			}
		})
	);
	await signInToTheTaskBoard(page);
	await page.locator('[data-task-quick-add-launcher]').click();
	const panel = page.getByRole('dialog', { name: '업무 추가' });
	const note = panel.getByPlaceholder('예: 금요일까지 견적서 보내기');

	await note.fill('적은 업무 하나, 아무도아닌이름에게 맡긴 업무 하나');
	await panel.getByRole('button', { name: '추가', exact: true }).click();

	const outcomes = panel.locator('[data-task-quick-add-outcomes]');
	await expect(outcomes.getByRole('listitem').filter({ hasText: addedTitle }).getByLabel('추가함')).toBeVisible();
	await expect(outcomes.getByRole('listitem').filter({ hasText: refusedTitle }).getByLabel('추가하지 못함')).toBeVisible();
	await expect(note).toHaveValue('적은 업무 하나, 아무도아닌이름에게 맡긴 업무 하나');

	const added = await taskRowTitled(addedTitle);
	expect(added).not.toBeNull();
	if (!added) return;
	seededTaskIDs.push(added.id);
	expect(await taskRowTitled(refusedTitle)).toBeNull();
	await page.keyboard.press('Escape');
	await expect(taskCard(page, added.id)).toBeVisible();
});
