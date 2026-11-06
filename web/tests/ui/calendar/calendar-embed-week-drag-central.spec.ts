import { expect, test, type Page } from '@playwright/test';
import { signInToCalendar } from './calendar-central-test-utils';
import { openCalendarEmbed } from './calendar-embed-interaction-helpers';

async function routeCalendarHolidays(page: Page): Promise<void> {
	await page.route('**/api/calendar/holidays?**', async (route) => {
		await route.fulfill({ json: { holidays: [], degraded: false } });
	});
}

test.describe('embedded calendar week drag interactions', () => {
	test.beforeEach(async ({ page }) => {
		await routeCalendarHolidays(page);
	});

	test('hides DayTask native drag indicators inside the calendar stage', async ({ page }) => {
		await signInToCalendar(page);
		await openCalendarEmbed(page, '월');
		await page.locator('.calendar-stage').evaluate((stageElement) => {
			const indicator = document.createElement('div');
			indicator.className = 'df-drag-indicator-regular-pill df-event';
			indicator.textContent = 'Native drag indicator';
			stageElement.appendChild(indicator);
		});

		await expect(page.locator('.df-drag-indicator-regular-pill')).toHaveCSS('display', 'none');
	});

	test('scopes hidden DayTask panels to the calendar stage', async ({ page }) => {
		await signInToCalendar(page);
		await openCalendarEmbed(page, '월');
		await page.evaluate(() => {
			const outsidePanel = document.createElement('div');
			outsidePanel.id = 'outside-dayflow-panel';
			outsidePanel.className = 'df-event-detail-panel';
			outsidePanel.style.display = 'block';
			document.body.appendChild(outsidePanel);
		});
		await page.locator('.calendar-stage').evaluate((stageElement) => {
			const insidePanel = document.createElement('div');
			insidePanel.id = 'inside-dayflow-panel';
			insidePanel.className = 'df-event-detail-panel';
			insidePanel.style.display = 'block';
			stageElement.appendChild(insidePanel);
		});

		await expect(page.locator('#inside-dayflow-panel')).toHaveCSS('display', 'none');
		await expect(page.locator('#outside-dayflow-panel')).toHaveCSS('display', 'block');
	});
});
