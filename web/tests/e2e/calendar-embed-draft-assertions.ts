// 캘린더 embed draft와 popover e2e assertion helper를 제공합니다.
import { expect, type Page } from '@playwright/test';

export async function expectPopoverAnchoredToDraftEvent(page: Page): Promise<void> {
	await expect(page.locator('.calendar-draft-popover')).toBeVisible();
	await expect(page.locator('.draft-empty-title-event').first()).toBeVisible();
	const distance = await page.evaluate(() => {
		const popover = document.querySelector('.calendar-draft-popover');
		const draftEvent = Array.from(document.querySelectorAll<HTMLElement>('.draft-empty-title-event')).find((element) => {
			const rectangle = element.getBoundingClientRect();
			return rectangle.width > 0 && rectangle.height > 0 && !element.classList.contains('df-right-panel-event-card');
		});
		if (!(popover instanceof HTMLElement) || !(draftEvent instanceof HTMLElement)) return Number.POSITIVE_INFINITY;
		const popoverRectangle = popover.getBoundingClientRect();
		const draftRectangle = draftEvent.getBoundingClientRect();
		const horizontalGap = Math.max(
			0,
			Math.max(draftRectangle.left - popoverRectangle.right, popoverRectangle.left - draftRectangle.right)
		);
		const verticalOverlap = Math.max(
			0,
			Math.min(popoverRectangle.bottom, draftRectangle.bottom) - Math.max(popoverRectangle.top, draftRectangle.top)
		);
		return horizontalGap + (verticalOverlap > 0 ? 0 : 1000);
	});
	expect(distance).toBeLessThanOrEqual(24);
}

export async function expectPopoverArrowPointsToDraftEvent(page: Page): Promise<void> {
	const distance = await page.evaluate(() => {
		const popover = document.querySelector('.calendar-draft-popover');
		const draftEvent = Array.from(document.querySelectorAll<HTMLElement>('.draft-empty-title-event')).find((element) => {
			const rectangle = element.getBoundingClientRect();
			return rectangle.width > 0 && rectangle.height > 0 && !element.closest('.df-right-panel-events');
		});
		if (!(popover instanceof HTMLElement) || !(draftEvent instanceof HTMLElement)) return Number.POSITIVE_INFINITY;
		const popoverRectangle = popover.getBoundingClientRect();
		const draftRectangle = draftEvent.getBoundingClientRect();
		const popoverStyle = window.getComputedStyle(popover);
		const arrowTop = Number.parseFloat(popoverStyle.getPropertyValue('--draft-popover-arrow-top') || '42');
		const arrowY = popoverRectangle.top + arrowTop + 8;
		const arrowX = popover.classList.contains('popover-arrow-right') ? popoverRectangle.right + 9 : popoverRectangle.left - 9;
		const draftY = draftRectangle.top + Math.min(40, draftRectangle.height / 2);
		const draftX =
			arrowX < draftRectangle.left
				? draftRectangle.left
				: arrowX > draftRectangle.right
					? draftRectangle.right
					: arrowX;
		return Math.hypot(arrowX - draftX, arrowY - draftY);
	});
	expect(distance).toBeLessThanOrEqual(24);
}

export async function expectMiniCalendarSelectedDayTextVisible(page: Page, dateKey: string): Promise<void> {
	const result = await page.evaluate((selectedDateKey) => {
		const button = document.querySelector(`.df-mini-calendar-day[data-mini-date-key="${selectedDateKey}"]`);
		if (!(button instanceof HTMLElement)) return null;
		const style = window.getComputedStyle(button);
		return {
			text: button.childNodes[0]?.textContent?.trim() ?? '',
			color: style.color,
			backgroundColor: style.backgroundColor
		};
	}, dateKey);
	expect(result).not.toBeNull();
	expect(result?.text).toBe(String(Number(dateKey.slice(-2))));
	expect(result?.color).not.toBe(result?.backgroundColor);
	expect(result?.color).not.toBe('rgba(0, 0, 0, 0)');
}

export async function expectTimelineDraftCount(page: Page, expectedTimelineCount: number, expectedTotalCount: number): Promise<void> {
	await expect
		.poll(async () =>
			page.evaluate(() => {
				const eventIDs = Array.from(document.querySelectorAll('.df-event'))
					.map((element) => element.closest('[data-event-id]')?.getAttribute('data-event-id') ?? '')
					.filter(Boolean);
				return {
					total: new Set(eventIDs).size,
					timeline: new Set(eventIDs.filter((eventID) => eventID.startsWith('timeline-'))).size
				};
			})
		)
		.toEqual({ total: expectedTotalCount, timeline: expectedTimelineCount });
}
