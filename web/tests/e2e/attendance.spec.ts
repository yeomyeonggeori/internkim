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
		await expect(page.getByText(/부재 · 출장/)).toBeVisible();
		await page.getByRole('tab', { name: '개인' }).click();
		await expect(page.getByText('내 근무 시간')).toBeVisible();
		await expect(page.getByRole('button', { name: '부재 등록' })).toBeVisible();
		await page.getByRole('button', { name: /4 휴가/ }).click();
		await expect(page.getByLabel('시작일')).toHaveValue('2026-05-04');
		await expect(page.getByLabel('종료일')).toHaveValue('2026-05-04');
		await page.getByRole('tab', { name: '팀' }).click();
		await expect(page.locator('text=출석률').first()).toBeVisible();
	});

	test('keeps team cards as status-only and personal tab scoped to self', async ({ page }) => {
		await page.goto('/attendance');
		await selectKorean(page);
		await expect(page.getByRole('button', { name: /김철수/ })).toHaveCount(0);
		await expect(page.getByText(/김철수/).first()).toBeVisible();
		await expect(page.getByRole('tab', { name: '팀' })).toHaveAttribute('data-state', 'active');

		await page.getByRole('button', { name: '출결 새로고침' }).click();

		await expect(page.getByRole('tab', { name: '팀' })).toHaveAttribute('data-state', 'active');
		await page.getByRole('tab', { name: '개인' }).click();
		await expect(page.getByText('내 근무 시간')).toBeVisible();
		await expect(page.getByText('kim@example.com')).toBeVisible();
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
