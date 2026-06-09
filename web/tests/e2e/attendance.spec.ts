import { expect, test } from '@playwright/test';
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
		await page.getByLabel('Language').getByRole('button', { name: 'KO', exact: true }).click();
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
		await page.getByLabel('Language').getByRole('button', { name: 'KO', exact: true }).click();
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
		await page.getByLabel('Language').getByRole('button', { name: 'KO', exact: true }).click();
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
});
