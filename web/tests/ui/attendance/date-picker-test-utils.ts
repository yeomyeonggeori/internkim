import type { Locator, Page } from '@playwright/test';

export async function pickDate(page: Page, trigger: Locator, dateKey: string): Promise<void> {
	const [year, month] = dateKey.split('-').map(Number);
	await trigger.click();
	const calendar = page.locator('[data-slot="popover-content"]').last();
	await calendar.locator('select').nth(1).selectOption(String(year));
	await calendar.locator('select').first().selectOption(String(month));
	await calendar.locator(`[data-bits-day][data-value="${dateKey}"]:not([data-outside-month])`).click();
}
