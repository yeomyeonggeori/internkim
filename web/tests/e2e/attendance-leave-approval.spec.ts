import type { Page, Route } from '@playwright/test';
import { buildAttendanceSummaryFixture } from '../../dev-attendance-summary-fixture';
import type {
	LeaveApprovalChange,
	LeaveApprovalInbox,
	LeaveApprovalRequest
} from '../../src/routes/attendance/approval/leave-approval-types';
import { expect, test } from './attendance-page-test-fixture';
import { selectKorean } from './attendance-test-helpers';

test.describe('administrator leave approvals', () => {
	test('shows the admin inbox and moves an approved request to recent changes', async ({ page }) => {
		const state = await installLeaveApprovalRoute(page);
		await page.goto('/attendance');
		await selectKorean(page);

		const navigation = page.getByTestId('leave-approval-navigation');
		await expect(navigation).toBeVisible();
		await expect(navigation).toContainText('1');
		await navigation.click();

		const view = page.getByTestId('leave-approval-view');
		await expect(view).toBeVisible();
		const requestCard = view.getByTestId('leave-approval-request-leave-approval-pending');
		await expect(requestCard.getByText('kim@example.com')).toBeVisible();
		await expect(requestCard.getByText('개인 일정')).toBeVisible();
		await expect(requestCard.getByText('첨부자료 없음')).toBeVisible();
		await requestCard.getByRole('button', { name: '승인', exact: true }).click();

		await expect(view.getByText('승인 대기 중인 휴가가 없습니다.')).toBeVisible();
		await expect(navigation).toContainText('0');
		await view.getByRole('tab', { name: '최근 변경' }).click();
		await expect(view.getByTestId('leave-approval-request-leave-approval-pending')).toContainText(
			'승인'
		);
		expect(state.decisions).toEqual([{ action: 'approve', response: '' }]);
	});

	test('keeps an approved request out of pending when an older inbox load finishes later', async ({
		page
	}) => {
		const state = await installLeaveApprovalRoute(page);
		let inboxLoadCount = 0;
		let startStaleResponse: () => void = () => undefined;
		let releaseStaleResponse: () => void = () => undefined;
		let completeStaleResponse: () => void = () => undefined;
		const staleResponseStarted = new Promise<void>((resolve) => {
			startStaleResponse = resolve;
		});
		const staleResponseReleased = new Promise<void>((resolve) => {
			releaseStaleResponse = resolve;
		});
		const staleResponseCompleted = new Promise<void>((resolve) => {
			completeStaleResponse = resolve;
		});
		await page.route('**/attendance/api/leave-approvals**', async (route) => {
			if (route.request().method() !== 'GET') {
				await route.fallback();
				return;
			}
			inboxLoadCount += 1;
			const responseBody = JSON.stringify(state.currentInbox());
			if (inboxLoadCount === 2) {
				startStaleResponse();
				await staleResponseReleased;
				await route.fulfill({
					contentType: 'application/json',
					body: responseBody
				});
				completeStaleResponse();
				return;
			}
			await route.fulfill({
				contentType: 'application/json',
				body: responseBody
			});
		});
		await page.goto('/attendance');
		await selectKorean(page);
		const navigation = page.getByTestId('leave-approval-navigation');
		await navigation.click();
		const view = page.getByTestId('leave-approval-view');
		await view.getByRole('button', { name: '새로고침' }).click();
		await staleResponseStarted;

		const requestCard = view.getByTestId('leave-approval-request-leave-approval-pending');
		await requestCard.getByRole('button', { name: '승인', exact: true }).click();
		await expect(view.getByText('승인 대기 중인 휴가가 없습니다.')).toBeVisible();

		releaseStaleResponse();
		await staleResponseCompleted;
		await expect(view.getByText('승인 대기 중인 휴가가 없습니다.')).toBeVisible();
		await expect(navigation).toContainText('0');
	});

	test('requires a reason for changes and allows rejection without one', async ({ page }) => {
		const state = await installLeaveApprovalRoute(page);
		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByTestId('leave-approval-navigation').click();

		const view = page.getByTestId('leave-approval-view');
		const requestCard = view.getByTestId('leave-approval-request-leave-approval-pending');
		await requestCard.getByRole('button', { name: '보완 요청' }).click();
		const changesDialog = page.getByRole('dialog', { name: '보완 요청' });
		await expect(changesDialog.getByRole('button', { name: '보완 요청 보내기' })).toBeDisabled();
		await changesDialog.getByLabel('관리자 사유').fill('날짜를 다시 확인해 주세요.');
		await changesDialog.getByRole('button', { name: '보완 요청 보내기' }).click();
		await expect(view.getByText('승인 대기 중인 휴가가 없습니다.')).toBeVisible();
		expect(state.decisions).toEqual([
			{ action: 'needsChanges', response: '날짜를 다시 확인해 주세요.' }
		]);

		state.resetPending();
		await view.getByRole('button', { name: '새로고침' }).click();
		const resetCard = view.getByTestId('leave-approval-request-leave-approval-pending');
		await resetCard.getByRole('button', { name: '반려', exact: true }).click();
		const rejectDialog = page.getByRole('dialog', { name: '휴가 반려' });
		await expect(rejectDialog.getByRole('button', { name: '반려하기' })).toBeEnabled();
		await rejectDialog.getByRole('button', { name: '반려하기' }).click();
		const rejectConfirmation = page.getByRole('alertdialog', {
			name: '이 휴가 신청을 반려할까요?'
		});
		await expect(rejectConfirmation).toBeVisible();
		expect(state.decisions.at(-1)).toEqual({
			action: 'needsChanges',
			response: '날짜를 다시 확인해 주세요.'
		});
		await rejectConfirmation.getByRole('button', { name: '반려 확정' }).click();
		expect(state.decisions.at(-1)).toEqual({ action: 'reject', response: '' });
	});

	test('shows an early return with the approved leave interval in recent changes', async ({ page }) => {
		const fixture = approvalInboxFixture();
		const approvedRequest: LeaveApprovalRequest = {
			...fixture.pending[0],
			status: 'approved'
		};
		await installLeaveApprovalRoute(page, {
			pendingCount: 0,
			pending: [],
			recentChanges: [
				{
					request: approvedRequest,
					change: 'earlyReturn',
					returnedAt: '2026-08-03T03:30:00Z',
					changedAt: '2026-08-03T03:30:00Z'
				}
			]
		});
		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByTestId('leave-approval-navigation').click();

		const view = page.getByTestId('leave-approval-view');
		await view.getByRole('tab', { name: '최근 변경' }).click();
		const requestCard = view.getByTestId('leave-approval-request-leave-approval-pending');
		await expect(requestCard).toContainText('조기 복귀');
		await expect(requestCard).toContainText('09:00 – 13:00');
		await expect(requestCard).toContainText('조기 복귀 시각');
		await expect(requestCard).toContainText('오후 12:30');
	});

	test('hides approval navigation and tabs from regular employees', async ({ page }) => {
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? '2026-07';
			await route.fulfill({
				json: {
					...buildAttendanceSummaryFixture(month),
					isAdmin: false
				}
			});
		});
		await installLeaveApprovalRoute(page);
		await page.setViewportSize({ width: 390, height: 844 });
		await page.goto('/attendance');
		await selectKorean(page);

		await expect(page.getByTestId('leave-approval-navigation')).toHaveCount(0);
		await expect(page.getByRole('tab', { name: '휴가 승인' })).toHaveCount(0);
		await expect(page.getByTestId('leave-approval-view')).toHaveCount(0);
	});

	test('shows the approval inbox as an administrator mobile tab', async ({ page }) => {
		await installLeaveApprovalRoute(page);
		await page.setViewportSize({ width: 390, height: 844 });
		await page.goto('/attendance');
		await selectKorean(page);

		const approvalTab = page.getByRole('tab', { name: /휴가 승인/ });
		await expect(approvalTab).toBeVisible();
		await approvalTab.click();
		const approvalView = page.getByTestId('leave-approval-view');
		await expect(approvalView).toBeVisible();
		await expect(approvalView.getByText('kim@example.com')).toBeVisible();
	});
});

