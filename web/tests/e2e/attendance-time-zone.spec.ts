import type { Locator, Page } from '@playwright/test';
import { buildAttendanceSummaryFixture } from '../../dev-attendance-summary-fixture';
import type { AttendanceSummary } from '../../src/routes/attendance/attendance-context.svelte';
import { expect, test } from './attendance-page-test-fixture';
import { selectKorean } from './attendance-test-helpers';

const serverTime = new Date('2026-07-15T15:00:00+09:00');
const localDate = '2026-07-15';

async function openWorkRecordDetails(
	page: Page,
	summary: AttendanceSummary
): Promise<Locator> {
	await page.unroute('**/attendance/api/summary**');
	await page.route('**/attendance/api/summary**', async (route) => {
		await route.fulfill({ json: summary });
	});
	await page.goto('/attendance');
	await selectKorean(page);
	await page.getByTestId(`team-status-cell-kim@example.com-${localDate}`).click();
	return page.getByTestId('team-status-day-detail-dialog');
}

test.describe('attendance authoritative time zone', () => {
	test('disables work-record editing when the server marks the time zone non-authoritative', async ({ page }) => {
		const summary = {
			...buildAttendanceSummaryFixture(localDate.slice(0, 7), serverTime),
			timeZoneAuthoritative: false
		};
		const detailDialog = await openWorkRecordDetails(page, summary);

		const editButton = detailDialog.getByTestId('work-record-edit-button');
		await expect(editButton).toBeVisible();
		await expect(editButton).toBeDisabled();
	});

	test('disables work-record editing when a legacy summary omits time-zone authority', async ({ page }) => {
		const summaryWithAuthority = {
			...buildAttendanceSummaryFixture(localDate.slice(0, 7), serverTime),
			timeZoneAuthoritative: true
		};
		const { timeZoneAuthoritative: _timeZoneAuthoritative, ...legacySummary } = summaryWithAuthority;
		const detailDialog = await openWorkRecordDetails(page, legacySummary);

		const editButton = detailDialog.getByTestId('work-record-edit-button');
		await expect(editButton).toBeVisible();
		await expect(editButton).toBeDisabled();
	});

	test('closes work-record editing when a refresh revokes time-zone authority', async ({ page }) => {
		let summary = {
			...buildAttendanceSummaryFixture(localDate.slice(0, 7), serverTime),
			timeZoneAuthoritative: true
		};
		let summaryRequestCount = 0;
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			summaryRequestCount += 1;
			await route.fulfill({ json: summary });
		});
		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByTestId(`team-status-cell-kim@example.com-${localDate}`).click();

		const detailDialog = page.getByTestId('team-status-day-detail-dialog');
		const editButton = detailDialog.getByTestId('work-record-edit-button');
		await editButton.click();
		await expect(detailDialog.getByTestId('work-record-edit-actions')).toBeVisible();

		const initialRequestCount = summaryRequestCount;
		summary = { ...summary, timeZoneAuthoritative: false };
		await page.evaluate(() => window.dispatchEvent(new Event('focus')));

		await expect.poll(() => summaryRequestCount).toBe(initialRequestCount + 1);
		await expect(detailDialog.getByTestId('work-record-edit-actions')).toHaveCount(0);
		await expect(editButton).toBeDisabled();
	});

	test('closes work-record editing when a refresh changes the workspace time zone', async ({ page }) => {
		let summary = {
			...buildAttendanceSummaryFixture(localDate.slice(0, 7), serverTime),
			timeZone: 'Asia/Seoul',
			timeZoneAuthoritative: true
		};
		let summaryRequestCount = 0;
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			summaryRequestCount += 1;
			await route.fulfill({ json: summary });
		});
		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByTestId(`team-status-cell-kim@example.com-${localDate}`).click();

		const detailDialog = page.getByTestId('team-status-day-detail-dialog');
		const editButton = detailDialog.getByTestId('work-record-edit-button');
		await editButton.click();
		await expect(detailDialog.getByTestId('work-record-edit-actions')).toBeVisible();

		const initialRequestCount = summaryRequestCount;
		summary = { ...summary, timeZone: 'America/Los_Angeles' };
		await page.evaluate(() => window.dispatchEvent(new Event('focus')));

		await expect.poll(() => summaryRequestCount).toBe(initialRequestCount + 1);
		await expect(detailDialog.getByTestId('work-record-edit-actions')).toHaveCount(0);
		await expect(editButton).toBeEnabled();
	});
});

test.describe('attendance server time zone differs from the browser', () => {
	test.use({ timezoneId: 'America/Los_Angeles' });

	test('uses Seoul server time for the maximum and future-time fallback', async ({ page }) => {
		await page.clock.setFixedTime(serverTime);
		const summary = {
			...buildAttendanceSummaryFixture(localDate.slice(0, 7), serverTime),
			serverTime: serverTime.toISOString(),
			timeZone: 'Asia/Seoul',
			timeZoneAuthoritative: true
		};
		const detailDialog = await openWorkRecordDetails(page, summary);
		await detailDialog.getByTestId('work-record-edit-button').click();
		const clockOutInput = detailDialog
			.getByTestId('team-status-day-segment')
			.nth(1)
			.getByLabel('퇴근');

		await expect(clockOutInput).toHaveAttribute('max', '15:00');
		await clockOutInput.fill('16:30');
		await expect(clockOutInput).toHaveValue('15:00');
	});
});
