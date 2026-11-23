import { expect, test, type Page } from '@playwright/test';
import { pickDate } from './date-picker-test-utils';
import { leaveRowsOf, seoulDateToday, seoulInstant, signInToAttendance } from './attendance-central-test-utils';
import { cleanupLeave, member1ID, member3ID, member3Name, seedLeave } from '../support/central-test-utils';

test.describe.configure({ mode: 'serial', timeout: 120_000 });
test.use({ locale: 'ko-KR' });

const movedSubject = 'E2E 변경 원본';
const movedReason = 'E2E 변경 요청';
const takenBackSubject = 'E2E 취소 대상';
const correctedSubject = 'E2E 날짜 수정 대상';

let movedLeaveID = '';
let takenBackLeaveID = '';
let correctedLeaveID = '';

function seoulDateShifted(date: string, offsetDays: number): string {
	const day = new Date(`${date}T00:00:00+09:00`);
	day.setUTCDate(day.getUTCDate() + offsetDays);
	return new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Seoul' }).format(day);
}

function seoulDateAfter(offsetDays: number): string {
	return seoulDateShifted(seoulDateToday(), offsetDays);
}

const movedFrom = seoulDateAfter(40);
const movedTo = seoulDateAfter(41);
const takenBackDate = seoulDateAfter(43);
const correctedFrom = seoulDateAfter(-10);
const correctedTo = seoulDateAfter(-9);

function approvedWholeDay(date: string, note: string, memberID = member1ID) {
	return {
		memberID,
		kind: 'annual',
		days: 1,
		status: 'approved' as const,
		startISO: seoulInstant(date, '00:00'),
		endISO: seoulInstant(seoulDateShifted(date, 1), '00:00'),
		note
	};
}

test.beforeAll(async () => {
	[movedLeaveID] = await seedLeave([approvedWholeDay(movedFrom, movedSubject)]);
	[takenBackLeaveID] = await seedLeave([approvedWholeDay(takenBackDate, takenBackSubject)]);
	[correctedLeaveID] = await seedLeave([approvedWholeDay(correctedFrom, correctedSubject, member3ID)]);
});

test.afterAll(async () => {
	const left = (await leaveRowsOf(member1ID)).filter((row) =>
		[movedSubject, movedReason, takenBackSubject].includes(row.note ?? '')
	);
	await cleanupLeave([movedLeaveID, takenBackLeaveID, correctedLeaveID, ...left.map((row) => row.id)]);
});

async function openLeaveHistory(page: Page): Promise<void> {
	await signInToAttendance(page);
	await page.getByTestId('leave-history-navigation').click();
	await page.getByTestId('leave-history-view').waitFor({ state: 'visible', timeout: 20000 });
}

test('an approved leave asks for other days and stands until they are approved', async ({ page }) => {
	await openLeaveHistory(page);

	const original = page.getByTestId('leave-history-row').filter({ hasText: movedSubject });
	await original.getByRole('button', { name: '변경', exact: true }).click();

	const form = page.getByTestId('leave-change-dialog').getByTestId('leave-request-form');
	await form.waitFor({ state: 'visible', timeout: 20000 });
	await pickDate(page, form.getByTestId('leave-request-date-field').getByRole('button'), movedTo);
	await form.getByRole('textbox').last().fill(movedReason);
	await form.getByRole('button', { name: '승인 요청' }).click();

	await expect
		.poll(
			async () => {
				const rows = await leaveRowsOf(member1ID);
				return {
					asked: rows.find((row) => row.note === movedReason)?.status ?? 'missing',
					original: rows.find((row) => row.id === movedLeaveID)?.status ?? 'missing'
				};
			},
			{ timeout: 30000 }
		)
		.toEqual({ asked: 'requested', original: 'approved' });

	const asked = page.getByTestId('leave-history-row').filter({ hasText: movedReason });
	await expect(asked.getByTestId('leave-history-replaces')).toContainText(movedFrom, { timeout: 20000 });
	await expect(original.getByRole('button', { name: '변경', exact: true })).toHaveCount(0);
});

test('the approval inbox shows the days a request moves from, and approving it moves the leave', async ({ page }) => {
	const asked = (await leaveRowsOf(member1ID)).find((row) => row.note === movedReason);
	expect(asked).toBeDefined();

	await signInToAttendance(page);
	await page.getByTestId('leave-approval-navigation').click();
	const card = page.getByTestId(`leave-approval-request-${asked?.id}`);
	await expect(card.getByTestId('leave-approval-replaces')).toBeVisible({ timeout: 20000 });
	await expect(card).toContainText(`${movedFrom} → ${movedTo}`);

	await card.getByRole('button', { name: '승인', exact: true }).click();
	await expect
		.poll(
			async () => {
				const rows = await leaveRowsOf(member1ID);
				return {
					asked: rows.find((row) => row.id === asked?.id)?.status ?? 'missing',
					original: rows.find((row) => row.id === movedLeaveID)?.status ?? 'missing'
				};
			},
			{ timeout: 30000 }
		)
		.toEqual({ asked: 'approved', original: 'missing' });
});

test('an approved leave that has not started is taken back by its owner', async ({ page }) => {
	await openLeaveHistory(page);

	const approved = page.getByTestId('leave-history-row').filter({ hasText: takenBackSubject });
	await approved.getByRole('button', { name: '휴가 취소' }).click();
	await page.getByRole('alertdialog').getByRole('button', { name: '휴가 취소' }).click();

	await expect
		.poll(async () => (await leaveRowsOf(member1ID)).some((row) => row.id === takenBackLeaveID), { timeout: 30000 })
		.toBe(false);
	await expect(approved).toHaveCount(0, { timeout: 20000 });
});

test('an administrator moves a past leave to other days and has to say why', async ({ page }) => {
	await signInToAttendance(page);
	await page.getByTestId('leave-management-navigation').click();
	await page.getByTestId('leave-management-view').waitFor({ state: 'visible', timeout: 20000 });
	await page.getByTestId('leave-management-employee-row').filter({ hasText: member3Name }).first().click();

	const managed = page.locator(`[data-managed-leave-request="${correctedLeaveID}"]`);
	await managed.getByRole('button', { name: '날짜 수정' }).click();
	const dialog = page.getByTestId('leave-days-correction-dialog');
	const save = dialog.getByRole('button', { name: '날짜 저장' });
	await pickDate(page, dialog.getByTestId('leave-days-correction-start').getByRole('button'), correctedTo);
	await expect(save).toBeDisabled();
	await dialog.getByRole('textbox').fill('E2E 일정 정정');
	await save.click();

	await expect
		.poll(
			async () => {
				const corrected = (await leaveRowsOf(member3ID)).find((row) => row.id === correctedLeaveID);
				return corrected ? new Date(corrected.starts_at).toISOString() : 'missing';
			},
			{ timeout: 30000 }
		)
		.toBe(new Date(seoulInstant(correctedTo, '00:00')).toISOString());
});
