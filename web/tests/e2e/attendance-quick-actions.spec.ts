import { buildAttendanceSummaryFixture } from '../../dev-attendance-summary-fixture';
import type { AttendanceSummary } from '../../src/routes/attendance/attendance-context.svelte';
import { expect, test } from './attendance-page-test-fixture';
import { buildWideTooltipSummary, selectKorean, todayDateInSeoul } from './attendance-test-helpers';

test.describe('attendance quick actions', () => {
	test('shows clock-in and clock-out times without seconds', async ({ page }) => {
		const todayDate = todayDateInSeoul();
		await page.clock.setFixedTime(new Date(`${todayDate}T21:00:00+09:00`));
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? todayDate.slice(0, 7);
			const summary = month === todayDate.slice(0, 7)
				? buildCompletedQuickActionsSummary(month, todayDate)
				: buildAttendanceSummaryFixture(month);
			await route.fulfill({ json: summary });
		});

		await page.goto('/attendance');
		await selectKorean(page);

		const clockInTime = page.getByTestId('quick-actions-clock-in-time');
		const timeRow = clockInTime.locator('..');
		const clockOutTime = timeRow.locator(':scope > span').nth(1);
		await expect(clockInTime).toHaveText('19:36');
		await expect(clockOutTime).toHaveText('20:45');
	});
});

function buildCompletedQuickActionsSummary(month: string, todayDate: string): AttendanceSummary {
	const summary = buildWideTooltipSummary(month, todayDate);
	const activeClockIn = summary.events.findLast(
		(event) =>
			event.email === summary.currentUserEmail && event.localDate === todayDate && event.kind === 'clock_in'
	);
	if (!activeClockIn) throw new Error('Expected an active clock-in fixture');
	return {
		...summary,
		events: [
			...summary.events,
			{
				...activeClockIn,
				id: 'quick-actions-clock-out',
				kind: 'clock_out',
				occurredAt: `${todayDate}T20:45:33+09:00`,
				localTime: '20:45:33'
			}
		]
	};
}
