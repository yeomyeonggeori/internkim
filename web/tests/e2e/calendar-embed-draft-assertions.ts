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
	expect(distance).toBeLessThanOrEqual(28);
}

export async function expectPopoverArrowPointsToElement(page: Page, selector: string): Promise<void> {
	await expect
		.poll(async () => popoverArrowDistanceToElement(page, selector))
		.toBeLessThanOrEqual(28);
}

export async function expectPopoverOpensLeftOfElement(page: Page, selector: string): Promise<void> {
	await expect
		.poll(async () =>
			page.evaluate((targetSelector) => {
				const popover = document.querySelector<HTMLElement>('.calendar-draft-popover');
				const target = document.querySelector<HTMLElement>(targetSelector);
				if (!popover || !target) return false;
				const popoverRectangle = popover.getBoundingClientRect();
				const targetRectangle = target.getBoundingClientRect();
				return popover.classList.contains('popover-arrow-right') && popoverRectangle.right <= targetRectangle.left - 8;
			}, selector)
		)
		.toBe(true);
}

export async function expectPopoverArrowPointsToEventTitleEnd(page: Page, selector: string): Promise<void> {
	await expect
		.poll(async () =>
			page.evaluate((targetSelector) => {
				const popover = document.querySelector<HTMLElement>('.calendar-draft-popover');
				const title = document.querySelector<HTMLElement>(`${targetSelector} .calendar-event-title`);
				if (!popover || !title) return Number.POSITIVE_INFINITY;
				const popoverRectangle = popover.getBoundingClientRect();
				const titleRectangle = title.getBoundingClientRect();
				const popoverStyle = window.getComputedStyle(popover);
				const arrowTop = Number.parseFloat(popoverStyle.getPropertyValue('--draft-popover-arrow-top') || '42');
				const arrowY = popoverRectangle.top + arrowTop + 8;
				const arrowX = popover.classList.contains('popover-arrow-right') ? popoverRectangle.right + 9 : popoverRectangle.left - 9;
				return Math.hypot(arrowX - titleRectangle.right, arrowY - (titleRectangle.top + titleRectangle.height / 2));
			}, selector)
		)
		.toBeLessThanOrEqual(28);
}

export async function expectPopoverArrowPointsToEventTimeEnd(page: Page, selector: string): Promise<void> {
	await expect
		.poll(async () =>
			page.evaluate((targetSelector) => {
				const popover = document.querySelector<HTMLElement>('.calendar-draft-popover');
				const time = document.querySelector<HTMLElement>(`${targetSelector} .calendar-event-time`);
				if (!popover || !time) return Number.POSITIVE_INFINITY;
				const popoverRectangle = popover.getBoundingClientRect();
				const timeRectangle = time.getBoundingClientRect();
				const popoverStyle = window.getComputedStyle(popover);
				const arrowTop = Number.parseFloat(popoverStyle.getPropertyValue('--draft-popover-arrow-top') || '42');
				const arrowY = popoverRectangle.top + arrowTop + 8;
				const arrowX = popover.classList.contains('popover-arrow-right') ? popoverRectangle.right + 9 : popoverRectangle.left - 9;
				return Math.hypot(arrowX - timeRectangle.right, arrowY - (timeRectangle.top + timeRectangle.height / 2));
			}, selector)
		)
		.toBeLessThanOrEqual(28);
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

async function popoverArrowDistanceToElement(page: Page, selector: string): Promise<number> {
	return page.evaluate((targetSelector) => {
		const popover = document.querySelector<HTMLElement>('.calendar-draft-popover');
		const target = document.querySelector<HTMLElement>(targetSelector);
		if (!popover || !target) return Number.POSITIVE_INFINITY;
		const popoverRectangle = popover.getBoundingClientRect();
		const targetRectangle = target.getBoundingClientRect();
		const popoverStyle = window.getComputedStyle(popover);
		const arrowTop = Number.parseFloat(popoverStyle.getPropertyValue('--draft-popover-arrow-top') || '42');
		const arrowY = popoverRectangle.top + arrowTop + 8;
		const arrowX = popover.classList.contains('popover-arrow-right') ? popoverRectangle.right + 9 : popoverRectangle.left - 9;
		const targetX = Math.max(targetRectangle.left, Math.min(arrowX, targetRectangle.right));
		const targetY = Math.max(targetRectangle.top, Math.min(arrowY, targetRectangle.bottom));
		return Math.hypot(arrowX - targetX, arrowY - targetY);
	}, selector);
}
