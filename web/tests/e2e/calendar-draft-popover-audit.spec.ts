import { expect, test } from '@playwright/test';
import { routeCalendarAPI, waitForClientHydration } from './calendar-draft-popover-test-utils';

test.describe('calendar draft popover audit details', () => {
	test.beforeEach(async ({ page }) => {
		await routeCalendarAPI(page);
	});

	test('shows event audit details inside the edit popover', async ({ page }) => {
		await page.unroute('**/calendar/api/events?**');
		await page.route('**/calendar/api/events?**', async (route) => {
			await route.fulfill({
				json: {
					events: [
						{
							id: 'audited-event',
							title: '감사 표시 일정',
							description: '',
							location: '',
							startISO: '2026-06-10T09:00:00.000Z',
							endISO: '2026-06-10T10:00:00.000Z',
							timeZone: 'Asia/Seoul',
							isAllDay: false,
							color: '#1677ff',
							createdByEmail: 'creator@example.com',
							createdByName: '등록자',
							updatedByEmail: 'editor@example.com',
							updatedByName: '수정자',
							updatedByAt: '2026-06-10T11:30:00.000Z',
							updatedAt: '2026-06-10T11:30:00.000Z'
						}
					]
				}
			});
		});
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await page.locator('.calendar-month-direct-event[data-event-id="audited-event"]').click();

		const popover = page.locator('.calendar-draft-popover');
		const popoverAuditCard = popover.locator('.event-audit-card');
		await expect(popover).toBeVisible();
		await expect(popoverAuditCard).toBeVisible();
		await expect(popoverAuditCard).toContainText('등록');
		await expect(popoverAuditCard).toContainText('등록자');
		await expect(popoverAuditCard).toContainText('수정');
		await expect(popoverAuditCard).toContainText('수정자');
		await expect(page.locator('.calendar-stage > .event-audit-card')).toHaveCount(0);
	});

	test('localizes event audit details inside the edit popover in English', async ({ page }) => {
		await page.unroute('**/admin/api/locale');
		await page.route('**/admin/api/locale', async (route) => {
			await route.fulfill({ json: { locale: 'en' } });
		});
		await page.unroute('**/calendar/api/events?**');
		await page.route('**/calendar/api/events?**', async (route) => {
			await route.fulfill({
				json: {
					events: [
						{
							id: 'english-audited-event',
							title: 'English audit event',
							description: '',
							location: '',
							startISO: '2026-06-10T09:00:00.000Z',
							endISO: '2026-06-10T10:00:00.000Z',
							timeZone: 'Asia/Seoul',
							isAllDay: false,
							color: '#1677ff',
							createdByEmail: 'creator@example.com',
							createdByName: 'Creator',
							updatedByEmail: 'editor@example.com',
							updatedByName: 'Editor',
							updatedByAt: '2026-06-10T11:30:00.000Z',
							updatedAt: '2026-06-10T11:30:00.000Z'
						}
					]
				}
			});
		});
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await page.locator('.calendar-month-direct-event[data-event-id="english-audited-event"]').click();

		const popoverAuditCard = page.locator('.calendar-draft-popover .event-audit-card');
		await expect(popoverAuditCard).toBeVisible();
		await expect(popoverAuditCard).toContainText('Created');
		await expect(popoverAuditCard).toContainText('Creator');
		await expect(popoverAuditCard).toContainText('Updated');
		await expect(popoverAuditCard).toContainText('Editor');
	});

	test('shows empty audit state inside the edit popover when audit metadata is missing', async ({ page }) => {
		await page.unroute('**/calendar/api/events?**');
		await page.route('**/calendar/api/events?**', async (route) => {
			await route.fulfill({
				json: {
					events: [
						{
							id: 'audit-empty-event',
							title: '이력 없는 일정',
							description: '',
							location: '',
							startISO: '2026-06-10T09:00:00.000Z',
							endISO: '2026-06-10T10:00:00.000Z',
							timeZone: 'Asia/Seoul',
							isAllDay: false,
							color: '#1677ff',
							updatedAt: '2026-06-10T11:30:00.000Z'
						}
					]
				}
			});
		});
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await page.locator('.calendar-month-direct-event[data-event-id="audit-empty-event"]').click();

		const popover = page.locator('.calendar-draft-popover');
		const popoverAuditCard = popover.locator('.event-audit-card');
		await expect(popover).toBeVisible();
		await expect(popoverAuditCard).toBeVisible();
		await expect(popoverAuditCard).toContainText('등록·수정 정보 없음');
		await expect(page.locator('.calendar-stage > .event-audit-card')).toHaveCount(0);
	});
});
