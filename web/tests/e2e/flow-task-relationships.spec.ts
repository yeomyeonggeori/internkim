import { expect, test } from '@playwright/test';
import { flowDashboardTaskID, taskCard } from './flow-task-helpers';

test.describe('flow task relationships', () => {
	test.beforeEach(async ({ request }) => {
		const response = await request.post('/flow/api/test/reset');
		expect(response.ok()).toBe(true);
	});

	test('shows the parent and direct children in the task detail', async ({ page }) => {
		await page.goto('/flow/');
		await expect(page.getByRole('tab', { name: '보드', exact: true })).toHaveAttribute('aria-selected', 'true');
		await taskCard(page, flowDashboardTaskID).click();

		const relationships = page.locator('[data-flow-task-relationships]');
		await expect(relationships.getByRole('heading', { name: '업무 관계' })).toBeVisible();
		await expect(relationships.getByText('연결된 부모 업무가 없습니다.')).toBeVisible();
		await expect(relationships.getByRole('heading', { name: '자녀 업무', exact: true })).toBeVisible();
		await expect(relationships.getByText('2 / 6')).toBeVisible();
		await expect(relationships.locator('[data-flow-relationship-task]')).toHaveCount(8);
		await expect(relationships.locator('[data-flow-relationship-list]')).toHaveClass(/divide-y/);
		const firstChild = relationships.locator('[data-flow-relationship-task]').first();
		await expect(firstChild.locator('[data-slot="avatar"]')).toBeVisible();
	});

	test('connects multiple existing child tasks from the edit sheet', async ({ page }) => {
		await page.goto('/flow/');
		await expect(page.getByRole('tab', { name: '보드', exact: true })).toHaveAttribute('aria-selected', 'true');
		await taskCard(page, flowDashboardTaskID).click();
		await page.getByRole('button', { name: '업무 수정', exact: true }).click();

		const relationships = page.locator('[data-flow-task-relationships]');
		await relationships.getByRole('button', { name: '자녀 업무 추가', exact: true }).click();
		await page.getByPlaceholder('자녀 업무 검색').fill('릴리즈 노트 초안 작성');
		await page.getByRole('dialog', { name: '자녀 업무' })
			.getByRole('option', { name: /릴리즈 노트 초안 작성/ })
			.first()
			.click();
		await page.getByRole('button', { name: '선택한 업무 연결', exact: true }).click();

		await expect(relationships.getByText('릴리즈 노트 초안 작성', { exact: true })).toBeVisible();
	});

	test('confirms before replacing an unsaved edit with a new child draft', async ({ page }) => {
		await page.goto('/flow/');
		await taskCard(page, flowDashboardTaskID).click();
		await page.getByRole('button', { name: '업무 수정', exact: true }).click();

		const relationships = page.locator('[data-flow-task-relationships]');
		await relationships.getByRole('button', { name: '자녀 업무 추가', exact: true }).click();
		page.once('dialog', (dialog) => dialog.dismiss());
		await page.getByRole('option', { name: '새 자녀 업무 만들기', exact: true }).click();
		await expect(page.getByRole('dialog', { name: '자녀 업무' })).toBeVisible();

		page.once('dialog', (dialog) => dialog.accept());
		await page.getByRole('option', { name: '새 자녀 업무 만들기', exact: true }).click();
		await expect(page.getByRole('heading', { name: '업무 추가', exact: true })).toBeVisible();
	});
});
