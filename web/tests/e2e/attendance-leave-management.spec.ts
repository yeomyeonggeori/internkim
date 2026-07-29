import { createDevEmployeeLeaveMockState } from '../../dev-attendance-leave-mock';
import {
	createDevLeaveManagementMockResponse,
	createDevLeaveManagementMockState
} from '../../dev-attendance-leave-management-mock';
import { expect, test } from './attendance-page-test-fixture';
import { selectKorean } from './attendance-test-helpers';

test.describe('administrator employee leave management', () => {
	test('keeps the latest employee selection when responses finish out of order', async ({
		page
	}) => {
		const managementState = createDevLeaveManagementMockState(
			createDevEmployeeLeaveMockState()
		);
		let releaseFirstSelection: () => void = () => undefined;
		let markFirstSelectionRequested: () => void = () => undefined;
		const firstSelectionRequested = new Promise<void>((resolve) => {
			markFirstSelectionRequested = resolve;
		});
		await page.route('**/attendance/api/leave-management**', async (route) => {
			const request = route.request();
			const requestURL = new URL(request.url());
			if (
				request.method() === 'GET' &&
				requestURL.searchParams.get('email') === 'kim@example.com'
			) {
				markFirstSelectionRequested();
				await new Promise<void>((resolve) => {
					releaseFirstSelection = resolve;
				});
			}
			const response = createDevLeaveManagementMockResponse(managementState, {
				method: request.method(),
				pathname: requestURL.pathname,
				searchParams: requestURL.searchParams,
				body: request.postData() ?? undefined
			});
			if (!response) {
				await route.fallback();
				return;
			}
			await route.fulfill({
				status: response.status,
				contentType: 'application/json',
				body: JSON.stringify(response.body)
			});
		});
		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByTestId('leave-management-navigation').click();

		const view = page.getByTestId('leave-management-view');
		const kimButton = view.getByRole('button', {
			name: '김철수 kim@example.com'
		});
		const seoheeButton = view.getByRole('button', {
			name: '이서희 seohee@example.com'
		});
		await kimButton.click();
		await firstSelectionRequested;
		await seoheeButton.click();

		const detailHeader = view.getByTestId('leave-management-employee-detail-header');
		await expect(seoheeButton).toHaveAttribute('aria-current', 'true');
		await expect(detailHeader.getByText('이서희', { exact: true })).toBeVisible();
		await expect(detailHeader.getByText('seohee@example.com', { exact: true })).toBeVisible();

		releaseFirstSelection();
		await expect(seoheeButton).toHaveAttribute('aria-current', 'true');
		await expect(detailHeader.getByText('이서희', { exact: true })).toBeVisible();
		await expect(view.getByText('남음 11.5일')).toBeVisible();
	});

	test('reviews balances, adjusts leave, and adds a past leave record', async ({ page }) => {
		const managementState = createDevLeaveManagementMockState(
			createDevEmployeeLeaveMockState()
		);
		let shouldDelayManagementRequest = false;
		let releaseRefresh: () => void = () => undefined;
		let markRefreshRequested: () => void = () => undefined;
		const refreshRequested = new Promise<void>((resolve) => {
			markRefreshRequested = resolve;
		});
		await page.route('**/attendance/api/leave-management**', async (route) => {
			const request = route.request();
			const requestURL = new URL(request.url());
			if (shouldDelayManagementRequest) {
				markRefreshRequested();
				await new Promise<void>((resolve) => {
					releaseRefresh = resolve;
				});
				shouldDelayManagementRequest = false;
			}
			const response = createDevLeaveManagementMockResponse(managementState, {
				method: request.method(),
				pathname: requestURL.pathname,
				searchParams: requestURL.searchParams,
				body: request.postData() ?? undefined
			});
			if (!response) {
				await route.fallback();
				return;
			}
			await route.fulfill({
				status: response.status,
				contentType: 'application/json',
				body: JSON.stringify(response.body)
			});
		});
		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByTestId('leave-management-navigation').click();

		const view = page.getByTestId('leave-management-view');
		await expect(view).toBeVisible();
		await expect(view.getByText('김철수', { exact: true })).toBeVisible();
		await expect(view.getByText('이서희', { exact: true })).toBeVisible();
		const employeeButton = view.getByRole('button', {
			name: '김철수 kim@example.com'
		});
		await expect(employeeButton.locator('[data-slot="avatar"]')).toHaveCount(1);
		await employeeButton.click();
		await expect(employeeButton).toHaveAttribute('aria-current', 'true');
		await expect(employeeButton.locator('xpath=ancestor::tr')).toHaveAttribute(
			'data-state',
			'selected'
		);

		await expect(view.getByText('2026년 연차 부여')).toBeVisible();
		await expect(view.getByText('남음 13.5일')).toBeVisible();
		await expect(
			view.getByTestId('leave-management-employee-detail-header').locator('[data-slot="avatar"]')
		).toHaveCount(1);

		shouldDelayManagementRequest = true;
		const refreshButton = view.getByTestId('leave-management-refresh');
		await refreshButton.click();
		await refreshRequested;
		await expect(refreshButton).toHaveAttribute('aria-busy', 'true');
		await expect(refreshButton).toBeDisabled();
		releaseRefresh();
		await expect(refreshButton).toHaveAttribute('aria-busy', 'false');

		const approvedPartialLeaveRow = view
			.locator('[data-managed-leave-request]')
			.filter({ hasText: '2026-07-21' })
			.first();
		await approvedPartialLeaveRow.getByRole('button', { name: '시간 정정' }).click();
		const timeDialog = page.getByTestId('leave-time-correction-dialog');
		await timeDialog.getByLabel('시작 시간').fill('13:00');
		await timeDialog.getByLabel('종료 시간').fill('17:00');
		await timeDialog.getByLabel('사유').fill('실제 사용 시간 반영');
		await timeDialog.getByRole('button', { name: '시간 저장' }).click();

		await expect(timeDialog).toBeHidden();
		await expect(approvedPartialLeaveRow.getByText('13:00–17:00')).toBeVisible();

		await view.getByRole('button', { name: '휴가 잔여량 조정' }).click();
		const adjustmentDialog = page.getByTestId('leave-adjustment-dialog');
		await adjustmentDialog.getByLabel('조정 일수').fill('1');
		await expect(adjustmentDialog.getByLabel('사유 (선택)')).toHaveValue('');
		await adjustmentDialog.getByRole('button', { name: '조정 저장' }).click();

		await expect(adjustmentDialog).toBeHidden();
		await expect(view.getByText('남음 14.5일')).toBeVisible();

		await view.getByRole('button', { name: '과거 휴가 등록' }).click();
		const pastLeaveDialog = page.getByTestId('past-leave-dialog');
		await expect(pastLeaveDialog.getByLabel('시작일')).toHaveAttribute('max', /\d{4}-\d{2}-\d{2}/);
		await expect(pastLeaveDialog.getByLabel('종료일')).toHaveAttribute('max', /\d{4}-\d{2}-\d{2}/);
		await pastLeaveDialog.getByLabel('시작일').fill('2026-07-24');
		await pastLeaveDialog.getByLabel('종료일').fill('2026-07-24');
		await expect(pastLeaveDialog.getByLabel('사유 (선택)')).toHaveValue('');
		await pastLeaveDialog.getByRole('button', { name: '휴가 등록' }).click();

		await expect(pastLeaveDialog).toBeHidden();
		await expect(view.getByText('남음 13.5일')).toBeVisible();

		const pastLeaveRow = view
			.locator('[data-managed-leave-request]')
			.filter({ hasText: '2026-07-24' })
			.first();
		await pastLeaveRow.getByRole('button', { name: '휴가 취소' }).click();
		const cancelDialog = page.getByTestId('leave-management-cancel-dialog');
		await expect(cancelDialog.getByText('승인된 휴가를 취소할까요?')).toBeVisible();
		await cancelDialog.getByRole('button', { name: '취소 확정' }).click();

		await expect(cancelDialog).toBeHidden();
		await expect(pastLeaveRow.getByText('취소', { exact: true })).toBeVisible();
		await expect(view.getByText('남음 14.5일')).toBeVisible();
	});
});
