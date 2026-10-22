import { expect, test, type Page } from '@playwright/test';
import { pickDate } from './date-picker-test-utils';
import { signInToTheCentralPlane } from '../support/central-plane-sign-in';
import { centralPlaneAdminClient, cleanupLeave, member1ID } from '../support/central-test-utils';
import { attendanceRowsOf, cleanupAttendanceEvents } from './attendance-central-test-utils';

test.describe.configure({ mode: 'serial', timeout: 120_000 });
test.use({ locale: 'ko-KR' });

let leaveBeforeTheSpec = new Set<string>();
let attendanceBeforeTheSpec = new Set<string>();

async function leaveIDsOfMember1(): Promise<string[]> {
	const rows = await centralPlaneAdminClient().from('leave').select('id').eq('member_id', member1ID);
	if (rows.error) throw new Error(`Failed to read leave: ${rows.error.message}`);
	return rows.data.map((row) => String(row.id));
}

async function attendanceIDsOfMember1(): Promise<string[]> {
	return (await attendanceRowsOf(member1ID)).map((row) => row.id);
}

test.beforeAll(async () => {
	leaveBeforeTheSpec = new Set(await leaveIDsOfMember1());
	attendanceBeforeTheSpec = new Set(await attendanceIDsOfMember1());
});

test.afterAll(async () => {
	await cleanupLeave((await leaveIDsOfMember1()).filter((id) => !leaveBeforeTheSpec.has(id)));
	await cleanupAttendanceEvents((await attendanceIDsOfMember1()).filter((id) => !attendanceBeforeTheSpec.has(id)));
});

function seoulParts(): { date: string; hour: number; minute: number } {
	const parts = new Intl.DateTimeFormat('en-CA', {
		timeZone: 'Asia/Seoul',
		year: 'numeric',
		month: '2-digit',
		day: '2-digit',
		hour: '2-digit',
		minute: '2-digit',
		hour12: false
	}).formatToParts(new Date());
	const of = (type: string) => parts.find((part) => part.type === type)?.value ?? '';
	return {
		date: `${of('year')}-${of('month')}-${of('day')}`,
		hour: Number(of('hour')),
		minute: Number(of('minute'))
	};
}

const quarterDayMinutes = 120;
const latestQuarterDayStart = 23 * 60 + 59 - quarterDayMinutes;

function startedBeforeNowWithinTheDay(): string {
	const { hour, minute } = seoulParts();
	const started = hour * 60 + minute - 30;
	const clamped = Math.min(latestQuarterDayStart, Math.max(0, started));
	return `${String(Math.floor(clamped / 60)).padStart(2, '0')}:${String(clamped % 60).padStart(2, '0')}`;
}

async function requestLeaveCoveringNow(page: Page): Promise<void> {
	await signInToTheCentralPlane(page, '/example-co/attendance');
	await page.getByTestId('leave-history-navigation').click();
	await page.getByRole('button', { name: '휴가 신청', exact: true }).click();
	const form = page.getByTestId('leave-request-dialog').getByTestId('leave-request-form');
	await form.waitFor({ state: 'visible', timeout: 20000 });

	await form.getByTestId('leave-request-unit-control').getByRole('button', { name: '반반차' }).click();
	await pickDate(page, form.getByTestId('leave-request-date-field').getByRole('button'), seoulParts().date);
	await form
		.getByTestId('leave-start-time-field')
		.locator('input[type="time"]')
		.fill(startedBeforeNowWithinTheDay());
	await form.getByRole('button', { name: '승인 요청' }).click();
}

async function approveTheRequest(page: Page): Promise<void> {
	await page.getByTestId('leave-approval-navigation').click();
	const view = page.getByTestId('leave-approval-view');
	await view.waitFor({ state: 'visible', timeout: 20000 });
	const quarterDayRequest = view
		.locator('[data-testid^="leave-approval-request-"]')
		.filter({ hasText: '반반차' });
	await expect(quarterDayRequest).toHaveCount(1, { timeout: 20000 });
	await quarterDayRequest.getByRole('button', { name: '승인', exact: true }).click();
	await expect(quarterDayRequest).toHaveCount(0, { timeout: 20000 });
}

test('a member who returns early is clocked in and no longer on leave', async ({ page }) => {
	await requestLeaveCoveringNow(page);
	await approveTheRequest(page);

	await page.reload();
	const onLeave = page.getByTestId('attendance-own-strip');
	await expect(onLeave).toBeVisible({ timeout: 20000 });
	await expect(onLeave).toContainText('휴가 중');

	await page.getByRole('button', { name: '출근', exact: true }).click();
	await page.getByRole('button', { name: '출근하기' }).click();

	await expect(page.getByTestId('attendance-own-strip')).not.toContainText('휴가 중', { timeout: 20000 });
	await expect(page.getByRole('button', { name: '퇴근', exact: true })).toBeVisible({
		timeout: 20000
	});
});
