import { expect, test } from '@playwright/test';
import type { TaskState } from '../../src/routes/task/task-types';
import { taskDashboardTaskID, marketScanTaskID, taskCard } from './task-helpers';

const completedChildTaskID = '26W23-calendar-sync';

test.describe('flow task child progress', () => {
	test.beforeEach(async ({ request }) => {
		const response = await request.post('/task/api/test/reset');
		expect(response.ok()).toBe(true);
	});

	test('shows progress from direct children hidden by the participant filter', async ({ page }) => {
		await page.route('**/task/api/state', async (route) => {
			const response = await route.fetch();
			const state = (await response.json()) as TaskState;
			await route.fulfill({
				response,
				json: {
					...state,
					tasks: state.tasks.map((task) =>
						task.id === completedChildTaskID || task.id === marketScanTaskID
							? {
									...task,
									parentTaskID: taskDashboardTaskID,
									ownerID: 'hidden-member',
									ownerName: '숨은 담당자',
									participantIDs: ['hidden-member'],
									participantNames: ['숨은 담당자']
								}
							: task
					)
				}
			});
		});

		await page.goto('/flow/');
		const parentCard = taskCard(page, taskDashboardTaskID);
		const progress = parentCard.locator('[data-task-child-progress]');

		await expect(parentCard).toBeVisible();
		await expect(taskCard(page, completedChildTaskID)).toHaveCount(0);
		await expect(taskCard(page, marketScanTaskID)).toHaveCount(0);
		await expect(progress).toContainText('2/7');
		await expect(progress).not.toContainText('자녀 업무');
		await expect(progress).not.toContainText('completed');
		await expect(progress).toContainText('29%');
		await expect(progress.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '2');
		await expect(progress.getByRole('progressbar')).toHaveAttribute('aria-valuemax', '7');
		await expect(progress.locator('[data-task-child-progress-segment]')).toHaveCount(7);
	});
});
