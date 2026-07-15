import { expect, test } from '@playwright/test';
import { isUnknownRecord, taskCard } from './flow-task-helpers';

test.describe('flow task filters', () => {
	test.beforeEach(async ({ request }) => {
		const response = await request.post('/flow/api/test/reset');
		expect(response.ok()).toBe(true);
	});

	test('selects every participant when choosing all participants', async ({ page }) => {
		await page.route('**/flow/api/state', async (route) => {
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
		const filterPanel = page.locator('[data-flow-filter-panel]');
		const participantCheckboxes = filterPanel.getByRole('checkbox');
		await expect(participantCheckboxes.first()).toBeVisible();
		expect(await participantCheckboxes.count()).toBeGreaterThan(1);
		await expect(page.getByRole('button', { name: '필터 1', exact: true })).toBeVisible();
		await expect(filterPanel.locator('input[type="checkbox"]:checked')).toHaveCount(1);
		await expect(taskCard(page, 'stale-participant-task')).toHaveCount(0);

		await filterPanel.getByRole('button', { name: '전체 참여자', exact: true }).click();

		for (const participantCheckbox of await participantCheckboxes.all()) {
			await expect(participantCheckbox).toBeChecked();
		}
		await expect(page.getByRole('button', { name: '필터', exact: true })).toBeVisible();
		await expect(taskCard(page, 'stale-participant-task')).toBeVisible();

		await participantCheckboxes.first().uncheck();
		await expect(participantCheckboxes.first()).not.toBeChecked();
		await expect(page.getByRole('button', { name: '필터 1', exact: true })).toBeVisible();
		await expect(taskCard(page, 'stale-participant-task')).toHaveCount(0);

		await participantCheckboxes.first().check();

		for (const participantCheckbox of await participantCheckboxes.all()) {
			await expect(participantCheckbox).toBeChecked();
		}
		await expect(page.getByRole('button', { name: '필터', exact: true })).toBeVisible();
		await expect(taskCard(page, 'stale-participant-task')).toBeVisible();
	});
});
