import { expect, test } from '@playwright/test';
import { flowDashboardTaskID, marketScanTaskID, openFlowBoard, taskCard, useMemberFlowSession } from './flow-task-helpers';

test.describe('flow task sidebar', () => {
	test.beforeEach(async ({ request }) => {
		const response = await request.post('/flow/api/test/reset');
		expect(response.ok()).toBe(true);
	});

	test('explains why a non-participant task editor is read-only', async ({ page }) => {
		await useMemberFlowSession(page, 'designer@example.com', '이영희');
		await openFlowBoard(page);

		await taskCard(page, marketScanTaskID).click();
		await expect(page.getByText('관리자 또는 참여자만 수정할 수 있습니다.')).toBeVisible();
		await expect(page.getByRole('button', { name: '업무 저장', exact: true })).toHaveCount(0);
		await expect(page.getByRole('button', { name: '업무 삭제', exact: true })).toHaveCount(0);
	});

	test('keeps assignment fields locked for a participant who is not the owner', async ({ page }) => {
		await useMemberFlowSession(page, 'engineer@example.com', '박민준');
		await openFlowBoard(page);

		await taskCard(page, flowDashboardTaskID).click();
		await expect(page.getByPlaceholder('업무 내용')).toBeEnabled();
		await expect(page.getByRole('button', { name: '업무 저장', exact: true })).toBeVisible();
		await expect(page.getByText('관리자 또는 참여자만 수정할 수 있습니다.')).toHaveCount(0);
		await expect(page.getByRole('button', { name: /제거$/ })).toHaveCount(0);
	});

	test('keeps the owner participant locked while assignment fields are editable', async ({ page }) => {
		await openFlowBoard(page);

		await taskCard(page, flowDashboardTaskID).click();

		await expect(page.getByRole('button', { name: '김철수 제거', exact: true })).toHaveCount(0);
		await expect(page.getByRole('button', { name: '박민준 제거', exact: true })).toBeVisible();
	});
});
