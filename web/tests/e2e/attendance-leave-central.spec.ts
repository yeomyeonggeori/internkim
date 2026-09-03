import { expect, test, type Page } from '@playwright/test';
import {
	leaveRowsOf,
	seoulDateToday,
	seoulInstant,
	signInToAttendance
} from './attendance-central-test-utils';
import { cleanupLeave, member1ID, member3ID, member3Name, seedLeave } from './central-test-utils';

test.describe.configure({ mode: 'serial', timeout: 120_000 });
test.use({ locale: 'ko-KR' });

const requestedReason = 'E2E 휴가 신청';
const approvalSubject = 'E2E 승인 대상';
const rejectionSubject = 'E2E 반려 대상';

let approvalLeaveID = '';
let rejectionLeaveID = '';
let requestedLeaveIDs: string[] = [];

function laterThisMonth(offsetDays: number): string {
	const today = new Date(`${seoulDateToday()}T00:00:00+09:00`);
	today.setUTCDate(today.getUTCDate() + offsetDays);
	return new Intl.DateTimeFormat('en-CA', {
		timeZone: 'Asia/Seoul',
		year: 'numeric',
		month: '2-digit',
		day: '2-digit'
	}).format(today);
}

const requestedDate = laterThisMonth(10);
const approvalDate = laterThisMonth(12);
const rejectionDate = laterThisMonth(14);
const pendingCountDate = laterThisMonth(16);

function wholeDay(date: string): { startISO: string; endISO: string } {
	const next = new Date(`${date}T00:00:00+09:00`);
	next.setUTCDate(next.getUTCDate() + 1);
	const nextDate = new Intl.DateTimeFormat('en-CA', {
		timeZone: 'Asia/Seoul',
		year: 'numeric',
		month: '2-digit',
		day: '2-digit'
	}).format(next);
	return { startISO: seoulInstant(date, '00:00'), endISO: seoulInstant(nextDate, '00:00') };
}

test.beforeAll(async () => {
	[approvalLeaveID] = await seedLeave([
		{
			memberID: member3ID,
			kind: 'annual',
			days: 1,
			status: 'requested',
			...wholeDay(approvalDate),
			note: approvalSubject
		}
	]);
	[rejectionLeaveID] = await seedLeave([
		{
			memberID: member3ID,
			kind: 'annual',
			days: 1,
			status: 'requested',
			...wholeDay(rejectionDate),
			note: rejectionSubject
		}
	]);
});

test.afterAll(async () => {
	await cleanupLeave([approvalLeaveID, rejectionLeaveID, ...requestedLeaveIDs]);
});

async function openLeaveHistory(page: Page): Promise<void> {
	await signInToAttendance(page);
	await page.getByTestId('leave-history-navigation').click();
	await page.getByTestId('leave-history-view').waitFor({ state: 'visible', timeout: 20000 });
}

async function openApprovals(page: Page): Promise<void> {
	await signInToAttendance(page);
	await page.getByTestId('leave-approval-navigation').click();
	await page.getByTestId('leave-approval-view').waitFor({ state: 'visible', timeout: 20000 });
}

function approvalCard(page: Page, leaveID: string) {
	return page.getByTestId(`leave-approval-request-${leaveID}`);
}

async function statusOf(memberID: string, leaveID: string): Promise<string> {
	const rows = await leaveRowsOf(memberID);
	return rows.find((row) => row.id === leaveID)?.status ?? 'missing';
}

test('leave_request writes the leave the form asked for', async ({ page }) => {
	await signInToAttendance(page);
	await page.getByRole('button', { name: '휴가 등록' }).first().click();

	const form = page.getByTestId('leave-request-dialog').getByTestId('leave-request-form');
	await form.waitFor({ state: 'visible', timeout: 20000 });
	await form.getByTestId('leave-request-type-trigger').click();
	await page.getByRole('option', { name: '연차', exact: true }).click();
	await form.getByTestId('leave-request-date-field').locator('input[type="date"]').fill(requestedDate);
	await form.getByRole('textbox').last().fill(requestedReason);
	await form.getByRole('button', { name: '승인 요청' }).click();

	await expect
		.poll(
			async () => {
				const rows = await leaveRowsOf(member1ID);
				const created = rows.find((row) => row.note === requestedReason);
				requestedLeaveIDs = created ? [created.id] : [];
				return created
					? { kind: created.kind, status: created.status, days: Number(created.days) }
					: null;
			},
			{ timeout: 30000 }
		)
		.toEqual({ kind: 'annual', status: 'requested', days: 1 });

	await page.getByTestId('leave-history-navigation').click();
	const historyRow = page.getByTestId('leave-history-row').filter({ hasText: requestedReason });
	await expect(historyRow).toBeVisible({ timeout: 20000 });
	await expect(historyRow).toContainText('승인 대기');
});

test('leave_delete takes the pending request back out of the record', async ({ page }) => {
	await openLeaveHistory(page);

	const historyRow = page.getByTestId('leave-history-row').filter({ hasText: requestedReason });
	await historyRow.getByRole('button', { name: '신청 취소' }).click();
	await page.getByRole('alertdialog').getByRole('button', { name: '신청 취소' }).click();

	await expect
		.poll(
			async () => (await leaveRowsOf(member1ID)).filter((row) => row.note === requestedReason).length,
			{ timeout: 30000 }
		)
		.toBe(0);
	await expect(historyRow).toHaveCount(0, { timeout: 20000 });
});

test('leave_decide approves the request the inbox shows', async ({ page }) => {
	await openApprovals(page);

	const card = approvalCard(page, approvalLeaveID);
	await expect(card).toContainText(member3Name);
	await expect(card).toContainText(approvalSubject);
	await card.getByRole('button', { name: '승인', exact: true }).click();

	await expect.poll(() => statusOf(member3ID, approvalLeaveID), { timeout: 30000 }).toBe('approved');
	await expect(card).toHaveCount(0, { timeout: 20000 });
});

test('leave_decide rejects only after the confirmation', async ({ page }) => {
	await openApprovals(page);

	const card = approvalCard(page, rejectionLeaveID);
	await card.getByRole('button', { name: '반려', exact: true }).click();
	await expect(page.getByRole('alertdialog')).toBeVisible();
	expect(await statusOf(member3ID, rejectionLeaveID)).toBe('requested');

	await page.getByRole('alertdialog').getByRole('button', { name: '반려 확정' }).click();

	await expect.poll(() => statusOf(member3ID, rejectionLeaveID), { timeout: 30000 }).toBe('rejected');
	await expect(card).toHaveCount(0, { timeout: 20000 });
});

test('the approval inbox counts what the record still holds', async ({ page }) => {
	const pendingLeaveIDs = await seedLeave([
		{
			memberID: member3ID,
			kind: 'annual',
			days: 1,
			status: 'requested',
			...wholeDay(pendingCountDate),
			note: 'E2E 대기 건수'
		}
	]);
	try {
		await signInToAttendance(page);

		const pendingRows = (await leaveRowsOf(member3ID)).filter((row) => row.status === 'requested');
		await expect(page.getByTestId('leave-approval-navigation')).toContainText(
			String(pendingRows.length),
			{ timeout: 20000 }
		);
	} finally {
		await cleanupLeave(pendingLeaveIDs);
	}
});
