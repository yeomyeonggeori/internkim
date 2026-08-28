import { expect, test } from '@playwright/test';
import { isUnknownRecord, taskCard } from './task-helpers';

test.describe('flow task filters', () => {
	test.beforeEach(async ({ request }) => {
		const response = await request.post('/task/api/test/reset');
		expect(response.ok()).toBe(true);
	});

	test('shows tasks from every participant when the participant filter is set to all', async ({ page }) => {
		await page.route('**/task/api/state', async (route) => {
			const response = await route.fetch();
			const state: unknown = await response.json();
			if (!isUnknownRecord(state)) throw new Error('Flow state response must be an object');
			if (!Array.isArray(state.tasks)) throw new Error('Flow state response must include tasks');
			if (!isUnknownRecord(state.currentWeek) || typeof state.currentWeek.code !== 'string') {
				throw new Error('Flow state response must include the current week');
			}
			const currentWeekCode = state.currentWeek.code;
			const referenceTask = state.tasks.find((task) => isUnknownRecord(task) && task.weekCode === currentWeekCode);
			if (!referenceTask) throw new Error('Flow state must include a task');
			await route.fulfill({
				response,
				json: {
					...state,
					tasks: [
						...state.tasks,
						{
							...referenceTask,
							id: 'stale-participant-task',
							ownerID: 'former-member',
							ownerName: '이전 구성원',
							participantIDs: ['former-member'],
							participantNames: ['이전 구성원'],
							content: '현재 구성원 목록에 없는 참여자의 업무'
						}
					]
				}
			});
		});
		await page.goto('/flow/');

		await page.getByRole('button', { name: /필터/ }).click();
		const filterPanel = page.locator('[data-task-filter-panel]');
		const participantCombobox = filterPanel.getByRole('combobox', { name: '참여자', exact: true });
		await expect(participantCombobox).toBeVisible();
		await expect(page.getByRole('button', { name: '필터 1', exact: true })).toBeVisible();
		await expect(taskCard(page, 'stale-participant-task')).toHaveCount(0);

		await participantCombobox.click();
		const participantOptions = page.getByRole('option');
		await expect(participantOptions.first()).toBeVisible();
		expect(await participantOptions.count()).toBeGreaterThan(1);
		await page.getByRole('option', { name: '전체', exact: true }).click();

		await expect(page.getByRole('button', { name: '필터', exact: true })).toBeVisible();
		await expect(taskCard(page, 'stale-participant-task')).toBeVisible();

		await participantCombobox.click();
		await page.getByRole('option').nth(1).click();
		await expect(page.getByRole('button', { name: '필터 1', exact: true })).toBeVisible();
		await expect(taskCard(page, 'stale-participant-task')).toHaveCount(0);

		await participantCombobox.click();
		await page.getByRole('option', { name: '전체', exact: true }).click();
		await expect(page.getByRole('button', { name: '필터', exact: true })).toBeVisible();
		await expect(taskCard(page, 'stale-participant-task')).toBeVisible();
	});
});
