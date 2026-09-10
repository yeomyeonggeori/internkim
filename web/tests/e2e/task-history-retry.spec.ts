import { expect, type Page, type Route, test } from '@playwright/test';

const failedTaskRunID = 'failed-task-run-001';
const completedTaskRunID = 'completed-task-run-002';
const childTaskRunID = 'retry-child-run-003';

test.describe('task history retry', () => {
	test.beforeEach(async ({ page }) => {
		await page.route('**/agent/api/**', async (route) => {
			await route.fulfill({ json: {} });
		});
		await page.route('**/agent/api/buzz-vault', async (route) => {
			await route.fulfill({ json: { found: true, wrapped: { copies: [] } } });
		});
		await page.route('**/auth/session**', async (route) => {
			await route.fulfill({ json: { authenticated: true, email: 'tester@example.com', canViewTasks: true } });
		});
		await page.route('**/admin/api/session', async (route) => {
			await route.fulfill({ json: { email: 'tester@example.com', isAdmin: true } });
		});
		await page.route('**/admin/api/locale', async (route) => {
			await route.fulfill({ json: { locale: 'en' } });
		});
	});

	test('retries one failed run and follows the child to completion', async ({ page }) => {
		const retryRequests: string[] = [];
		let childStatus: 'planned' | 'completed' = 'planned';
		await installTaskHistoryFixtures(page, {
			onRetry: async (route) => {
				retryRequests.push(route.request().url());
				expect(route.request().postDataJSON()).toEqual({ taskRunID: failedTaskRunID });
				await route.fulfill({ status: 202, json: { taskRunID: childTaskRunID, status: 'planned' } });
			},
			getDetail: (taskRunID) => {
				const detail = detailForTaskRun(taskRunID, childStatus);
				if (taskRunID === childTaskRunID) childStatus = 'completed';
				return detail;
			}
		});

		await page.setViewportSize({ width: 1440, height: 1000 });
		await page.goto(`/runs/${failedTaskRunID}`);
		await expect(page.getByText('Failed', { exact: true })).toBeVisible();
		await page.screenshot({ path: '../.artifacts/task-history-retry-ui/desktop.png' });
		await page.getByRole('button', { name: /^retry$/i }).click();

		await expect.poll(() => retryRequests.length).toBe(1);
		await expect(page).toHaveURL(new RegExp(`${childTaskRunID}$`));
		await expect(page.getByText('Completed', { exact: true })).toBeVisible();
		await expect(page.getByRole('button', { name: /^retry$/i })).toHaveCount(0);
		expect(retryRequests).toHaveLength(1);
	});

	test('does not offer retry for a completed run on mobile', async ({ page }) => {
		await installTaskHistoryFixtures(page, {
			getDetail: () => detailForTaskRun(completedTaskRunID, 'completed')
		});
		await page.setViewportSize({ width: 444, height: 866 });
		await page.goto(`/runs/${completedTaskRunID}`);

		await expect(page.getByText('Completed', { exact: true })).toBeVisible();
		await expect(page.getByRole('button', { name: /^retry$/i })).toHaveCount(0);
		await page.screenshot({ path: '../.artifacts/task-history-retry-ui/mobile.png' });
	});

	test('retries from the mobile task list without opening the source row', async ({ page }) => {
		await installTaskHistoryFixtures(page, {
			onRetry: async (route) => {
				expect(route.request().postDataJSON()).toEqual({ taskRunID: failedTaskRunID });
				await route.fulfill({ status: 202, json: { taskRunID: childTaskRunID, status: 'planned' } });
			},
			getDetail: (taskRunID) => detailForTaskRun(taskRunID, 'completed')
		});
		await page.setViewportSize({ width: 375, height: 812 });
		await page.goto('/runs');
		const retry = page.getByRole('button', { name: /^retry$/i });
		await expect(retry).toHaveCount(1);
		await page.screenshot({ path: '../.artifacts/task-history-retry-ui/mobile-list.png' });
		await retry.click();
		await expect(page).toHaveURL(new RegExp(`${childTaskRunID}$`));
		await expect(page.getByText('Completed', { exact: true })).toBeVisible();
	});
});

type FixtureOptions = {
	onRetry?: (route: Route) => Promise<void>;
	getDetail: (taskRunID: string) => TaskDetailFixture;
};

type TaskDetailFixture = {
	taskRun: {
		taskRunID: string;
		status: string;
		prompt: string;
		createdAt: string;
		updatedAt: string;
		failureReason?: string;
	};
	taskEvents: Array<{ name: string; body: string; createdAt: string }>;
};

async function installTaskHistoryFixtures(page: Page, options: FixtureOptions): Promise<void> {
	await page.route('**/runs/api?*', async (route) => {
		await route.fulfill({
			json: {
				taskRuns: [
					{ taskRunID: failedTaskRunID, status: 'failed', prompt: 'Rebuild the report', createdAt: '2026-09-10T00:00:00Z', updatedAt: '2026-09-10T00:00:00Z' },
					{ taskRunID: completedTaskRunID, status: 'completed', prompt: 'Completed report', createdAt: '2026-09-09T00:00:00Z', updatedAt: '2026-09-09T00:00:00Z' }
				],
				totalCount: 2,
				dailyCostSummaries: [],
				dailyCostScope: { taskRunLimit: 500, taskRunCount: 2, totalTaskRunCount: 2, isTruncated: false }
			}
		});
	});
	await page.route('**/runs/api/retry', async (route) => {
		if (route.request().method() !== 'POST' || !options.onRetry) return route.fallback();
		await options.onRetry(route);
	});
	await page.route('**/runs/api/detail?*', async (route) => {
		const taskRunID = new URL(route.request().url()).searchParams.get('taskRunID') ?? '';
		await route.fulfill({ json: options.getDetail(taskRunID) });
	});
}

function detailForTaskRun(taskRunID: string, childStatus: 'planned' | 'completed'): TaskDetailFixture {
	const isChild = taskRunID === childTaskRunID;
	const status = isChild ? childStatus : taskRunID === failedTaskRunID ? 'failed' : 'completed';
	return {
		taskRun: {
			taskRunID,
			status,
			createdAt: '2026-09-10T00:00:00Z',
			updatedAt: '2026-09-10T00:01:00Z',
			prompt: isChild ? 'Rebuild the report' : taskRunID === failedTaskRunID ? 'Rebuild the report' : 'Completed report',
			...(status === 'failed' ? { failureReason: 'The report export failed' } : {})
		},
		taskEvents: [{ name: 'agent.task_launched', body: '{}', createdAt: '2026-09-10T00:00:00Z' }]
	};
}
