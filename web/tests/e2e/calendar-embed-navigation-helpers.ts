import { expect, type Page } from '@playwright/test';
import { calendarEmbedPath } from './calendar-central-test-utils';

export async function openCalendarEmbed(page: Page, viewLabel: '일' | '주' | '월'): Promise<void> {
	await page.goto(`${calendarEmbedPath}?date=2026-06-08`);
	await page.evaluate((view) => {
		window.localStorage.setItem('internkim.calendar.view', view);
	}, calendarViewStorageValue(viewLabel));
	await page.reload();
	await expect(page.locator('.calendar-stage')).toHaveClass(new RegExp(`calendar-stage-${calendarViewStorageValue(viewLabel)}`));
}

export async function navigateEmbeddedCalendar(page: Page, dateKey: string): Promise<void> {
	await page.waitForSelector('.calendar-stage', { state: 'attached' });
	await page.evaluate(
		() =>
			new Promise<void>((resolve) => {
				requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
			})
	);
	await expect
		.poll(async () =>
			page.evaluate((selectedDateKey) => {
				const selectedDate = new Date(`${selectedDateKey}T12:00:00`);
				const selectedDateValue = selectedDate.toISOString();
				window.localStorage.setItem('internkim.calendar.visibleDate', selectedDateValue);
				window.dispatchEvent(
					new StorageEvent('storage', {
						key: 'internkim.calendar.visibleDate',
						newValue: selectedDateValue
					})
				);
				const channel = new BroadcastChannel('internkim-calendar');
				channel.postMessage({ type: 'calendar-navigate', dateKey: selectedDateKey });
				channel.close();
				window.postMessage({ type: 'calendar-navigate', dateKey: selectedDateKey }, window.location.origin);
				return document.querySelector<HTMLElement>('.calendar-stage')?.dataset.calendarSelectedDateKey ?? '';
			}, dateKey)
		)
		.toBe(dateKey);
}

function calendarViewStorageValue(viewLabel: '일' | '주' | '월'): 'day' | 'week' | 'month' {
	if (viewLabel === '일') return 'day';
	if (viewLabel === '주') return 'week';
	return 'month';
}
