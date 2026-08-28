import { expect, test } from '@playwright/test';
import { taskDashboardTaskID, taskCard } from './task-helpers';

test.describe('flow task relationships', () => {
	test.beforeEach(async ({ request }) => {
		const response = await request.post('/task/api/test/reset');
		expect(response.ok()).toBe(true);
	});

	test('shows the parent and direct children in the task detail', async ({ page }) => {
		await page.goto('/flow/');
		await expect(page.getByRole('tab', { name: '보드', exact: true })).toHaveAttribute('aria-selected', 'true');
		await taskCard(page, taskDashboardTaskID).click();

		const relationships = page.locator('[data-task-relationships]');
		await expect(relationships.getByRole('heading', { name: '업무 관계' })).toBeVisible();
		await expect(relationships.getByText('연결된 부모 업무가 없습니다.')).toHaveCount(0);
		await expect(relationships.getByText('연결된 자녀 업무가 없습니다.')).toHaveCount(0);
		await expect(relationships.getByRole('button', { name: '부모 업무 추가', exact: true })).toHaveCount(0);
		await expect(relationships.getByRole('button', { name: '자녀 업무 추가', exact: true })).toHaveCount(0);
		await expect(relationships.getByRole('heading', { name: '자녀 업무', exact: true })).toBeVisible();
		const progress = relationships.getByLabel('자녀 업무 2 / 7 완료, 29%');
		await expect(progress).toBeVisible();
		await expect(progress).toHaveAttribute('data-slot', 'badge');
		await expect(progress.locator('[data-task-relationship-progress-ring]')).toBeVisible();
		await expect(relationships.locator('[data-slot="separator"]')).toHaveCount(1);
		await expect(relationships.locator('[data-task-relationship-task]')).toHaveCount(8);
		await expect(relationships.locator('[data-relationship-status-kind="completed"]')).toHaveCount(2);
		await expect(relationships.locator('[data-relationship-status-kind="incomplete"]')).toHaveCount(6);
		await expect(relationships.locator('[data-relationship-status-kind="excluded"]')).toHaveCount(0);
		await expect(relationships.locator('[data-task-relationship-list]')).toHaveClass(/divide-y/);
		const childRows = relationships.locator('[data-task-relationship-task]');
		for (let index = 0; index < 6; index += 1) {
			await expect(childRows.nth(index).locator('[data-relationship-status-kind="incomplete"]')).toBeVisible();
		}
		for (let index = 6; index < 8; index += 1) {
			await expect(childRows.nth(index).locator('[data-relationship-status-kind="completed"]')).toBeVisible();
		}
		const firstChild = relationships.locator('[data-task-relationship-task]').first();
		await expect(firstChild.locator('[data-slot="avatar"]')).toBeVisible();
	});

	test('shows relationship actions only after entering edit mode', async ({ page }) => {
		await page.goto('/flow/');
		await taskCard(page, taskDashboardTaskID).click();
		const relationships = page.locator('[data-task-relationships]');
		await relationships.getByRole('button', { name: /자녀 업무: 캘린더 원격 동기화 재시도 점검/ }).click();

		await expect(relationships.getByRole('button', { name: '부모 업무 추가', exact: true })).toHaveCount(0);
		await expect(relationships.getByText('Flow 주간 리포트 카드 정리', { exact: true })).toBeVisible();
		await expect(relationships.getByRole('button', { name: '자녀 업무 추가', exact: true })).toHaveCount(0);
		await expect(relationships.getByText('연결된 자녀 업무가 없습니다.')).toHaveCount(0);

		await page.getByRole('button', { name: '업무 수정', exact: true }).click();
		await expect(relationships.getByRole('button', { name: '부모 업무 추가', exact: true })).toHaveCount(0);
		await expect(relationships.getByRole('button', { name: '자녀 업무 추가', exact: true })).toBeVisible();
	});

	test('connects multiple existing child tasks from the edit sheet', async ({ page }) => {
		await page.goto('/flow/');
		await expect(page.getByRole('tab', { name: '보드', exact: true })).toHaveAttribute('aria-selected', 'true');
		await taskCard(page, taskDashboardTaskID).click();
		await page.getByRole('button', { name: '업무 수정', exact: true }).click();

		const relationships = page.locator('[data-task-relationships]');
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
		await taskCard(page, taskDashboardTaskID).click();
		await page.getByRole('button', { name: '업무 수정', exact: true }).click();

		const relationships = page.locator('[data-task-relationships]');
		await relationships.getByRole('button', { name: '자녀 업무 추가', exact: true }).click();
		page.once('dialog', (dialog) => dialog.dismiss());
		await page.getByRole('option', { name: '새 자녀 업무 만들기', exact: true }).click();
		await expect(page.getByRole('dialog', { name: '자녀 업무' })).toBeVisible();

		page.once('dialog', (dialog) => dialog.accept());
		await page.getByRole('option', { name: '새 자녀 업무 만들기', exact: true }).click();
		await expect(page.getByRole('heading', { name: '업무 추가', exact: true })).toBeVisible();
	});
});
