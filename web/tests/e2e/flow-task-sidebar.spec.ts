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

	test('keeps task editor dropdown options interactive above the sheet', async ({ page }) => {
		await openFlowBoard(page);

		await taskCard(page, flowDashboardTaskID).click();

		for (const selection of [
			{ trigger: '담당자', option: '이영희', expected: '이영희' },
			{ trigger: '상태', option: '완료', expected: '완료' },
			{ trigger: '사업', option: '김인턴', expected: '김인턴' },
			{ trigger: '종류', option: '개선', expected: '개선' },
			{ trigger: '크기', option: 'L · 5km · 16h', expected: 'L · 5km · 16h' }
		]) {
			const trigger = page.getByRole('button', { name: selection.trigger, exact: true });
			await trigger.click();
			await page.getByRole('option', { name: selection.option, exact: true }).click();
			await expect(trigger).toContainText(selection.expected);
		}
	});
});
