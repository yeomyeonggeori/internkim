import { buildAttendanceSummaryFixture } from '../../dev-attendance-summary-fixture';
import { expect, test } from './attendance-page-test-fixture';
import { selectKorean, todayDateInSeoul } from './attendance-test-helpers';

test.describe('attendance team status', () => {
	test('shows registered members even when they have no attendance records', async ({ page }) => {
		const summary = buildAttendanceSummaryFixture('2026-06');
		const registeredMemberWithoutRecords = {
			email: 'no-record@example.com',
			displayName: '무기록',
			mattermostUsername: 'no-record',
		};

		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? summary.month;
			await route.fulfill({
				json: month === summary.month
					? { ...summary, members: [...summary.members, registeredMemberWithoutRecords] }
					: buildAttendanceSummaryFixture(month)
			});
		});

		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByPlaceholder('직원 검색').fill(registeredMemberWithoutRecords.displayName);

		await expect(page.getByText(registeredMemberWithoutRecords.displayName)).toBeVisible();
		await expect(page.getByTestId(`team-status-cell-${registeredMemberWithoutRecords.email}-2026-06-16`)).toBeVisible();
	});


	test('marks the today row header with an inverted color treatment', async ({ page }) => {
		const todayDate = todayDateInSeoul();
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? todayDate.slice(0, 7);
			await route.fulfill({ json: buildAttendanceSummaryFixture(month) });
		});

		const otherDate = todayDate.endsWith('-01') ? `${todayDate.slice(0, 8)}02` : `${todayDate.slice(0, 8)}01`;
		await page.goto('/attendance');
		await selectKorean(page);

		const todayRowHeader = page.getByTestId(`team-status-day-${todayDate}`);
		const otherRowHeader = page.getByTestId(`team-status-day-${otherDate}`);

		await expect(todayRowHeader).toBeVisible();
		const [todayStyle, otherStyle] = await Promise.all([
			todayRowHeader.evaluate((element) => getComputedStyle(element).backgroundColor),
			otherRowHeader.evaluate((element) => getComputedStyle(element).backgroundColor)
		]);
		expect(todayStyle).not.toBe('rgba(0, 0, 0, 0)');
		expect(todayStyle).not.toBe(otherStyle);
	});


	test('keeps the today row visible while scrolling past it in both directions', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-25T09:00:00+09:00'));
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			await route.fulfill({ json: buildAttendanceSummaryFixture('2026-06') });
		});

		await page.goto('/attendance');
		await selectKorean(page);

		const statusTable = page.getByTestId('team-status-table');
		const todayRowHeader = page.getByTestId('team-status-day-2026-06-25');
		await expect(todayRowHeader).toBeVisible();

		await statusTable.evaluate((table) => {
			table.scrollTop = 0;
		});
		const containerBoxNearTop = await statusTable.boundingBox();
		const rowBoxNearTop = await todayRowHeader.boundingBox();
		if (!containerBoxNearTop || !rowBoxNearTop) throw new Error('Expected the status table and today row to be measurable');
		expect(rowBoxNearTop.y).toBeGreaterThanOrEqual(containerBoxNearTop.y - 1);
		expect(rowBoxNearTop.y + rowBoxNearTop.height).toBeLessThanOrEqual(containerBoxNearTop.y + containerBoxNearTop.height + 1);

		await statusTable.evaluate((table) => {
			table.scrollTop = table.scrollHeight;
		});
		const containerBoxNearBottom = await statusTable.boundingBox();
		const rowBoxNearBottom = await todayRowHeader.boundingBox();
		if (!containerBoxNearBottom || !rowBoxNearBottom) throw new Error('Expected the status table and today row to be measurable');
		expect(rowBoxNearBottom.y).toBeGreaterThanOrEqual(containerBoxNearBottom.y - 1);
		expect(rowBoxNearBottom.y + rowBoxNearBottom.height).toBeLessThanOrEqual(containerBoxNearBottom.y + containerBoxNearBottom.height + 1);

		const otherVisibleRowHeader = page.getByTestId('team-status-day-2026-06-30');
		await expect(otherVisibleRowHeader).toBeVisible();
		const otherRowText = await otherVisibleRowHeader.textContent();
		expect(otherRowText?.trim()).toContain('6/30');
	});

});
