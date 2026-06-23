import { expect, test, type Page } from '@playwright/test';
import { buildAttendanceSummaryFixture } from '../../dev-attendance-summary-fixture';

test.describe('attendance', () => {
	test.beforeEach(async ({ page }) => {
		await page.route('**/admin/api/session', async (route) => {
			await route.fulfill({ json: { email: 'tester@example.com', isAdmin: true } });
		});
		await page.route('**/admin/api/locale', async (route) => {
			await route.fulfill({ json: { locale: 'ko' } });
		});
		await page.route('**/auth/session**', async (route) => {
			await route.fulfill({ json: { authenticated: true, email: 'tester@example.com' } });
		});
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? '2026-05';
			await route.fulfill({ json: buildAttendanceSummaryFixture(month) });
		});
	});

	test('renders team and personal tabs with fixture summary', async ({ page }) => {
		await page.goto('/attendance');
		await selectKorean(page);
		await expect(page.getByRole('tab', { name: '팀' })).toBeVisible();
		await expect(page.getByRole('tab', { name: '개인' })).toBeVisible();
		await expect(page.getByLabel('사용자')).toHaveCount(0);
		await page.getByRole('tab', { name: '개인' }).click();
		await expect(page.getByText('내 근무 시간')).toBeVisible();
		await expect(page.getByRole('button', { name: '2026년 5월' })).toBeVisible();
		await expect(page.getByRole('button', { name: '이전 달' })).toHaveCount(1);
		await expect(page.getByRole('button', { name: '다음 달' })).toHaveCount(1);
		await expect(page.getByRole('button', { name: '부재 등록' })).toBeVisible();
		await expect(page.getByLabel('부재 유형').locator('option')).toHaveText(['휴가', '출장', '기타']);
		await page.getByRole('button', { name: /4 휴가/ }).click();
		await expect(page.getByLabel('시작일')).toHaveValue('2026-05-04');
		await expect(page.getByLabel('종료일')).toHaveValue('2026-05-04');
		await page.getByRole('tab', { name: '팀' }).click();
		const teamCalendar = page.getByTestId('team-month-calendar');
		await expect(teamCalendar).toBeVisible();
		await expect(page.getByText('2026-05 출결 달력')).toBeVisible();
		await expect(page.getByText('팀 근무 시간')).toHaveCount(0);
		await expect(page.getByTestId('team-calendar-day-2026-05-01').getByText('이영희 출장')).toBeVisible();
		await expect(page.getByTestId('team-calendar-day-2026-05-01').getByText('5/6')).toBeVisible();
		await expect(page.getByTestId('team-calendar-day-2026-05-06').getByText('박지민 휴가')).toBeVisible();
		await expect(page.getByTestId('team-calendar-day-2026-05-07').getByText('휴가', { exact: true })).toHaveCount(0);
		await expect(page.getByTestId('team-calendar-day-2026-05-08').getByText('휴가', { exact: true })).toHaveCount(0);
		await expect(teamCalendar.getByRole('button', { name: '이전 달' })).toBeVisible();
		await teamCalendar.getByRole('button', { name: '다음 달' }).click();
		await expect(page.getByText('2026-06 출결 달력')).toBeVisible();
		await expect(teamCalendar.locator('[data-testid^="team-calendar-day-"]')).toHaveCount(35);
		const overlappingDay = page.getByTestId('team-calendar-day-2026-06-10');
		await expect(overlappingDay.getByRole('button', { name: '+1건' })).toBeVisible();
		await overlappingDay.getByRole('button', { name: '+1건' }).click();
		await expect(overlappingDay.getByText('6/9~6/11')).toBeVisible();
		await expect(overlappingDay.getByText('6/10~6/12')).toBeVisible();
		await expect(overlappingDay.getByText('6/10', { exact: true })).toBeVisible();
		await page.getByTestId('team-calendar-day-2026-06-03').click();
		await expect(overlappingDay.getByText('6/9~6/11')).toHaveCount(0);
		for (const date of ['2026-06-24', '2026-06-25', '2026-06-26']) {
			const overflowRangeDay = page.getByTestId(`team-calendar-day-${date}`);
			await expect(overflowRangeDay.getByText('이영희 기타')).toHaveCount(0);
			await expect(overflowRangeDay.getByRole('button', { name: '+1건' })).toBeVisible();
			await overflowRangeDay.getByRole('button', { name: '+1건' }).click();
			await expect(overflowRangeDay.getByText('이영희 기타')).toBeVisible();
			await expect(overflowRangeDay.locator('[title="이영희 기타 6/24~6/26"]')).toBeVisible();
			await page.getByTestId('team-calendar-day-2026-06-03').click();
		}
		await expect(page.getByTestId('team-status-grid')).toBeVisible();
		await expect(page.getByText('월간 근무 현황표')).toBeVisible();
		const statusTable = page.getByTestId('team-status-table');
		await page.getByPlaceholder('직원 검색').fill('김철수');
		await expect(statusTable.getByText('김철수')).toBeVisible();
		await expect(statusTable.getByText('강민호')).toHaveCount(0);
	});

	test('keeps team tab active after refresh and personal tab scoped to self', async ({ page }) => {
		await page.goto('/attendance');
		await selectKorean(page);
		await expect(page.getByTestId('team-status-grid')).toBeVisible();
		await expect(page.getByRole('tab', { name: '팀' })).toHaveAttribute('data-state', 'active');

		await page.getByRole('button', { name: '출결 새로고침' }).click();

		await expect(page.getByRole('tab', { name: '팀' })).toHaveAttribute('data-state', 'active');
		await page.getByRole('tab', { name: '개인' }).click();
		await expect(page.getByText('내 근무 시간')).toBeVisible();
		await expect(page.getByText('kim@example.com', { exact: true })).toBeVisible();
	});

	test('opens status day details from a team status cell', async ({ page }) => {
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? todayDateInSeoul().slice(0, 7);
			await route.fulfill({ json: buildAttendanceSummaryFixture(month) });
		});

		const todayDate = todayDateInSeoul();
		await page.goto('/attendance');
		await selectKorean(page);
		const todayCell = page.getByTestId(`team-status-cell-kim@example.com-${todayDate}`);
		await expect(todayCell.getByText('외부')).toBeVisible();
		await expect(todayCell.getByText('근무 중')).toHaveCount(0);
		await todayCell.click();

		const dialog = page.getByTestId('team-status-day-detail-dialog');
		await expect(dialog.getByText('김철수')).toBeVisible();
		await expect(dialog.getByText('상태')).toHaveCount(0);
		await expect(dialog.getByText('3h 25m')).toBeVisible();
		await expect(dialog.getByText('재택')).toBeVisible();
		await expect(dialog.getByText('사무실')).toBeVisible();
		await expect(dialog.getByText('외부')).toBeVisible();
		await expect(dialog.getByText('08:30-10:20')).toBeVisible();
		await expect(dialog.getByText('10:45-12:20')).toBeVisible();
		await expect(dialog.getByText('12:45~')).toBeVisible();
	});

	test('shows the selected day in the team status table', async ({ page }) => {
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? todayDateInSeoul().slice(0, 7);
			await route.fulfill({ json: buildAttendanceSummaryFixture(month) });
		});

		const todayDate = todayDateInSeoul();
		await page.goto('/attendance');
		await selectKorean(page);

		await expect(page.getByTestId(`team-status-day-${todayDate}`)).toBeVisible();
	});

	test('keeps team month calendar inside a narrow viewport', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		await page.goto('/attendance');
		await selectKorean(page);

		const calendar = page.getByTestId('team-month-calendar');
		await expect(calendar).toBeVisible();
		const hasHorizontalOverflow = await calendar.evaluate((element) => element.scrollWidth > element.clientWidth + 1);

		expect(hasHorizontalOverflow).toBe(false);
	});

	test('registers own absence without sending an email override', async ({ page }) => {
		let requestBody: Record<string, unknown> = {};
		await page.route('**/attendance/api/absences', async (route) => {
			requestBody = JSON.parse(route.request().postData() ?? '{}') as Record<string, unknown>;
			await route.fulfill({
				json: {
					absences: [
						{
							id: 'absence-created',
							email: 'kim@example.com',
							kind: 'leave',
							labelKey: 'leave',
							date: '2026-06-10',
							createdAt: '2026-06-10T09:00:00+09:00'
						}
					]
				}
			});
		});

		await page.goto('/attendance?tab=personal');
		await selectKorean(page);
		await page.getByLabel('시작일').fill('2026-06-10');
		await page.getByLabel('종료일').fill('2026-06-10');
		await page.getByLabel('사유').fill('family');
		await page.getByRole('button', { name: '부재 등록' }).click();

		await expect(page.getByText('부재를 등록했습니다.')).toBeVisible();
		expect(requestBody).toEqual({
			kind: 'leave',
			startDate: '2026-06-10',
			endDate: '2026-06-10',
			reason: 'family'
		});
	});

	test('shows a weekday-only notice when a weekend absence creates no records', async ({ page }) => {
		await page.route('**/attendance/api/absences', async (route) => {
			await route.fulfill({ json: { absences: [] } });
		});

		await page.goto('/attendance?tab=personal');
		await selectKorean(page);
		await page.getByLabel('시작일').fill('2026-06-13');
		await page.getByLabel('종료일').fill('2026-06-14');
		await page.getByRole('button', { name: '부재 등록' }).click();

		await expect(page.getByText('등록할 평일이 없습니다.')).toBeVisible();
		await expect(page.getByText('부재를 등록했습니다.')).toHaveCount(0);
	});

	test('keeps personal calendar times inside mobile day cells', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		await page.goto('/attendance?tab=personal');
		await selectKorean(page);

		await expect(page.getByTestId('personal-month-calendar-grid')).toBeVisible();
		const overflowingLines = await page
			.getByTestId('personal-calendar-time-line')
			.evaluateAll((elements) =>
				elements
					.map((element) => {
						const cell = element.closest('[data-testid^="personal-calendar-day-"]');
						if (!cell) return null;
						const cellBounds = cell.getBoundingClientRect();
						const elementBounds = element.getBoundingClientRect();
						const isOutsideCell =
							elementBounds.left < cellBounds.left - 0.5 || elementBounds.right > cellBounds.right + 0.5;
						return isOutsideCell ? element.textContent?.trim() ?? '' : null;
					})
					.filter((line) => line !== null)
			);

		expect(overflowingLines).toEqual([]);
	});

	test('shows multiple personal work segments and chronological event rows', async ({ page }) => {
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? todayDateInSeoul().slice(0, 7);
			await route.fulfill({ json: buildAttendanceSummaryFixture(month) });
		});

		const todayDate = todayDateInSeoul();
		await page.goto('/attendance?tab=personal');
		await selectKorean(page);
		await page.getByTestId(`personal-calendar-day-${todayDate}`).click();

		const detailPanel = page.getByTestId('personal-day-detail-panel');
		await expect(detailPanel.getByText('3구간')).toBeVisible();
		await expect(detailPanel.getByText('08:30-10:20')).toBeVisible();
		await expect(detailPanel.getByText('10:45-12:20')).toBeVisible();
		await expect(detailPanel.getByText('12:45~')).toBeVisible();

		const eventLabels = detailPanel.getByTestId('personal-day-event-label');
		await expect(eventLabels.nth(0)).toContainText('출근 08:30');
		await expect(eventLabels.nth(1)).toContainText('퇴근 10:20');
		await expect(eventLabels.nth(2)).toContainText('출근 10:45');
		await expect(eventLabels.nth(3)).toContainText('퇴근 12:20');
		await expect(eventLabels.nth(4)).toContainText('출근 12:45');
	});
});

async function selectKorean(page: Page): Promise<void> {
	await page.getByRole('button', { name: 'Change language' }).click();
	await page.getByRole('menuitemradio', { name: '한국어' }).click();
}

function todayDateInSeoul(): string {
	const parts = new Intl.DateTimeFormat('en-US', {
		timeZone: 'Asia/Seoul',
		year: 'numeric',
		month: '2-digit',
		day: '2-digit'
	}).formatToParts(new Date());
	const dateParts = Object.fromEntries(parts.map((part) => [part.type, part.value]));
	return `${dateParts.year}-${dateParts.month}-${dateParts.day}`;
}
