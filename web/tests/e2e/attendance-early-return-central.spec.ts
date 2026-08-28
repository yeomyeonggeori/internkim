import { expect, test, type Page } from '@playwright/test';
import { signInToTheCentralPlane } from './central-plane-sign-in';

test.describe.configure({ mode: 'serial', timeout: 120_000 });
test.use({ locale: 'ko-KR' });

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

function startedHalfAnHourAgo(): string {
	const { hour, minute } = seoulParts();
	const started = hour * 60 + minute - 30;
	const clamped = Math.max(0, started);
	return `${String(Math.floor(clamped / 60)).padStart(2, '0')}:${String(clamped % 60).padStart(2, '0')}`;
}

async function requestLeaveCoveringNow(page: Page): Promise<void> {
	await signInToTheCentralPlane(page, '/example-co/attendance');
	await page.getByRole('button', { name: '휴가 등록' }).first().click();
	const form = page.getByTestId('leave-request-dialog').getByTestId('leave-request-form');
	await form.waitFor({ state: 'visible', timeout: 20000 });

	await form.getByTestId('leave-request-unit-control').getByRole('button', { name: '반반차' }).click();
	await form.getByTestId('leave-request-date-field').locator('input[type="date"]').fill(seoulParts().date);
	await form
		.getByTestId('leave-start-time-field')
		.locator('input[type="time"]')
		.fill(startedHalfAnHourAgo());
	await form.getByRole('button', { name: '승인 요청' }).click();
}

async function approveTheRequest(page: Page): Promise<void> {
	await page.getByTestId('leave-approval-navigation').click();
	const view = page.getByTestId('leave-approval-view');
	await view.waitFor({ state: 'visible', timeout: 20000 });
	await view.getByRole('button', { name: '승인' }).first().click();
	await expect(view.getByRole('button', { name: '승인' })).toHaveCount(0, { timeout: 20000 });
}

test('a member who returns early is clocked in and no longer on leave', async ({ page }) => {
	await requestLeaveCoveringNow(page);
	await approveTheRequest(page);

	await page.reload();
	const onLeave = page.getByTestId('active-leave-status');
	await expect(onLeave).toBeVisible({ timeout: 20000 });
	await expect(onLeave).toContainText('휴가 중');

	await page.getByRole('button', { name: '출근', exact: true }).click();
	await page.getByRole('button', { name: '출근하기' }).click();

	await expect(page.getByTestId('active-leave-status')).toHaveCount(0, { timeout: 20000 });
	await expect(page.getByRole('button', { name: '퇴근', exact: true })).toBeVisible({
		timeout: 20000
	});
});
