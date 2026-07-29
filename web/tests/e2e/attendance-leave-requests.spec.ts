import type { Page } from '@playwright/test';
import {
	createDevEmployeeLeaveMockResponse,
	createDevEmployeeLeaveMockState,
	type DevEmployeeLeaveMockState
} from '../../dev-attendance-leave-mock';
import {
	createDevLeaveApprovalMockResponse,
	createDevLeaveApprovalMockState
} from '../../dev-attendance-leave-approval-mock';
import { buildAttendanceSummaryFixture } from '../../dev-attendance-summary-fixture';
import { expect, test } from './attendance-page-test-fixture';
import { selectKorean } from './attendance-test-helpers';

test.describe('employee leave requests', () => {
	test('submits a full-day request from the desktop dialog without changing attendance records', async ({
		page
	}) => {
		const svelteOwnershipWarnings: string[] = [];
		page.on('console', (message) => {
			if (message.text().includes('ownership_invalid_binding')) {
				svelteOwnershipWarnings.push(message.text());
			}
		});
		const leaveState = await installLeaveMock(page);
		let attendanceSummaryRequestCount = 0;
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			attendanceSummaryRequestCount += 1;
			const requestURL = new URL(route.request().url());
			const summary = buildAttendanceSummaryFixture(
				requestURL.searchParams.get('month') ?? '2026-08'
			);
			await route.fulfill({
				json: {
					...summary,
					month: '2026-08',
					absences: summary.absences.filter((absence) => absence.email !== summary.currentUserEmail)
				}
			});
		});
		await page.clock.setFixedTime(new Date('2026-08-01T10:00:00+09:00'));
		await page.goto('/attendance');
		await selectKorean(page);

		const leaveSummary = page.getByTestId('leave-balance-summary');
		const approvalNavigation = page.getByTestId('leave-approval-navigation');
		const initialPendingCount = Number(
			(await approvalNavigation.locator('[data-slot="badge"]').textContent()) ?? '0'
		);
		await expect(leaveSummary.getByText('사용')).toBeVisible();
		await expect(leaveSummary.getByText('1일')).toBeVisible();
		await expect(leaveSummary.getByText('0.5일')).toBeVisible();
		await expect(leaveSummary.getByText('13.5일')).toBeVisible();
		await expect(page.getByLabel('내 근무 시간')).toBeVisible();
		await expect(page.getByTestId('leave-history-needs-changes-count')).toHaveText('1');

		await page.getByRole('button', { name: '휴가 등록' }).click();
		const dialog = page.getByTestId('leave-request-dialog');
		await expect(dialog).toBeVisible();
		await expect(dialog.getByRole('tab')).toHaveCount(0);
		const dialogBox = await dialog.boundingBox();
		expect(dialogBox?.width ?? 0).toBeGreaterThan(700);

		await dialog.getByLabel('시작일').fill('2026-08-14');
		await dialog.getByLabel('종료일').fill('2026-08-17');
		const preview = dialog.getByTestId('leave-request-preview');
		await expect(preview.getByText('2026-08-14')).toBeVisible();
		await expect(dialog.getByText('2026-08-15')).toBeVisible();
		await expect(dialog.getByText('비근무일').first()).toBeVisible();
		await expect(dialog.getByText('공휴일')).toBeVisible();
		await expect(preview.getByText('1일').last()).toBeVisible();
		await expect(dialog.getByLabel('신청 사유 (선택)')).toHaveValue('');

		await dialog.getByRole('button', { name: '승인 요청' }).click();

		await expect(dialog).toBeHidden();
		await expect(approvalNavigation.locator('[data-slot="badge"]')).toHaveText(
			String(initialPendingCount + 1)
		);
		const createdRequest = leaveState.payload.requests.find(
			(request) => request.startDate === '2026-08-14'
		);
		expect(createdRequest).toBeDefined();
		await approvalNavigation.click();
		await expect(page.getByTestId(`leave-approval-request-${createdRequest?.id}`)).toContainText(
			'2026-08-14'
		);
		await page.getByRole('button', { name: '휴가 내역' }).click();
		await expect(page.getByTestId('leave-history-view')).toBeVisible();
		expect(createdRequest).toMatchObject({
			status: 'pending',
			unit: 'fullDay',
			startDate: '2026-08-14',
			endDate: '2026-08-17',
			deductionMilliDays: 1000,
			reason: '',
			attachments: []
		});
		expect(attendanceSummaryRequestCount).toBe(2);
		expect(svelteOwnershipWarnings).toEqual([]);
	});

	test('keeps the post-submission history when an older leave load finishes later', async ({
		page
	}) => {
		const leaveState = await installLeaveMock(page);
		let leaveLoadCount = 0;
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
		await page.route('**/attendance/api/leave', async (route) => {
			if (route.request().method() !== 'GET') {
				await route.fallback();
				return;
			}
			leaveLoadCount += 1;
			const response = createDevEmployeeLeaveMockResponse(leaveState, {
				method: 'GET',
				pathname: '/attendance/api/leave'
			});
			if (!response) throw new Error('employee leave fixture response is missing');
			const responseBody = JSON.stringify(response.body);
			if (leaveLoadCount === 2) {
				startStaleResponse();
				await staleResponseReleased;
				await route.fulfill({
					status: response.status,
					contentType: 'application/json',
					body: responseBody
				});
				completeStaleResponse();
				return;
			}
			await route.fulfill({
				status: response.status,
				contentType: 'application/json',
				body: responseBody
			});
		});
		await page.clock.setFixedTime(new Date('2026-08-01T10:00:00+09:00'));
		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByRole('button', { name: '휴가 등록' }).click();
		await staleResponseStarted;

		const dialog = page.getByTestId('leave-request-dialog');
		await dialog.getByLabel('시작일').fill('2026-08-21');
		await dialog.getByLabel('종료일').fill('2026-08-21');
		await dialog.getByRole('button', { name: '승인 요청' }).click();
		await expect(dialog).toBeHidden();

		releaseStaleResponse();
		await staleResponseCompleted;
		await page.getByRole('button', { name: '휴가 내역' }).click();
		const createdRow = page
			.getByTestId('leave-history-row')
			.filter({ hasText: '2026-08-21' });
		await expect(createdRow).toBeVisible();
		await expect(createdRow.getByText('승인 대기', { exact: true })).toBeVisible();
	});

	test('uses the server preview for half-day and quarter-day custom times with optional evidence', async ({
		page
	}) => {
		const leaveState = await installLeaveMock(page);
		await page.clock.setFixedTime(new Date('2026-08-01T10:00:00+09:00'));
		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByRole('button', { name: '휴가 등록' }).click();
		const dialog = page.getByTestId('leave-request-dialog');

		await dialog.getByRole('button', { name: '반차', exact: true }).click();
		const dateFieldBox = await dialog.getByTestId('leave-request-date-field').boundingBox();
		const partialTimeBox = await dialog.getByTestId('leave-partial-time-fields').boundingBox();
		expect(dateFieldBox).not.toBeNull();
		expect(partialTimeBox).not.toBeNull();
		expect(partialTimeBox?.x).toBeGreaterThan(dateFieldBox?.x ?? 0);
		expect(Math.abs((partialTimeBox?.y ?? 0) - (dateFieldBox?.y ?? 0))).toBeLessThanOrEqual(1);
		await dialog.getByLabel('날짜').fill('2026-08-18');
		const preview = dialog.getByTestId('leave-request-preview');
		await expect(preview.getByText('09:00–14:00')).toBeVisible();
		await dialog.getByRole('button', { name: '오후' }).click();
		await expect(preview.getByText('14:00–18:00')).toBeVisible();
		await dialog.getByRole('button', { name: '직접 입력' }).click();
		const customStartTime = dialog.getByLabel('시작 시각');
		const customStartTimeBox = await dialog.getByTestId('leave-start-time-field').boundingBox();
		expect(customStartTimeBox).not.toBeNull();
		expect(customStartTimeBox?.y).toBeGreaterThan(dateFieldBox?.y ?? 0);
		expect(Math.abs((customStartTimeBox?.x ?? 0) - (dateFieldBox?.x ?? 0))).toBeLessThanOrEqual(1);
		await expect(customStartTime).toHaveAccessibleName('시작 시각');
		await expect(customStartTime).toHaveAttribute('id', 'leave-request-start-time');
		await expect(dialog.locator('label[for="leave-request-start-time"]')).toHaveText('시작 시각');
		await expect(
			dialog.getByText('종료 시각은 소정근로시간과 휴게시간을 반영해 자동 계산됩니다.')
		).toHaveCount(0);
		await customStartTime.fill('11:00');
		await expect(preview.getByText('11:00–16:00')).toBeVisible();
		await expect(preview.getByText('0.5일').last()).toBeVisible();

		await dialog.getByRole('button', { name: '반반차', exact: true }).click();
		await expect(dialog.getByRole('button', { name: '오전' })).toHaveCount(0);
		await expect(dialog.getByRole('button', { name: '오후' })).toHaveCount(0);
		await expect(dialog.getByRole('button', { name: '직접 입력' })).toHaveCount(0);
		await expect(dialog.getByLabel('시작 시각')).toBeVisible();
		const quarterDayStartTimeBox = await dialog.getByTestId('leave-start-time-field').boundingBox();
		expect(quarterDayStartTimeBox).not.toBeNull();
		expect(quarterDayStartTimeBox?.x).toBeGreaterThan(dateFieldBox?.x ?? 0);
		expect(Math.abs((quarterDayStartTimeBox?.y ?? 0) - (dateFieldBox?.y ?? 0))).toBeLessThanOrEqual(
			1
		);
		await expect(preview.getByText('11:00–14:00')).toBeVisible();
		await expect(preview.getByText('0.25일').last()).toBeVisible();

		const fileInput = dialog.locator('input[type="file"]');
		await expect(dialog.getByText('JPG, PNG, PDF · 파일당 최대 10MB · 최대 5개')).toBeVisible();
		await fileInput.setInputFiles({
			name: 'appointment.pdf',
			mimeType: 'application/pdf',
			buffer: Buffer.from('evidence')
		});
		await expect(
			dialog.getByTestId('leave-evidence-picker').getByText('appointment.pdf')
		).toBeVisible();
		await dialog.getByLabel('신청 사유').fill('관공서 방문');
		await dialog.getByRole('button', { name: '승인 요청' }).click();

		const createdRequest = leaveState.payload.requests.find(
			(request) => request.reason === '관공서 방문' && request.status === 'pending'
		);
		expect(createdRequest).toMatchObject({
			unit: 'quarterDay',
			partialPeriod: 'custom',
			startTime: '11:00',
			endTime: '14:00',
			deductionMilliDays: 250
		});
		expect(createdRequest?.attachments[0]).toMatchObject({
			fileName: 'appointment.pdf',
			contentType: 'application/pdf'
		});
	});

	test('labels the custom start time input in English', async ({ page }) => {
		await page.unroute('**/admin/api/locale');
		await page.route('**/admin/api/locale', async (route) => {
			await route.fulfill({ json: { locale: 'en' } });
		});
		await installLeaveMock(page);
		await page.clock.setFixedTime(new Date('2026-08-01T10:00:00+09:00'));
		await page.goto('/attendance');
		await page.getByRole('button', { name: 'Request leave' }).click();
		const dialog = page.getByTestId('leave-request-dialog');

		await dialog.getByRole('button', { name: 'Quarter day', exact: true }).click();
		const customStartTime = dialog.getByLabel('Start time');

		await expect(customStartTime).toHaveAccessibleName('Start time');
		await expect(customStartTime).toHaveAttribute('id', 'leave-request-start-time');
		await expect(dialog.locator('label[for="leave-request-start-time"]')).toHaveText('Start time');
	});

	test('shows unified private history and supports cancel and resubmit workflows', async ({
		page
	}) => {
		const leaveState = await installLeaveMock(page);
		await page.clock.setFixedTime(new Date('2026-08-01T10:00:00+09:00'));
		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByRole('button', { name: '휴가 내역' }).click();
		const historyView = page.getByTestId('leave-history-view');

		const historyRows = historyView.getByTestId('leave-history-row');
		await expect(historyRows.first()).toContainText('개인 일정');
		await expect(
			historyView.getByText('방문 일정을 확인할 수 있는 자료를 보완해 주세요.')
		).toBeVisible();
		await expect(historyView.getByText('appointment.pdf')).toBeVisible();
		await historyView.getByRole('button', { name: '잔여량 변동' }).click();
		await expect(historyRows).toHaveCount(3);
		await expect(historyView.getByText('1 / 3')).toBeVisible();
		await expect(historyRows.first()).toContainText('신청 예약');
		await expect(historyView.getByRole('button', { name: '전체' })).toHaveCount(0);
		await historyView.getByRole('button', { name: '신청' }).click();
		await expect(historyRows).toHaveCount(3);
		await historyView.getByRole('button', { name: '다음' }).click();
		await expect(historyRows.filter({ hasText: '일정 취소' }).getByText('취소됨')).toBeVisible();
		await expect(
			historyRows.filter({ hasText: '개인 용무' }).getByRole('button', { name: '신청 취소' })
		).toBeVisible();
		await historyView.getByRole('button', { name: '이전' }).click();

		let pendingRow = historyRows.filter({ hasText: '개인 일정' }).first();
		const editButton = pendingRow.getByRole('button', {
			name: '수정',
			exact: true
		});
		const cancelButton = pendingRow.getByRole('button', { name: '신청 취소' });
		const editButtonBox = await editButton.boundingBox();
		const cancelButtonBox = await cancelButton.boundingBox();
		expect(editButtonBox?.x ?? 0).toBeLessThan(cancelButtonBox?.x ?? 0);
		await editButton.click();
		const editDialog = page.getByTestId('leave-request-dialog');
		await expect(editDialog.getByRole('heading', { name: '휴가 신청 수정' })).toBeVisible();
		await expect(editDialog.getByLabel('신청 사유')).toHaveValue('개인 일정');
		await expect(editDialog.getByText('personal-schedule.pdf')).toBeVisible();
		await editDialog
			.getByRole('button', {
				name: 'personal-schedule.pdf 첨부 제거'
			})
			.click();
		await editDialog.locator('input[type="file"]').setInputFiles({
			name: 'updated-schedule.pdf',
			mimeType: 'application/pdf',
			buffer: Buffer.from('updated evidence')
		});
		await editDialog.getByRole('button', { name: '반차', exact: true }).click();
		await editDialog.getByLabel('날짜').fill('2026-08-04');
		await editDialog.getByRole('button', { name: '오후' }).click();
		await editDialog.getByLabel('신청 사유').fill('개인 일정 변경');
		await editDialog.getByRole('button', { name: '수정 저장' }).click();
		await expect(editDialog).toBeHidden();

		const editedRequest = leaveState.payload.requests.find(
			(request) => request.id === 'leave-request-pending'
		);
		expect(editedRequest).toMatchObject({
			status: 'pending',
			unit: 'halfDay',
			startDate: '2026-08-04',
			partialPeriod: 'afternoon',
			reason: '개인 일정 변경',
			revision: 2,
			canEdit: true
		});
		expect(editedRequest?.attachments.map((attachment) => attachment.fileName)).toEqual([
			'updated-schedule.pdf'
		]);
		pendingRow = historyRows.filter({ hasText: '개인 일정 변경' }).first();
		await pendingRow.getByRole('button', { name: '신청 취소' }).click();
		const cancelConfirmation = page.getByRole('alertdialog', {
			name: '휴가 신청을 취소할까요?'
		});
		await expect(cancelConfirmation).toBeVisible();
		await cancelConfirmation.getByRole('button', { name: '신청 취소' }).click();
		await expect(pendingRow).toHaveCount(0);
		expect(
			leaveState.payload.requests.find((request) => request.id === 'leave-request-pending')
		).toBeUndefined();
		await expect(
			historyRows
				.filter({ hasText: '가족 일정' })
				.getByRole('button', { name: '수정', exact: true })
		).toHaveCount(0);

		const needsChangesRow = historyRows.filter({ hasText: '관공서 방문' }).first();
		await needsChangesRow.getByRole('button', { name: '다시 제출' }).click();
		const dialog = page.getByTestId('leave-request-dialog');
		await expect(dialog.getByRole('heading', { name: '보완 요청된 신청' })).toBeVisible();
		await expect(
			dialog.getByTestId('leave-request-form').getByText('appointment.pdf')
		).toBeVisible();
		await dialog.getByRole('button', { name: '취소', exact: true }).click();
		await historyRows
			.filter({ hasText: '관공서 방문' })
			.first()
			.getByRole('button', { name: '다시 제출' })
			.click();
		await dialog.getByLabel('신청 사유').fill('관공서 방문 일정 보완');
		await dialog.getByLabel('보완 답변 (선택)').fill('예약 확인 자료를 추가했습니다.');
		await dialog.getByRole('button', { name: '다시 제출' }).click();

		await expect(dialog).toBeHidden();
		const resubmittedRequest = leaveState.payload.requests.find(
			(request) => request.id === 'leave-request-needs-changes'
		);
		expect(resubmittedRequest).toMatchObject({
			status: 'pending',
			reason: '관공서 방문 일정 보완',
			adminResponse: '방문 일정을 확인할 수 있는 자료를 보완해 주세요.',
			canResubmit: false
		});
		await expect(page.getByTestId('leave-history-needs-changes-count')).toHaveCount(0);
		await expect(
			historyRows.filter({ hasText: '관공서 방문 일정 보완' }).getByText('승인 대기')
		).toBeVisible();
	});

	test('paginates each leave history tab by three items and resets on tab changes', async ({
		page
	}) => {
		const leaveState = await installLeaveMock(page);
		const sourceRequest = leaveState.payload.requests[0];
		if (!sourceRequest) throw new Error('leave request fixture is missing');
		leaveState.payload.requests = Array.from({ length: 12 }, (_, index) => ({
			...structuredClone(sourceRequest),
			id: `leave-request-page-${index + 1}`,
			reason: `페이지 항목 ${index + 1}`,
			createdAt: `2026-07-${String(20 - index).padStart(2, '0')}T13:20:00+09:00`
		}));

		await page.clock.setFixedTime(new Date('2026-08-01T10:00:00+09:00'));
		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByRole('button', { name: '휴가 내역' }).click();
		const historyView = page.getByTestId('leave-history-view');
		const historyRows = historyView.getByTestId('leave-history-row');

		await expect(historyRows).toHaveCount(3);
		await expect(historyView.getByText('전체 12개 중 1–3')).toBeVisible();
		await expect(historyView.getByText('1 / 4')).toBeVisible();
		await historyView.getByRole('button', { name: '다음' }).click();
		await expect(historyView.getByText('전체 12개 중 4–6')).toBeVisible();
		await historyView.getByRole('button', { name: '다음' }).click();
		await historyView.getByRole('button', { name: '다음' }).click();
		await expect(historyRows).toHaveCount(3);
		await expect(historyView.getByText('전체 12개 중 10–12')).toBeVisible();
		await historyView.getByRole('button', { name: '잔여량 변동' }).click();
		await expect(historyView.getByText('1 / 3')).toBeVisible();
		await historyView.getByRole('button', { name: '신청' }).click();
		await expect(historyRows).toHaveCount(3);
		await expect(historyView.getByText('1 / 4')).toBeVisible();
	});

	test('refreshes the latest request when an administrator wins an edit race', async ({ page }) => {
		const leaveState = await installLeaveMock(page);
		await page.clock.setFixedTime(new Date('2026-08-01T10:00:00+09:00'));
		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByRole('button', { name: '휴가 내역' }).click();
		const historyView = page.getByTestId('leave-history-view');
		const pendingRow = historyView.getByTestId('leave-history-row').filter({
			hasText: '개인 일정'
		});
		await pendingRow.getByRole('button', { name: '수정', exact: true }).click();
		const dialog = page.getByTestId('leave-request-dialog');
		const request = leaveState.payload.requests.find(
			(candidate) => candidate.id === 'leave-request-pending'
		);
		if (!request) throw new Error('pending fixture request is missing');
		request.status = 'approved';
		request.canEdit = false;
		request.canCancel = false;
		request.revision += 1;

		await dialog.getByLabel('신청 사유').fill('동시에 수정한 일정');
		await dialog.getByRole('button', { name: '수정 저장' }).click();

		await expect(dialog).toBeHidden();
		await expect(
			historyView.getByText('신청 상태가 변경되어 처리할 수 없습니다. 최신 내역을 확인해 주세요.')
		).toBeVisible();
		const refreshedRow = historyView.getByTestId('leave-history-row').filter({
			hasText: '개인 일정'
		});
		await expect(refreshedRow.getByText('승인', { exact: true })).toBeVisible();
		await expect(refreshedRow.getByRole('button', { name: '수정', exact: true })).toHaveCount(0);
	});

	test('localizes leave error codes without exposing server error prose', async ({ page }) => {
		await installLeaveMock(page, {
			code: 'insufficientBalance',
			error: 'available balance is -250 for private employee account'
		});
		await page.clock.setFixedTime(new Date('2026-08-01T10:00:00+09:00'));
		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByRole('button', { name: '휴가 등록' }).click();
		const dialog = page.getByTestId('leave-request-dialog');

		await expect(dialog.getByText('사용 가능한 휴가 잔여량이 부족합니다.')).toBeVisible();
		await expect(
			dialog.getByText('available balance is -250 for private employee account')
		).toHaveCount(0);
	});

	test('uses the same overflow-safe full-screen dialog on mobile', async ({ page }) => {
		await installLeaveMock(page);
		await page.setViewportSize({ width: 390, height: 844 });
		await page.clock.setFixedTime(new Date('2026-08-01T10:00:00+09:00'));
		await page.goto('/attendance');
		await selectKorean(page);
		await expect(page.getByTestId('mobile-leave-history-needs-changes-count')).toHaveText('1');
		await page.getByRole('button', { name: '휴가 등록' }).click();
		const dialog = page.getByTestId('leave-request-dialog');
		await expect(dialog).toHaveCSS('transform', 'none');
		const dialogBox = await dialog.boundingBox();

		expect(dialogBox).toMatchObject({ x: 0, y: 0, width: 390, height: 844 });
		const overflow = await dialog.evaluate((element) => ({
			clientWidth: element.clientWidth,
			scrollWidth: element.scrollWidth
		}));
		expect(overflow.scrollWidth).toBeLessThanOrEqual(overflow.clientWidth);
		await dialog.getByRole('button', { name: '반반차', exact: true }).click();
		const dateFieldBox = await dialog.getByTestId('leave-request-date-field').boundingBox();
		const partialTimeBox = await dialog.getByTestId('leave-start-time-field').boundingBox();
		expect(dateFieldBox).not.toBeNull();
		expect(partialTimeBox).not.toBeNull();
		expect(partialTimeBox?.y).toBeGreaterThan(dateFieldBox?.y ?? 0);
		expect(Math.abs((partialTimeBox?.x ?? 0) - (dateFieldBox?.x ?? 0))).toBeLessThanOrEqual(1);

		const form = dialog.getByTestId('leave-request-form');
		await form.evaluate((element) => {
			element.scrollIntoView({ block: 'end' });
		});
		await expect(dialog.getByRole('button', { name: '승인 요청' })).toBeVisible();
		await expect(dialog.getByRole('tab')).toHaveCount(0);
		await dialog.getByRole('button', { name: '취소', exact: true }).click();
		await page.getByRole('tab', { name: '휴가 내역' }).click();
		await expect(page.getByTestId('leave-history-view')).toBeVisible();
	});
});

async function installLeaveMock(
	page: Page,
	previewFailure?: { code: string; error: string }
): Promise<DevEmployeeLeaveMockState> {
	const state = createDevEmployeeLeaveMockState();
	const approvalState = createDevLeaveApprovalMockState(state);
	await page.route('**/attendance/api/leave**', async (route) => {
		const request = route.request();
		const requestURL = new URL(request.url());
		if (
			previewFailure &&
			request.method() === 'POST' &&
			requestURL.pathname === '/attendance/api/leave-requests/preview'
		) {
			await route.fulfill({
				status: 409,
				contentType: 'application/json',
				body: JSON.stringify(previewFailure)
			});
			return;
		}
		const employeeResponse = createDevEmployeeLeaveMockResponse(state, {
			method: request.method(),
			pathname: requestURL.pathname,
			body: request.postData() ?? undefined,
			contentType: request.headers()['content-type']
		});
		const response =
			employeeResponse ??
			createDevLeaveApprovalMockResponse(approvalState, {
				method: request.method(),
				pathname: requestURL.pathname,
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
	return state;
}