type ApprovalRouteState = {
	decisions: Array<{ action: string; response: string }>;
	currentInbox: () => LeaveApprovalInbox;
	resetPending: () => void;
};

async function installLeaveApprovalRoute(
	page: Page,
	initialInbox = approvalInboxFixture()
): Promise<ApprovalRouteState> {
	let inbox = initialInbox;
	const decisions: Array<{ action: string; response: string }> = [];
	await page.route('**/attendance/api/leave-approvals**', async (route) => {
		await handleLeaveApprovalRoute(route, inbox, decisions, (nextInbox) => {
			inbox = nextInbox;
		});
	});
	return {
		decisions,
		currentInbox: () => inbox,
		resetPending: () => {
			inbox = approvalInboxFixture();
		}
	};
}

async function handleLeaveApprovalRoute(
	route: Route,
	inbox: LeaveApprovalInbox,
	decisions: Array<{ action: string; response: string }>,
	updateInbox: (inbox: LeaveApprovalInbox) => void
): Promise<void> {
	const request = route.request();
	if (request.method() === 'GET') {
		await route.fulfill({ json: inbox });
		return;
	}
	const decision = request.postDataJSON() as { action: string; response: string };
	decisions.push(decision);
	const pending = inbox.pending[0];
	if (!pending) {
		await route.fulfill({ status: 409, json: { code: 'invalidStatus' } });
		return;
	}
	const status = decision.action === 'approve' ? 'approved' : decision.action === 'reject' ? 'rejected' : 'needsChanges';
	const changedRequest: LeaveApprovalRequest = {
		...pending,
		status,
		adminResponse: decision.response,
		updatedAt: '2026-07-28T12:00:00+09:00'
	};
	const change: LeaveApprovalChange = {
		request: changedRequest,
		change: status,
		response: decision.response || undefined,
		changedAt: changedRequest.updatedAt
	};
	updateInbox({
		pendingCount: 0,
		pending: [],
		recentChanges: [change, ...inbox.recentChanges]
	});
	await route.fulfill({ json: { request: changedRequest } });
}

function approvalInboxFixture(): LeaveApprovalInbox {
	return {
		pendingCount: 1,
		pending: [
			{
				id: 'leave-approval-pending',
				employeeEmail: 'kim@example.com',
				leaveTypeID: 'annual',
				leaveTypeName: '연차',
				balanceMode: 'annual',
				status: 'pending',
				unit: 'halfDay',
				startDate: '2026-08-03',
				partialPeriod: 'morning',
				startTime: '09:00',
				endTime: '13:00',
				deductionMilliDays: 500,
				reason: '개인 일정',
				attachments: [],
				balance: {
					availableMilliDays: 9500,
					reservedMilliDays: 500,
					usedMilliDays: 5000
				},
				createdAt: '2026-07-27T13:20:00+09:00',
				updatedAt: '2026-07-27T13:20:00+09:00'
			}
		],
		recentChanges: []
	};
}
