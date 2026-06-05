import { expect, test } from '@playwright/test';
import { buildAttendanceSummaryFixture } from '../fixtures/attendance-summary';

test.describe('attendance', () => {
	test.beforeEach(async ({ page }) => {
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
		await page.getByRole('tab', { name: '개인' }).click();
		await expect(page.getByText('내 근무 시간')).toBeVisible();
		await page.getByRole('tab', { name: '팀' }).click();
		await expect(page.locator('text=출석률').first()).toBeVisible();
	});

	test('keeps the selected person tab after refreshing attendance', async ({ page }) => {
		await page.goto('/attendance');
		await page.getByLabel('Language').getByRole('button', { name: 'KO', exact: true }).click();
		await page.getByRole('button', { name: /김철수/ }).first().click();
		await expect(page.getByText('내 근무 시간')).toBeVisible();

		await page.getByRole('button', { name: '출결 새로고침' }).click();

		await expect(page.getByRole('tab', { name: '개인' })).toHaveAttribute('data-state', 'active');
		await expect(page.getByText('내 근무 시간')).toBeVisible();
	});

	test('updates a personal event location through the override API', async ({ page }) => {
		const summary = buildAttendanceSummaryFixture('2026-05');
		const event = summary.events.find(
			(candidate) =>
				candidate.email === 'kim@example.com' &&
				candidate.kind === 'clock_in' &&
				candidate.locationID === 'office' &&
				!candidate.canceledAt
		);
		if (!event) throw new Error('fixture clock-in event was not found');
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			await route.fulfill({ json: summary });
		});
		let patchCount = 0;
		await page.route('**/attendance/api/events/*/location', async (route) => {
			patchCount += 1;
			expect(route.request().method()).toBe('PATCH');
			expect(JSON.parse(route.request().postData() ?? '{}')).toEqual({ locationID: 'remote' });
			const updatedEvent = {
				...event,
				locationID: 'remote',
				locationName: '재택',
				parsedAs: { kind: event.kind, locationID: 'office' },
				overriddenBy: 'kim@example.com',
				overriddenAt: `${event.localDate}T10:30:00+09:00`,
			};
			const index = summary.events.findIndex((candidate) => candidate.id === event.id);
			summary.events[index] = updatedEvent;
			await route.fulfill({ json: updatedEvent });
		});

		await page.goto('/attendance');
		await page.getByLabel('Language').getByRole('button', { name: 'KO', exact: true }).click();
		await page.getByRole('tab', { name: '개인' }).click();
		await page
			.getByRole('button', {
				name: new RegExp(`${Number(event.localDate.slice(-2))}[\\s\\S]*${event.localTime}`),
			})
			.click();
		await page.getByRole('button', { name: new RegExp(`출근 ${event.localTime}`) }).click();
		await page.getByRole('button', { name: '재택' }).click();

		await expect.poll(() => patchCount).toBe(1);
		await expect(page.getByText('원래 장소')).toBeVisible();
		await expect(page.getByText('kim@example.com 가')).toBeVisible();
	});
});
